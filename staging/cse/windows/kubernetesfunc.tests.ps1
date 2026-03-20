if (-not (Get-PSDrive -Name C -ErrorAction SilentlyContinue)) {
    New-PSDrive -Name C -PSProvider FileSystem -Root ([System.IO.Path]::GetTempPath()) | Out-Null
}

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

$helperScriptPath = Join-Path $PSScriptRoot '..\..\..\parts\windows\windowscsehelper.ps1'
$scriptUnderTestPath = Join-Path $PSScriptRoot 'kubernetesfunc.ps1'

. $helperScriptPath
. $scriptUnderTestPath

Describe 'Get-CustomCloudCertEndpointModeFromLocation' {
    It 'returns legacy for ussec regions' {
        Get-CustomCloudCertEndpointModeFromLocation -Location 'ussecwest' | Should Be 'legacy'
    }

    It 'returns legacy for usnat regions' {
        Get-CustomCloudCertEndpointModeFromLocation -Location 'usnatcentral' | Should Be 'legacy'
    }

    It 'returns rcv1p for public regions' {
        Get-CustomCloudCertEndpointModeFromLocation -Location 'southcentralus' | Should Be 'rcv1p'
    }

    It 'handles mixed-case input' {
        Get-CustomCloudCertEndpointModeFromLocation -Location 'UsSeCeast' | Should Be 'legacy'
    }
}

Describe 'Register-CACertificatesRefreshTask' {
    BeforeEach {
        $script:lastScheduledTaskArgument = $null

        Mock Logs-To-Event
        Mock New-ScheduledTaskPrincipal -MockWith { return @{ Kind = 'principal' } }
        Mock New-JobTrigger -MockWith { return @{ Kind = 'trigger' } }
        Mock New-ScheduledTask -MockWith { return @{ Kind = 'definition' } }
        Mock Register-ScheduledTask
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
        $script:lastScheduledTaskArgument | Should Match ([regex]::Escape("Get-CACertificates -Location 'southcentralus'"))
    }
}

Describe 'Should-InstallCACertificatesRefreshTask' {
    BeforeEach {
    }

    It 'returns true for legacy regions without calling the opt-in endpoint' {
        Mock Retry-Command

        $result = Should-InstallCACertificatesRefreshTask -Location 'ussecwest'

        $result | Should Be $true
        Assert-MockCalled -CommandName Retry-Command -Exactly -Times 0
    }

    It 'returns true for rcv1p regions when opt-in is enabled' {
        Mock Retry-Command -MockWith {
            return [PSCustomObject]@{ Content = 'IsOptedInForRootCerts=true' }
        }

        $result = Should-InstallCACertificatesRefreshTask -Location 'southcentralus'

        $result | Should Be $true
        Assert-MockCalled -CommandName Retry-Command -Exactly -Times 1 -ParameterFilter { $Args.Uri -eq 'http://168.63.129.16/acms/isOptedInForRootCerts' }
    }

    It 'returns false for rcv1p regions when opt-in is disabled' {
        Mock Retry-Command -MockWith {
            return [PSCustomObject]@{ Content = 'IsOptedInForRootCerts=false' }
        }

        $result = Should-InstallCACertificatesRefreshTask -Location 'southcentralus'

        $result | Should Be $false
    }
}

Describe 'Get-CACertificates' {
    BeforeEach {
        Mock Create-Directory -MockWith {
            param($FullPath, $DirectoryUsage)
            if (-not (Test-Path $FullPath)) {
                New-Item -Path $FullPath -ItemType Directory -Force | Out-Null
            }
        }

        if (Test-Path 'C:\ca') {
            Remove-Item -Path 'C:\ca' -Recurse -Force
        }
    }

    It 'uses the legacy endpoint when location is a ussec/usnat region' {
        Mock Retry-Command -MockWith {
            param($Command, $Args, $Retries, $RetryDelaySeconds)
            return [PSCustomObject]@{
                Content = '{"Certificates":[{"Name":"legacy.crt","CertBody":"legacy-body"}]}'
            }
        }

        $result = Get-CACertificates -Location 'ussecwest'

        $result | Should Be $true
        Assert-MockCalled -CommandName Retry-Command -Exactly -Times 1 -ParameterFilter { $Args.Uri -eq 'http://168.63.129.16/machine?comp=acmspackage&type=cacertificates&ext=json' }
        Assert-MockCalled -CommandName Retry-Command -Exactly -Times 0 -ParameterFilter { $Args.Uri -eq 'http://168.63.129.16/acms/isOptedInForRootCerts' }
    }

    It 'returns false when certificate retrieval throws' {
        Mock Retry-Command -MockWith {
            throw 'simulated retrieval failure'
        }

        $result = Get-CACertificates -Location 'southcentralus'

        $result | Should Be $false
    }
}
