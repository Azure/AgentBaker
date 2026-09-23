BeforeAll {
    function Write-Log {
        param($Message)
        Write-Host "$Message"
    }

    function Logs-To-Event {
        param($TaskName, $TaskMessage)
        Write-Host "$TaskName $TaskMessage"
    }

    function Set-ExitCode {
        param($ExitCode, $ErrorMessage)
        throw "Unexpected Set-ExitCode: $ExitCode $ErrorMessage"
    }

    function Create-Directory {
        param($FullPath, $DirectoryUsage)
        if (-not (Test-Path $FullPath)) {
            New-Item -Path $FullPath -ItemType Directory -Force | Out-Null
        }
    }

    function Get-ScheduledTask {
        param($TaskName, $ErrorAction)
    }

    function New-ScheduledTaskAction {
        param($Execute, $Argument)
    }

    function New-ScheduledTaskPrincipal {
        param($UserId, $LogonType, $RunLevel)
    }

    function New-JobTrigger {
        param([switch]$Daily, $At, $DaysInterval)
    }

    function New-ScheduledTask {
        param($Action, $Principal, $Trigger, $Description)
    }

    function Register-ScheduledTask {
        param($TaskName, $InputObject)
    }

    if (-not (Get-Command Import-Certificate -ErrorAction SilentlyContinue)) {
        function Import-Certificate {
            [CmdletBinding()]
            param($FilePath, $CertStoreLocation)
            throw 'Import-Certificate must be mocked'
        }
    }

    $helperScriptPath = Join-Path $PSScriptRoot '..\..\..\parts\windows\windowscsehelper.ps1'
    $scriptUnderTestPath = Join-Path $PSScriptRoot 'kubernetesfunc.ps1'

    . $helperScriptPath
    . $scriptUnderTestPath

    # Re-stub Set-ExitCode: the initial stub above is overwritten when
    # windowscsehelper.ps1 is dot-sourced (it defines the real Set-ExitCode at
    # ~line 288). Restore the throw-on-call sentinel so unexpected Set-ExitCode
    # invocations in code under test surface as test failures instead of
    # silently running the production exit path.
    function Set-ExitCode {
        param($ExitCode, $ErrorMessage)
        throw "Unexpected Set-ExitCode: $ExitCode $ErrorMessage"
    }
}

Describe 'Get-CertEndpointModeFromLocation' {
    It 'returns legacy for ussec regions' {
        Get-CertEndpointModeFromLocation -Location 'ussecwest' | Should -Be 'legacy'
    }

    It 'returns legacy for usnat regions' {
        Get-CertEndpointModeFromLocation -Location 'usnatcentral' | Should -Be 'legacy'
    }

    It 'returns rcv1p for public regions' {
        Get-CertEndpointModeFromLocation -Location 'southcentralus' | Should -Be 'rcv1p'
    }

    It 'handles mixed-case input' {
        Get-CertEndpointModeFromLocation -Location 'UsSeCeast' | Should -Be 'legacy'
    }
}

Describe 'Register-CACertificatesRefreshTask' {
    BeforeEach {
        $script:lastScheduledTaskArgument = $null

        Mock Logs-To-Event -MockWith { }
        Mock New-ScheduledTaskPrincipal -MockWith { return @{ Kind = 'principal' } }
        Mock New-JobTrigger -MockWith { return @{ Kind = 'trigger' } }
        Mock New-ScheduledTask -MockWith { return @{ Kind = 'definition' } }
        Mock Register-ScheduledTask -MockWith { }
        Mock New-ScheduledTaskAction -MockWith {
            param($Execute, $Argument)
            $script:lastScheduledTaskArgument = $Argument
            return @{ Execute = $Execute; Argument = $Argument }
        }
    }

    It 'skips registration when the task already exists' {
        Mock Get-ScheduledTask -MockWith { return @{ TaskName = 'aks-ca-certs-refresh-task' } }

        Register-CACertificatesRefreshTask -Location 'southcentralus'

        Assert-MockCalled -CommandName Register-ScheduledTask -Exactly -Times 0
        Assert-MockCalled -CommandName New-ScheduledTaskAction -Exactly -Times 0
    }

    It 'creates a scheduled task that passes location for cert endpoint mode derivation' {
        Mock Get-ScheduledTask -MockWith { return $null }

        Register-CACertificatesRefreshTask -Location 'southcentralus'

        Assert-MockCalled -CommandName Register-ScheduledTask -Exactly -Times 1
        $script:lastScheduledTaskArgument | Should -Match ([regex]::Escape("Get-CACertificates -Location 'southcentralus'"))
    }
}

