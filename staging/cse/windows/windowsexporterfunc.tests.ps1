Describe 'Windows exporter CSE functions' {
    BeforeAll {
        . $PSScriptRoot\..\..\..\parts\windows\windowscsehelper.ps1
        . $PSScriptRoot\helpers.ps1
        . $PSCommandPath.Replace('.tests.ps1','.ps1')
        $global:KubeDir = 'C:\k'

        function Write-Log {
            param($Message)
            $global:WindowsExporterTestLogs += @($Message)
            Write-Host "LOG: $Message"
        }
    }

    Context 'Test-WindowsExporterAssets' {
        It 'returns NotBaked when neither marker is present' {
            Mock Test-Path -MockWith { return $false }

            Test-WindowsExporterAssets | Should -Be 'NotBaked'
        }

        It 'returns Invalid when only the baked skip marker exists' {
            Mock Test-Path -MockWith {
                param($Path)
                return $Path -eq $global:WindowsExporterSkipFile
            }

            Test-WindowsExporterAssets | Should -Be 'Invalid'
        }

        It 'returns Invalid when <Missing> is absent' -TestCases @(
            @{ Missing = 'skip marker' }
            @{ Missing = 'binary' }
            @{ Missing = 'config' }
            @{ Missing = 'nssm' }
        ) {
            param($Missing)
            $script:missingPath = switch ($Missing) {
                'skip marker' { $global:WindowsExporterSkipFile }
                'binary' { $global:WindowsExporterBinary }
                'config' { $global:WindowsExporterConfig }
                'nssm' { 'C:\k\nssm.exe' }
            }
            Mock Test-Path -MockWith {
                param($Path)
                return $Path -ne $script:missingPath
            }

            Test-WindowsExporterAssets | Should -Be 'Invalid'
        }

        It 'returns Ready when all required assets exist' {
            Mock Test-Path -MockWith { return $true }

            Test-WindowsExporterAssets | Should -Be 'Ready'
        }
    }

    Context 'Register-WindowsExporter' {
        BeforeEach {
            Mock New-Item
            Mock Remove-Item
            Mock Invoke-Nssm
            Mock Test-WindowsExporterHealth
        }

        AfterEach {
            Assert-MockCalled New-Item -Exactly -Times 0
            Assert-MockCalled Remove-Item -Exactly -Times 0
            Assert-MockCalled Test-WindowsExporterHealth -Exactly -Times 0
        }

        It 'no-ops successfully when the VHD has no exporter assets' {
            Mock Test-WindowsExporterAssets -MockWith { return 'NotBaked' }

            Register-WindowsExporter | Should -Be $true
            Assert-MockCalled Invoke-Nssm -Exactly -Times 0
        }

        It 'fails without touching the service when assets are invalid' {
            Mock Test-WindowsExporterAssets -MockWith { return 'Invalid' }

            Register-WindowsExporter | Should -Be $false
            Assert-MockCalled Invoke-Nssm -Exactly -Times 0
        }

        It 'installs a new service disabled without starting it' {
            Mock Test-WindowsExporterAssets -MockWith { return 'Ready' }
            Mock Get-Service -MockWith { return $null }

            Register-WindowsExporter | Should -Be $true

            Assert-MockCalled Invoke-Nssm -Exactly -Times 1 -ParameterFilter {
                $NssmArguments[0] -eq 'install' -and
                $NssmArguments[1] -eq $global:WindowsExporterServiceName -and
                $NssmArguments[2] -eq $global:WindowsExporterBinary
            }
            Assert-MockCalled Invoke-Nssm -Exactly -Times 1 -ParameterFilter {
                $NssmArguments[0] -eq 'set' -and $NssmArguments[2] -eq 'Start' -and $NssmArguments[3] -eq 'SERVICE_DISABLED'
            }
            Assert-MockCalled Invoke-Nssm -Exactly -Times 1 -ParameterFilter {
                $NssmArguments[0] -eq 'set' -and $NssmArguments[2] -eq 'AppStdout' -and $NssmArguments[3] -eq $global:WindowsExporterStdoutLog
            }
            Assert-MockCalled Invoke-Nssm -Exactly -Times 0 -ParameterFilter {
                $NssmArguments[0] -in @('start', 'stop') -or $NssmArguments -contains 'SERVICE_AUTO_START'
            }
        }

        It 'updates an existing service without reinstalling, stopping, or starting it' {
            Mock Test-WindowsExporterAssets -MockWith { return 'Ready' }
            Mock Get-Service -MockWith { return @{ Status = 'Running' } }

            Register-WindowsExporter | Should -Be $true

            Assert-MockCalled Invoke-Nssm -Exactly -Times 0 -ParameterFilter {
                $NssmArguments[0] -in @('install', 'start', 'stop')
            }
            Assert-MockCalled Invoke-Nssm -Exactly -Times 1 -ParameterFilter {
                $NssmArguments[0] -eq 'set' -and $NssmArguments[2] -eq 'Application' -and $NssmArguments[3] -eq $global:WindowsExporterBinary
            }
        }

        It 'returns a single boolean when nssm writes output' {
            Mock Test-WindowsExporterAssets -MockWith { return 'Ready' }
            Mock Get-Service -MockWith { return $null }
            Mock Invoke-Nssm -MockWith { 'Service "aks-windows-exporter" installed successfully!' }

            $result = Register-WindowsExporter

            $result | Should -BeOfType [bool]
            $result | Should -BeExactly $true
        }

        It 'returns failure when configuration fails' {
            Mock Test-WindowsExporterAssets -MockWith { return 'Ready' }
            Mock Get-Service -MockWith { return $null }
            Mock Invoke-Nssm -MockWith { throw 'nssm failed' }

            Register-WindowsExporter | Should -Be $false
        }
    }

    Context 'Enable-WindowsExporter' {
        BeforeEach {
            Mock New-Item
            Mock Remove-Item
            Mock Register-WindowsExporter -MockWith { return $true }
        }

        AfterEach {
            Assert-MockCalled New-Item -Exactly -Times 0
            Assert-MockCalled Remove-Item -Exactly -Times 0
        }

        It 'no-ops successfully when the VHD has no exporter assets' {
            Mock Test-WindowsExporterAssets -MockWith { return 'NotBaked' }
            Mock Invoke-Nssm

            Enable-WindowsExporter | Should -Be $true
            Assert-MockCalled Invoke-Nssm -Exactly -Times 0
            Assert-MockCalled Register-WindowsExporter -Exactly -Times 0
        }

        It 'fails without touching the service when assets are invalid' {
            Mock Test-WindowsExporterAssets -MockWith { return 'Invalid' }
            Mock Invoke-Nssm

            Enable-WindowsExporter | Should -Be $false
            Assert-MockCalled Invoke-Nssm -Exactly -Times 0
        }

        It 'enables and starts a service registered by BasePrep' {
            Mock Test-WindowsExporterAssets -MockWith { return 'Ready' }
            Mock Get-Service -MockWith { return @{ Status = 'Stopped' } }
            $script:exporterOperations = @()
            Mock Invoke-Nssm -MockWith {
                $script:exporterOperations += ($NssmArguments -join ' ')
            }
            Mock Test-WindowsExporterHealth -MockWith { return $true }

            Enable-WindowsExporter | Should -Be $true

            Assert-MockCalled Register-WindowsExporter -Exactly -Times 0
            $script:exporterOperations | Should -Be @(
                "set $($global:WindowsExporterServiceName) Start SERVICE_AUTO_START",
                "start $($global:WindowsExporterServiceName)"
            )
            Assert-MockCalled Test-WindowsExporterHealth -Exactly -Times 1
        }

        It 'registers the service first when BasePrep did not' {
            Mock Test-WindowsExporterAssets -MockWith { return 'Ready' }
            $script:serviceChecks = 0
            Mock Get-Service -MockWith {
                $script:serviceChecks++
                if ($script:serviceChecks -eq 1) { return $null }
                return @{ Status = 'Stopped' }
            }
            Mock Invoke-Nssm
            Mock Test-WindowsExporterHealth -MockWith { return $true }

            Enable-WindowsExporter | Should -Be $true

            Assert-MockCalled Register-WindowsExporter -Exactly -Times 1
            Assert-MockCalled Invoke-Nssm -Exactly -Times 1 -ParameterFilter { $NssmArguments[0] -eq 'start' }
        }

        It 'does not start the service when fallback registration fails' {
            Mock Test-WindowsExporterAssets -MockWith { return 'Ready' }
            Mock Get-Service -MockWith { return $null }
            Mock Register-WindowsExporter -MockWith { return $false }
            Mock Invoke-Nssm

            Enable-WindowsExporter | Should -Be $false

            Assert-MockCalled Invoke-Nssm -Exactly -Times 0
        }

        It 'restarts an existing <Status> service after enabling it' -TestCases @(
            @{ Status = 'Running' }
            @{ Status = 'Paused' }
        ) {
            param($Status)
            Mock Test-WindowsExporterAssets -MockWith { return 'Ready' }
            $script:exporterStatus = $Status
            Mock Get-Service -MockWith { return @{ Status = $script:exporterStatus } }
            $script:exporterOperations = @()
            Mock Invoke-Nssm -MockWith {
                $script:exporterOperations += $NssmArguments[0]
            }
            Mock Test-WindowsExporterHealth -MockWith { return $true }

            Enable-WindowsExporter | Should -Be $true

            $script:exporterOperations | Should -Be @('set', 'stop', 'start')
        }

        It 'does not stop a running exporter when enabling fails' {
            Mock Test-WindowsExporterAssets -MockWith { return 'Ready' }
            Mock Get-Service -MockWith { return @{ Status = 'Running' } }
            Mock Invoke-Nssm -MockWith {
                if ($NssmArguments[0] -eq 'set') { throw 'configuration failed' }
            }
            Mock Test-WindowsExporterHealth

            Enable-WindowsExporter | Should -Be $false

            Assert-MockCalled Invoke-Nssm -Exactly -Times 0 -ParameterFilter { $NssmArguments[0] -in @('stop', 'start') }
            Assert-MockCalled Test-WindowsExporterHealth -Exactly -Times 0
        }

        It 'succeeds when a nonzero start result is followed by service health' {
            Mock Test-WindowsExporterAssets -MockWith { return 'Ready' }
            Mock Get-Service -MockWith { return @{ Status = 'Stopped' } }
            Mock Invoke-Nssm -MockWith {
                if ($NssmArguments[0] -eq 'start') { throw 'nssm.exe start failed with exit code 1: SERVICE_START_PENDING' }
            }
            Mock Test-WindowsExporterHealth -MockWith { return $true }

            Enable-WindowsExporter | Should -Be $true

            Assert-MockCalled Test-WindowsExporterHealth -Exactly -Times 1
        }

        It 'returns a single boolean when nssm writes output' {
            Mock Test-WindowsExporterAssets -MockWith { return 'Ready' }
            Mock Get-Service -MockWith { return @{ Status = 'Running' } }
            Mock Invoke-Nssm -MockWith { 'aks-windows-exporter: STOP: The operation completed successfully.' }
            Mock Test-WindowsExporterHealth -MockWith { return $true }

            $result = Enable-WindowsExporter

            $result | Should -BeOfType [bool]
            $result | Should -BeExactly $true
        }

        It 'returns failure when health checks fail' {
            Mock Test-WindowsExporterAssets -MockWith { return 'Ready' }
            Mock Get-Service -MockWith { return @{ Status = 'Stopped' } }
            Mock Invoke-Nssm
            Mock Test-WindowsExporterHealth -MockWith { return $false }

            Enable-WindowsExporter | Should -Be $false
        }
    }

    Context 'Test-WindowsExporterHealth' {
        BeforeEach {
            Mock Get-Service -MockWith { return @{ Status = 'Running' } }
            Mock Start-Sleep
        }

        It 'sleeps only between failed attempts (retries: <Retries>)' -TestCases @(
            @{ Retries = 5 }
            @{ Retries = 0 }
        ) {
            param($Retries)
            Mock Invoke-WebRequest -MockWith { throw 'endpoint unavailable' }

            Test-WindowsExporterHealth -RetryCount $Retries -RetryInterval 5 | Should -Be $false

            Assert-MockCalled Get-Service -Exactly -Times ($Retries + 1)
            Assert-MockCalled Start-Sleep -Exactly -Times $Retries -ParameterFilter { $Seconds -eq 5 }
        }

        It 'probes the health endpoint on the exporter port and logs the version' {
            $script:requestedUris = @()
            Mock Invoke-WebRequest -MockWith {
                $script:requestedUris += $Uri
                if ($Uri -like '*/version') { return @{ Content = '{"version":"v0.31.2"}' } }
                return @{ Content = 'ok' }
            }
            $global:WindowsExporterTestLogs = @()

            Test-WindowsExporterHealth -RetryCount 0 -RetryInterval 0 | Should -Be $true

            $script:requestedUris | Should -Be @('http://localhost:19100/health', 'http://localhost:19100/version')
            $global:WindowsExporterTestLogs | Should -Contain 'aks-windows-exporter version v0.31.2'
        }

        It 'stays healthy when the version response is not JSON' {
            Mock Invoke-WebRequest -MockWith {
                if ($Uri -like '*/version') { return @{ Content = 'not json' } }
                return @{ Content = 'ok' }
            }

            Test-WindowsExporterHealth -RetryCount 0 -RetryInterval 0 | Should -Be $true
        }

        It 'waits for a pending service even when the health endpoint responds' {
            $script:serviceChecks = 0
            Mock Get-Service -MockWith {
                $script:serviceChecks++
                if ($script:serviceChecks -eq 1) { return @{ Status = 'StartPending' } }
                return @{ Status = 'Running' }
            }
            Mock Invoke-WebRequest -MockWith { return @{ Content = 'ok' } }

            Test-WindowsExporterHealth -RetryCount 1 -RetryInterval 1 | Should -Be $true

            Assert-MockCalled Get-Service -Exactly -Times 2
            Assert-MockCalled Start-Sleep -Exactly -Times 1
        }

        It 'does not accept endpoint health without a running service (<Status>)' -TestCases @(
            @{ Status = 'StartPending' }
            @{ Status = 'Stopped' }
            @{ Status = $null }
        ) {
            param($Status)
            $script:healthServiceStatus = $Status
            Mock Get-Service -MockWith {
                if ($null -eq $script:healthServiceStatus) { return $null }
                return @{ Status = $script:healthServiceStatus }
            }
            Mock Invoke-WebRequest -MockWith { return @{ Content = 'ok' } }

            Test-WindowsExporterHealth -RetryCount 1 -RetryInterval 1 | Should -Be $false

            Assert-MockCalled Get-Service -Exactly -Times 2
        }

        It 'rejects a health response without ok' {
            Mock Invoke-WebRequest -MockWith { return @{ Content = 'unhealthy' } }

            Test-WindowsExporterHealth -RetryCount 0 -RetryInterval 0 | Should -Be $false
        }
    }

    Context 'CSE function bundle' {
        It 'bakes the extension skip marker after staging assets' {
            $builderPath = Join-Path $PSScriptRoot '..\..\..\vhdbuilder\packer\windows\configure-windows-vhd.ps1'
            $builder = Get-Content -Path $builderPath -Raw
            $stage = (($builder -split 'function Install-WindowsExporterOnVHD', 2)[1] -split 'function Set-WinRmServiceDelayedStart', 2)[0]

            $stage | Should -Match 'New-Item -ItemType File -Path "C:\\k\\skip_vhd_windows_exporter" -Force -ErrorAction Stop'
            $stage.IndexOf('New-Item -ItemType File -Path "C:\k\skip_vhd_windows_exporter"') |
                Should -BeGreaterThan $stage.IndexOf('Copy-Item -Path $exporterConfigSrc')
        }

        It 'continues BasePrep after exporter registration failure or exception' {
            $templatePath = Join-Path $PSScriptRoot '..\..\..\parts\windows\kuberneteswindowssetup.ps1.template'
            $template = Get-Content -Path $templatePath -Raw
            $registerBlock = [regex]::Match($template, '(?s)    if \(Get-Command -Name Register-WindowsExporter.*?(?=    Set-Explorer)').Value
            $registerBlock | Should -Not -BeNullOrEmpty
            Mock Logs-To-Event
            Mock Register-WindowsExporter -MockWith { return $false }

            $result = & { & ([scriptblock]::Create($registerBlock)); 'continued' }
            $result | Should -Be 'continued'
            Mock Register-WindowsExporter -MockWith { throw 'unexpected registration failure' }
            $result = & { & ([scriptblock]::Create($registerBlock)); 'continued' }
            $result | Should -Be 'continued'
        }

        It 'continues NodePrep after exporter failure or exception' {
            $templatePath = Join-Path $PSScriptRoot '..\..\..\parts\windows\kuberneteswindowssetup.ps1.template'
            $template = Get-Content -Path $templatePath -Raw
            $enableBlock = [regex]::Match($template, '(?s)    if \(Get-Command -Name Enable-WindowsExporter.*?(?=    Write-Log "Starting NodePrep)').Value
            $enableBlock | Should -Not -BeNullOrEmpty
            Mock Logs-To-Event
            Mock Enable-WindowsExporter -MockWith { return $false }

            $result = & { & ([scriptblock]::Create($enableBlock)); 'continued' }
            $result | Should -Be 'continued'
            Mock Enable-WindowsExporter -MockWith { throw 'unexpected startup failure' }
            $result = & { & ([scriptblock]::Create($enableBlock)); 'continued' }
            $result | Should -Be 'continued'
        }

        It 'uses the Linux exporter port for all Windows exporter endpoints' {
            $configPath = Join-Path $PSScriptRoot '..\..\..\vhdbuilder\packer\windows\windows-exporter\windows-exporter-config.yml'

            $global:WindowsExporterPort | Should -Be 19100
            Get-Content -Path $configPath -Raw | Should -Match 'listen-address: ":19100"'
            Get-Content -Path $configPath -Raw | Should -Match 'enabled: ".*pagefile.*"'
            Get-Content -Path $configPath -Raw | Should -Match 'include: "\(\?i\)aks-windows-exporter\|kubelet\|kubeproxy\|containerd\|hns\|csi-proxy"'
        }

        It 'loads the windows exporter functions' {
            $allScript = Get-Content -Path (Join-Path $PSScriptRoot 'all.ps1') -Raw

            $allScript | Should -Match '(?m)^\. "\$WINDOWS_SCRIPTS_DIRECTORY\\windowsexporterfunc\.ps1"\r?$'
        }

        It 'registers the exporter disabled in BasePrep and enables it in NodePrep' {
            $templatePath = Join-Path $PSScriptRoot '..\..\..\parts\windows\kuberneteswindowssetup.ps1.template'
            $template = Get-Content -Path $templatePath -Raw
            $basePrep = (($template -split 'function BasePrep \{', 2)[1] -split 'function NodePrep \{', 2)[0]
            $nodePrep = (($template -split 'function NodePrep \{', 2)[1] -split '(?m)^try \{', 2)[0]

            $basePrep | Should -Match 'Register-WindowsExporter'
            $basePrep | Should -Not -Match 'Enable-WindowsExporter'
            $nodePrep | Should -Match 'Enable-WindowsExporter'
            $nodePrep | Should -Not -Match 'Register-WindowsExporter'
            # nssm.exe is delivered by the kube package.
            $basePrep.IndexOf('Register-WindowsExporter') |
                Should -BeGreaterThan $basePrep.IndexOf('Get-KubePackage')
        }
    }
}