Describe 'Should-InstallCACertificatesRefreshTask' {
    BeforeEach {
        Mock Invoke-CACertificatesRequest -MockWith { }
    }

    It 'returns true for legacy regions without calling the opt-in endpoint' {
        Mock Invoke-CACertificatesRequest

        $result = Should-InstallCACertificatesRefreshTask -Location 'ussecwest'

        $result | Should -Be $true
        Assert-MockCalled -CommandName Invoke-CACertificatesRequest -Exactly -Times 0
    }

    It 'returns true for rcv1p regions when opt-in is enabled' {
        $script:lastRetryUri = $null
        Mock Invoke-CACertificatesRequest -MockWith {
            param($Command, $Args, $Retries, $RetryDelaySeconds)
            $script:lastRetryUri = $PSBoundParameters['Args'].Uri
            $script:lastRequestParameters = $PSBoundParameters
            return [PSCustomObject]@{ Content = '{"IsOptedInForRootCerts":true}' }
        }

        $result = Should-InstallCACertificatesRefreshTask -Location 'southcentralus' -InformationVariable logs 6>$null

        $result | Should -Be $true
        Assert-MockCalled -CommandName Invoke-CACertificatesRequest -Exactly -Times 1
        $script:lastRetryUri | Should -Be 'http://168.63.129.16/acms/isOptedInForRootCerts'
        $logs.Count | Should -Be 1
        "$($logs[0])" | Should -Match 'CA certificates opt-in: True$'
        $script:lastRequestParameters.Command | Should -Be 'Invoke-WebRequest'
        $script:lastRequestParameters.Args.UseBasicParsing | Should -Be $true
        $script:lastRequestParameters.Args.TimeoutSec | Should -Be 30
        $script:lastRequestParameters.Retries | Should -Be 10
        $script:lastRequestParameters.RetryDelaySeconds | Should -Be 10
    }

    It 'returns false for rcv1p regions when opt-in is disabled' {
        Mock Invoke-CACertificatesRequest -MockWith {
            return [PSCustomObject]@{ Content = '{"IsOptedInForRootCerts":false}' }
        }

        $result = Should-InstallCACertificatesRefreshTask -Location 'southcentralus' -InformationVariable logs 6>$null

        $result | Should -Be $false
        $logs.Count | Should -Be 1
        "$($logs[0])" | Should -Match 'CA certificates opt-in: False$'
    }

    It 'keeps registration failures visible without logging raw opt-in JSON' {
        Mock Invoke-CACertificatesRequest -MockWith { throw 'simulated opt-in failure' }

        Should-InstallCACertificatesRefreshTask -Location 'southcentralus' -InformationVariable logs 6>$null | Should -Be $false

        $logs.Count | Should -Be 1
        "$($logs[0])" | Should -Match 'Skipping CA refresh task registration.*simulated opt-in failure'
        ($logs -join "`n") | Should -Not -Match 'wireserver response:|refresh completed:'
    }
}

Describe 'Invoke-CACertificatesRequest' {
    BeforeAll {
        $script:sharedRetry = (Get-Command Retry-Command).ScriptBlock
    }

    BeforeEach {
        $script:capturedLogs = @()
        $script:response = [PSCustomObject]@{ Content = 'synthetic response' }
        $script:attempt = 0
        $script:requestArgs = @{
            Uri = 'http://synthetic.invalid/certificate'
            UseBasicParsing = $true
            TimeoutSec = 30
        }
        Mock Invoke-WebRequest -MockWith { return $script:response }
        Mock Start-Sleep
        Mock Retry-Command -MockWith { throw 'CA requests must not change shared retry behavior' }
    }

    It 'returns the original response quietly on first-attempt success and splats request options' {
        $result = Invoke-CACertificatesRequest -Command 'Invoke-WebRequest' -Args $script:requestArgs -Retries 10 -RetryDelaySeconds 10 -InformationVariable script:capturedLogs 6>$null

        [object]::ReferenceEquals($result, $script:response) | Should -Be $true
        Assert-MockCalled Invoke-WebRequest -Exactly -Times 1 -ParameterFilter {
            $Uri -eq 'http://synthetic.invalid/certificate' -and $UseBasicParsing -and $TimeoutSec -eq 30
        }
        $script:capturedLogs.Count | Should -Be 0
        Assert-MockCalled Start-Sleep -Exactly -Times 0
        Assert-MockCalled Retry-Command -Exactly -Times 0
    }

    It 'logs only real retries and preserves the requested backoff before returning' {
        Mock Invoke-WebRequest -MockWith {
            $script:attempt++
            if ($script:attempt -le 2) { throw 'transient failure' }
            return $script:response
        }

        $result = Invoke-CACertificatesRequest -Command 'Invoke-WebRequest' -Args $script:requestArgs -Retries 3 -RetryDelaySeconds 7 -InformationVariable script:capturedLogs 6>$null

        [object]::ReferenceEquals($result, $script:response) | Should -Be $true
        Assert-MockCalled Invoke-WebRequest -Exactly -Times 3
        Assert-MockCalled Start-Sleep -Exactly -Times 2 -ParameterFilter { $Seconds -eq 7 }
        $script:capturedLogs.Count | Should -Be 2
        "$($script:capturedLogs[0])" | Should -Match 'Retry 1 : Invoke-WebRequest$'
        "$($script:capturedLogs[1])" | Should -Match 'Retry 2 : Invoke-WebRequest$'
    }

    It 'rethrows the terminal error without an extra attempt or final sleep' {
        Mock Invoke-WebRequest -MockWith {
            $script:attempt++
            throw "failure $script:attempt"
        }

        {
            Invoke-CACertificatesRequest -Command 'Invoke-WebRequest' -Args $script:requestArgs -Retries 3 -RetryDelaySeconds 7 -InformationVariable script:capturedLogs 6>$null
        } | Should -Throw '*failure 3*'

        Assert-MockCalled Invoke-WebRequest -Exactly -Times 3
        Assert-MockCalled Start-Sleep -Exactly -Times 2 -ParameterFilter { $Seconds -eq 7 }
        $script:capturedLogs.Count | Should -Be 2
        ($script:capturedLogs -join "`n") | Should -Not -Match 'Retry 0|synthetic.invalid'
    }

    It 'does not sleep or retry when only one attempt is allowed' {
        Mock Invoke-WebRequest -MockWith { throw 'single attempt failure' }

        {
            Invoke-CACertificatesRequest -Command 'Invoke-WebRequest' -Args $script:requestArgs -Retries 1 -RetryDelaySeconds 7 -InformationVariable script:capturedLogs 6>$null
        } | Should -Throw '*single attempt failure*'

        Assert-MockCalled Invoke-WebRequest -Exactly -Times 1
        Assert-MockCalled Start-Sleep -Exactly -Times 0
        $script:capturedLogs.Count | Should -Be 0
    }

    It 'preserves empty and array success-output semantics' {
        Mock Invoke-WebRequest -MockWith { return $null }
        $expected = @(& $script:sharedRetry -Command 'Invoke-WebRequest' -Args $script:requestArgs -Retries 1 -RetryDelaySeconds 1 6>$null)
        $result = @(Invoke-CACertificatesRequest -Command 'Invoke-WebRequest' -Args $script:requestArgs -Retries 1 -RetryDelaySeconds 1 -InformationVariable script:capturedLogs 6>$null)
        $result.Count | Should -Be $expected.Count
        $result | Should -BeNullOrEmpty
        $script:capturedLogs.Count | Should -Be 0
        Mock Invoke-WebRequest -MockWith { return @('first', 'second') }
        $result = @(Invoke-CACertificatesRequest -Command 'Invoke-WebRequest' -Args $script:requestArgs -Retries 1 -RetryDelaySeconds 1 -InformationVariable script:capturedLogs 6>$null)
        $result.Count | Should -Be 2
        $result[0] | Should -Be 'first'
        $result[1] | Should -Be 'second'
        $script:capturedLogs.Count | Should -Be 0
    }

    It 'rejects an empty command before invoking it' {
        { Invoke-CACertificatesRequest -Command '' -Args $script:requestArgs -Retries 1 -RetryDelaySeconds 1 } | Should -Throw
        Assert-MockCalled Invoke-WebRequest -Exactly -Times 0
    }

    It 'rejects null request arguments before invoking the command' {
        { Invoke-CACertificatesRequest -Command 'Invoke-WebRequest' -Args $null -Retries 1 -RetryDelaySeconds 1 } | Should -Throw
        Assert-MockCalled Invoke-WebRequest -Exactly -Times 0
    }
}

Describe 'Get-CACertificates' {
    BeforeEach {
        Mock Create-Directory
        Mock Join-Path -ParameterFilter { $Path -eq 'C:\ca' } -MockWith {
            param($Path, $ChildPath)
            Microsoft.PowerShell.Management\Join-Path $TestDrive $ChildPath
        }
        Mock Import-Certificate
    }

    It 'uses the legacy endpoint when location is a ussec/usnat region' {
        $script:retryUris = @()
        Mock Invoke-CACertificatesRequest -MockWith {
            param($Command, $Args, $Retries, $RetryDelaySeconds)
            $script:retryUris += $PSBoundParameters['Args'].Uri
            return [PSCustomObject]@{
                Content = '{"Certificates":[{"Name":"legacy.crt","CertBody":"legacy-body"}]}'
            }
        }

        $result = Get-CACertificates -Location 'ussecwest'

        $result | Should -Be $true
        Assert-MockCalled -CommandName Invoke-CACertificatesRequest -Exactly -Times 1
        $script:retryUris | Should -Contain 'http://168.63.129.16/machine?comp=acmspackage&type=cacertificates&ext=json'
        $script:retryUris | Should -Not -Contain 'http://168.63.129.16/acms/isOptedInForRootCerts'
        [IO.File]::ReadAllBytes((Join-Path $TestDrive 'legacy.crt')) | Should -Not -Contain 0
        Assert-MockCalled -CommandName Import-Certificate -Exactly -Times 1 -ParameterFilter {
            $FilePath -eq (Join-Path $TestDrive 'legacy.crt') -and
            $CertStoreLocation -eq 'Cert:\LocalMachine\Root' -and
            $ErrorAction -eq 'Stop'
        }
    }

    It 'imports rcv1p root and intermediate certificates into their LocalMachine stores' {
        Mock Invoke-CACertificatesRequest -MockWith {
            param($Command, $Args, $Retries, $RetryDelaySeconds)
            $uri = $PSBoundParameters['Args'].Uri
            if ($uri -eq 'http://168.63.129.16/acms/isOptedInForRootCerts') {
                return [PSCustomObject]@{ Content = '{"IsOptedInForRootCerts":true}' }
            }
            if ($uri -like '*type=operationrequestsroot&*') {
                return [PSCustomObject]@{ Content = '{"OperationsInfo":[{"ResouceFileName":"root.crt"}]}' }
            }
            if ($uri -like '*type=operationrequestsintermediate&*') {
                return [PSCustomObject]@{ Content = '{"OperationsInfo":[{"ResouceFileName":"intermediate.crt"}]}' }
            }
            return [PSCustomObject]@{ Content = 'certificate-body' }
        }

        $result = Get-CACertificates -Location 'southcentralus'

        $result | Should -Be $true
        [IO.File]::ReadAllBytes((Join-Path $TestDrive 'root.crt')) | Should -Not -Contain 0
        [IO.File]::ReadAllBytes((Join-Path $TestDrive 'intermediate.crt')) | Should -Not -Contain 0
        Assert-MockCalled -CommandName Import-Certificate -Exactly -Times 1 -ParameterFilter {
            $FilePath -eq (Join-Path $TestDrive 'root.crt') -and
            $CertStoreLocation -eq 'Cert:\LocalMachine\Root' -and
            $ErrorAction -eq 'Stop'
        }
        Assert-MockCalled -CommandName Import-Certificate -Exactly -Times 1 -ParameterFilter {
            $FilePath -eq (Join-Path $TestDrive 'intermediate.crt') -and
            $CertStoreLocation -eq 'Cert:\LocalMachine\CA' -and
            $ErrorAction -eq 'Stop'
        }
    }

    It 'returns false when certificate retrieval throws' {
        Mock Invoke-CACertificatesRequest -MockWith {
            throw 'simulated retrieval failure'
        }

        $result = Get-CACertificates -Location 'southcentralus'

        $result | Should -Be $false
    }

    It 'throws when certificate retrieval fails with -FailOnError' {
        Mock Invoke-CACertificatesRequest -MockWith {
            throw 'simulated retrieval failure'
        }

        { Get-CACertificates -Location 'southcentralus' -FailOnError } | Should -Throw '*Failed to process CA certificates*simulated retrieval failure*'
    }

    It 'identifies the certificate and store when import fails with -FailOnError' {
        Mock Invoke-CACertificatesRequest -MockWith {
            return [PSCustomObject]@{
                Content = '{"Certificates":[{"Name":"broken.crt","CertBody":"invalid-body"}]}'
            }
        }
        Mock Import-Certificate -MockWith {
            throw 'simulated import failure'
        }

        {
            Get-CACertificates -Location 'ussecwest' -FailOnError
        } | Should -Throw "*Failed to import CA certificate 'broken.crt' into Cert:\LocalMachine\Root*simulated import failure*"
    }

    It 'throws when legacy endpoint returns empty data with -FailOnError' {
        Mock Invoke-CACertificatesRequest -MockWith {
            return [PSCustomObject]@{
                Content = '{"Certificates":[]}'
            }
        }

        { Get-CACertificates -Location 'ussecwest' -FailOnError } | Should -Throw '*CA certificates rawdata is empty*'
    }

    It 'falls back to legacy endpoint when called without -Location (backward compat)' {
        $script:retryUris = @()
        Mock Invoke-CACertificatesRequest -MockWith {
            param($Command, $Args, $Retries, $RetryDelaySeconds)
            $script:retryUris += $PSBoundParameters['Args'].Uri
            return [PSCustomObject]@{
                Content = '{"Certificates":[{"Name":"compat.crt","CertBody":"compat-body"}]}'
            }
        }

        $result = Get-CACertificates

        $result | Should -Be $true
        Assert-MockCalled -CommandName Invoke-CACertificatesRequest -Exactly -Times 1
        $script:retryUris | Should -Contain 'http://168.63.129.16/machine?comp=acmspackage&type=cacertificates&ext=json'
    }
}

Describe 'Should-InstallCACertificatesRefreshTask - backward compat' {
    It 'returns true when called without -Location (backward compat)' {
        $result = Should-InstallCACertificatesRefreshTask

        $result | Should -Be $true
    }
}

Describe 'CA refresh logging' {
    BeforeEach {
        $script:optIn = $true
        $script:rootResources = @('root.crt')
        $script:intermediateResources = @('intermediate.crt')
        $script:emptyResources = @()
        $script:missingOperations = $false
        Mock Create-Directory
        Mock Join-Path -ParameterFilter { $Path -eq 'C:\ca' } -MockWith {
            param($Path, $ChildPath)
            Microsoft.PowerShell.Management\Join-Path $TestDrive $ChildPath
        }
        Mock Import-Certificate
        Mock Start-Sleep
        Mock Retry-Command -MockWith { throw 'CA refresh must use its own request helper' }
        Mock Invoke-WebRequest -MockWith {
            param($Uri)
            if ($Uri -like '*/isOptedInForRootCerts') {
                return [PSCustomObject]@{ Content = (@{ IsOptedInForRootCerts = $script:optIn } | ConvertTo-Json -Compress) }
            }
            if ($Uri -like '*type=cacertificates&*') {
                $certificates = @($script:rootResources | ForEach-Object { @{ Name = $_; CertBody = 'synthetic-body' } })
                return [PSCustomObject]@{ Content = (@{ Certificates = $certificates } | ConvertTo-Json -Depth 4 -Compress) }
            }
            if ($Uri -like '*type=operationrequests*&*') {
                if ($script:missingOperations) {
                    return [PSCustomObject]@{ Content = '{}' }
                }
                $resources = if ($Uri -like '*type=operationrequestsroot&*') {
                    $script:rootResources
                } else {
                    $script:intermediateResources
                }
                $operations = @($resources | ForEach-Object { @{ ResouceFileName = $_ } })
                return [PSCustomObject]@{ Content = (@{ OperationsInfo = $operations } | ConvertTo-Json -Depth 4 -Compress) }
            }
            foreach ($resource in @($script:rootResources) + @($script:intermediateResources)) {
                if (-not $resource) { continue }
                $type = [IO.Path]::GetFileNameWithoutExtension($resource)
                $ext = [IO.Path]::GetExtension($resource).TrimStart('.')
                if ($Uri -eq "http://168.63.129.16/machine?comp=acmspackage&type=$type&ext=$ext") {
                    $content = if ($script:emptyResources -contains $resource) { '' } else { 'synthetic-body' }
                    return [PSCustomObject]@{ Content = $content }
                }
            }
            throw "Unexpected mocked WireServer URI: $Uri"
        }
    }

    AfterEach {
        Assert-MockCalled Retry-Command -Exactly -Times 0
    }

    It 'emits a constant number of information records and a single boolean for <Count> certificates' -TestCases @(
        @{ Count = 2 },
        @{ Count = 10 }
    ) {
        param($Count)
        $script:rootResources = @(1..($Count - 1) | ForEach-Object { "root-$_.crt" })

        $output = @(Get-CACertificates -Location 'southcentralus' 6>&1 3>&1)

        @($output | Where-Object { $_ -is [bool] }).Count | Should -Be 1
        ($output | Where-Object { $_ -is [bool] }) | Should -Be $true
        $information = @($output | Where-Object { $_ -is [System.Management.Automation.InformationRecord] })
        $information.Count | Should -Be 3
        $output.Count | Should -Be 4
        "$($information[1])" | Should -Match 'CA certificates opt-in: True$'
        "$($information[2])" | Should -Match "CA certificates refresh completed: imported $Count certificates[.]$"
        ($information -join "`n") | Should -Not -Match 'Retry 0|Write certificate |Import certificate |"IsOptedInForRootCerts"'
        Assert-MockCalled Import-Certificate -Exactly -Times $Count
        Assert-MockCalled Invoke-WebRequest -Exactly -Times ($Count + 3) -ParameterFilter {
            $UseBasicParsing -and $TimeoutSec -eq 30
        }
    }

    It 'preserves information logs when the scheduled pipeline discards success output' {
        $output = @(& { Get-CACertificates -Location 'southcentralus' | Out-Null } 6>&1)

        $output.Count | Should -Be 3
        @($output | Where-Object { $_ -isnot [System.Management.Automation.InformationRecord] }).Count | Should -Be 0
        "$($output[-1])" | Should -Match 'imported 2 certificates[.]$'
    }

    It 'reports one completion count for legacy imports without per-certificate chatter' {
        $script:rootResources = @('first.crt', 'second.crt')

        $output = @(Get-CACertificates 6>&1)

        $output[-1] | Should -Be $true
        $output.Count | Should -Be 3
        "$($output[0])" | Should -Match 'defaulting to legacy endpoint mode$'
        "$($output[1])" | Should -Match 'imported 2 certificates[.]$'
        Assert-MockCalled Import-Certificate -Exactly -Times 2 -ParameterFilter {
            $CertStoreLocation -eq 'Cert:\LocalMachine\Root' -and $ErrorAction -eq 'Stop'
        }
        Assert-MockCalled Invoke-WebRequest -Exactly -Times 1 -ParameterFilter {
            $UseBasicParsing -and $TimeoutSec -eq 30
        }
    }

    It 'logs opt-out without fetching or importing any certificates' {
        $script:optIn = $false

        $output = @(Get-CACertificates -Location 'southcentralus' 6>&1)

        $output[-1] | Should -Be $false
        ($output -join "`n") | Should -Match 'CA certificates opt-in: False'
        ($output -join "`n") | Should -Match 'Skipping custom cloud root cert installation'
        ($output -join "`n") | Should -Not -Match 'refresh completed:|Retry 0|wireserver response:'
        Assert-MockCalled Invoke-WebRequest -Exactly -Times 1
        Assert-MockCalled Import-Certificate -Exactly -Times 0
    }

    It 'counts only successful imports when a downloaded certificate is empty' {
        $script:emptyResources = @('root.crt')

        $output = @(Get-CACertificates -Location 'southcentralus' 6>&1 3>&1)

        $output[-1] | Should -Be $true
        $warning = @($output | Where-Object { "$_" -like '*Warning: empty certificate content for root.crt' })
        $warning.Count | Should -Be 1
        $warning[0] | Should -BeOfType ([System.Management.Automation.InformationRecord])
        ($output -join "`n") | Should -Match 'imported 1 certificates[.]'
        Assert-MockCalled Import-Certificate -Exactly -Times 1 -ParameterFilter {
            $CertStoreLocation -eq 'Cert:\LocalMachine\CA'
        }
    }

    It 'preserves path rejection warnings and does not count rejected or missing names' {
        $script:rootResources = @('../rejected.crt', '')

        $output = @(Get-CACertificates -Location 'southcentralus' 6>&1)

        $output[-1] | Should -Be $true
        ($output -join "`n") | Should -Match 'Warning: rejecting certificate filename with path separators: ../rejected.crt'
        ($output -join "`n") | Should -Match 'imported 1 certificates[.]'
        Assert-MockCalled Invoke-WebRequest -Exactly -Times 4
        Assert-MockCalled Import-Certificate -Exactly -Times 1
    }

    It 'does not emit a completion message if an import fails after an earlier successful import' {
        Mock Import-Certificate -ParameterFilter { $CertStoreLocation -eq 'Cert:\LocalMachine\CA' } -MockWith {
            throw 'simulated intermediate import failure'
        }

        $output = @(Get-CACertificates -Location 'southcentralus' 6>&1 3>&1)

        $output[-1] | Should -Be $false
        ($output -join "`n") | Should -Match 'Warning: failed to process CA certificates'
        ($output -join "`n") | Should -Match 'simulated intermediate import failure'
        ($output -join "`n") | Should -Not -Match 'refresh completed:'
        @($output | Where-Object { $_ -is [System.Management.Automation.WarningRecord] }).Count | Should -Be 0
        Assert-MockCalled Import-Certificate -Exactly -Times 2
    }

    It 'throws on an rcv1p import failure with FailOnError without logging completion' {
        $script:capturedLogs = @()
        Mock Import-Certificate -MockWith { throw 'simulated import failure' }

        { Get-CACertificates -Location 'southcentralus' -FailOnError -InformationVariable script:capturedLogs 6>$null } | Should -Throw '*Failed to import CA certificate*simulated import failure*'

        ($script:capturedLogs -join "`n") | Should -Not -Match 'refresh completed:'
    }

    It 'does not log successful completion for empty legacy or rcv1p data in <Location>' -TestCases @(
        @{ Location = 'ussecwest' },
        @{ Location = 'southcentralus' }
    ) {
        param($Location)
        $script:rootResources = @()
        $script:intermediateResources = @()

        $output = @(Get-CACertificates -Location $Location 6>&1)

        $output[-1] | Should -Be $false
        ($output -join "`n") | Should -Match 'Warning:'
        ($output -join "`n") | Should -Not -Match 'refresh completed:'
        Assert-MockCalled Import-Certificate -Exactly -Times 0
    }

    It 'throws for empty opted-in data with FailOnError without logging completion' {
        $script:rootResources = @()
        $script:intermediateResources = @()
        $script:capturedLogs = @()

        { Get-CACertificates -Location 'southcentralus' -FailOnError -InformationVariable script:capturedLogs 6>$null } | Should -Throw '*No CA certificates were downloaded*'

        ($script:capturedLogs -join "`n") | Should -Not -Match 'refresh completed:'
    }

    It 'preserves missing operation warnings without logging successful completion' {
        $script:missingOperations = $true

        $output = @(Get-CACertificates -Location 'southcentralus' 6>&1)

        $output[-1] | Should -Be $false
        ($output -join "`n") | Should -Match 'Warning: no operation requests found for operationrequestsroot'
        ($output -join "`n") | Should -Match 'Warning: no operation requests found for operationrequestsintermediate'
        ($output -join "`n") | Should -Not -Match 'refresh completed:'
        Assert-MockCalled Import-Certificate -Exactly -Times 0
    }
}

Describe 'Register-CACertificatesRefreshTask - backward compat' {
    BeforeEach {
        $script:lastScheduledTaskArgument = $null

        Mock Logs-To-Event -MockWith { }
        Mock New-ScheduledTaskPrincipal -MockWith { return @{ Kind = 'principal' } }
        Mock New-JobTrigger -MockWith { return @{ Kind = 'trigger' } }
        Mock New-ScheduledTask -MockWith { return @{ Kind = 'definition' } }
        Mock Register-ScheduledTask -MockWith { }
        Mock New-ScheduledTaskAction -MockWith {
            param($Execute, $Argument)
            $script:lastScheduledTaskArgument = $Argument
            return @{ Execute = $Execute; Argument = $Argument }
        }
    }

    It 'creates a scheduled task without -Location when called without it (backward compat)' {
        Mock Get-ScheduledTask -MockWith { return $null }

        Register-CACertificatesRefreshTask

        Assert-MockCalled -CommandName Register-ScheduledTask -Exactly -Times 1
        $script:lastScheduledTaskArgument | Should -Match ([regex]::Escape("Get-CACertificates |"))
        $script:lastScheduledTaskArgument | Should -Not -Match "Location"
    }
}
