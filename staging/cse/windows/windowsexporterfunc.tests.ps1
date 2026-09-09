Describe 'Windows exporter CSE functions' {
    BeforeAll {
        . $PSScriptRoot\..\..\..\parts\windows\windowscsehelper.ps1
        . $PSCommandPath.Replace('.tests.ps1','.ps1')

        function Write-Log {
            param($Message)
            Write-Host "LOG: $Message"
        }
    }

    Context 'Install-WindowsExporter' {
        BeforeEach {
            Mock New-Item
        }

        It 'no-ops when the VHD assets marker is absent' {
            Mock Test-Path -MockWith { return $false }

            Install-WindowsExporter | Should -Be $true
            Assert-MockCalled New-Item -Exactly -Times 0
        }

        It 'fails when the assets marker is present but the binary is absent' {
            Mock Test-Path -MockWith {
                param($Path)
                return $Path -eq $global:WindowsExporterAssetsFile
            }

            Install-WindowsExporter | Should -Be $false
            Assert-MockCalled New-Item -Exactly -Times 0
        }

        It 'fails when the assets marker and binary are present but the config is absent' {
            Mock Test-Path -MockWith {
                param($Path)
                return $Path -in @($global:WindowsExporterAssetsFile, $global:WindowsExporterBinary)
            }

            Install-WindowsExporter | Should -Be $false
            Assert-MockCalled New-Item -Exactly -Times 0
        }

        It 'leaves ownership with the extension when nssm is absent after assets are present' {
            Mock Test-Path -MockWith {
                param($Path)
                return $Path -ne $global:WindowsExporterNssm
            }

            Install-WindowsExporter | Should -Be $false
            Assert-MockCalled New-Item -Exactly -Times 0
        }

        It 'installs, configures, and starts a healthy exporter' {
            Mock Test-Path -MockWith { return $true }
            Mock Get-Service -MockWith { return $null }
            Mock Invoke-WindowsExporterNssm
            Mock Test-WindowsExporterHealth -MockWith { return $true }
            Mock New-Item

            Install-WindowsExporter | Should -Be $true

            Assert-MockCalled Invoke-WindowsExporterNssm -Exactly -Times 1 -ParameterFilter {
                $Arguments[0] -eq 'install' -and
                $Arguments[1] -eq $global:WindowsExporterServiceName -and
                $Arguments[2] -eq $global:WindowsExporterBinary
            }
            Assert-MockCalled Invoke-WindowsExporterNssm -Exactly -Times 1 -ParameterFilter {
                $Arguments[0] -eq 'start' -and $Arguments[1] -eq $global:WindowsExporterServiceName
            }
            Assert-MockCalled New-Item -Exactly -Times 1 -ParameterFilter {
                $Path -eq $global:WindowsExporterSkipFile
            }
        }

        It 'configures before restarting an existing <Status> service' -TestCases @(
            @{ Status = 'Running' }
            @{ Status = 'Paused' }
        ) {
            param($Status)
            Mock Test-Path -MockWith { return $true }
            $script:exporterStatus = $Status
            $script:exporterOperations = @()
            Mock Get-Service -MockWith { return @{ Status = $script:exporterStatus } }
            Mock Invoke-WindowsExporterNssm -MockWith {
                param($Arguments)
                $script:exporterOperations += $Arguments[0]
            }
            Mock Test-WindowsExporterHealth -MockWith { return $true }
            Mock New-Item

            Install-WindowsExporter | Should -Be $true

            Assert-MockCalled Invoke-WindowsExporterNssm -Exactly -Times 0 -ParameterFilter {
                $Arguments[0] -eq 'install'
            }
            Assert-MockCalled Invoke-WindowsExporterNssm -Exactly -Times 1 -ParameterFilter {
                $Arguments[0] -eq 'stop' -and $Arguments[1] -eq $global:WindowsExporterServiceName
            }
            Assert-MockCalled Invoke-WindowsExporterNssm -Exactly -Times 1 -ParameterFilter {
                $Arguments[0] -eq 'set' -and
                $Arguments[1] -eq $global:WindowsExporterServiceName -and
                $Arguments[2] -eq 'Application' -and
                $Arguments[3] -eq $global:WindowsExporterBinary
            }
            Assert-MockCalled Invoke-WindowsExporterNssm -Exactly -Times 1 -ParameterFilter {
                $Arguments[0] -eq 'start'
            }
            $script:exporterOperations[-2] | Should -Be 'stop'
            $script:exporterOperations[-1] | Should -Be 'start'
            @($script:exporterOperations[0..($script:exporterOperations.Count - 3)] | Where-Object { $_ -ne 'set' }).Count | Should -Be 0
        }

        It 'starts an existing stopped service without stopping it again' {
            Mock Test-Path -MockWith { return $true }
            Mock Get-Service -MockWith { return @{ Status = 'Stopped' } }
            Mock Invoke-WindowsExporterNssm
            Mock Test-WindowsExporterHealth -MockWith { return $true }

            Install-WindowsExporter | Should -Be $true

            Assert-MockCalled Invoke-WindowsExporterNssm -Exactly -Times 0 -ParameterFilter { $Arguments[0] -eq 'stop' }
            Assert-MockCalled Invoke-WindowsExporterNssm -Exactly -Times 1 -ParameterFilter { $Arguments[0] -eq 'start' }
        }

        It 'does not stop a running exporter when a later configuration setting fails' {
            Mock Test-Path -MockWith { return $true }
            Mock Get-Service -MockWith { return @{ Status = 'Running' } }
            Mock Invoke-WindowsExporterNssm -MockWith {
                param($Arguments)
                if ($Arguments[0] -eq 'set' -and $Arguments[2] -eq 'AppRotateBytes') {
                    throw 'configuration failed'
                }
            }

            Install-WindowsExporter | Should -Be $false

            Assert-MockCalled Invoke-WindowsExporterNssm -Exactly -Times 1 -ParameterFilter { $Arguments[2] -eq 'Application' }
            Assert-MockCalled Invoke-WindowsExporterNssm -Exactly -Times 0 -ParameterFilter { $Arguments[0] -eq 'stop' }
            Assert-MockCalled Invoke-WindowsExporterNssm -Exactly -Times 0 -ParameterFilter { $Arguments[0] -eq 'start' }
            Assert-MockCalled New-Item -Exactly -Times 0
        }

        It 'does not claim ownership when starting the reconfigured service fails' {
            Mock Test-Path -MockWith { return $true }
            Mock Get-Service -MockWith { return @{ Status = 'Running' } }
            Mock Invoke-WindowsExporterNssm -MockWith {
                param($Arguments)
                if ($Arguments[0] -eq 'start') { throw 'start failed' }
            }
            Mock Test-WindowsExporterHealth -MockWith { return $false }

            Install-WindowsExporter | Should -Be $false

            Assert-MockCalled Invoke-WindowsExporterNssm -Exactly -Times 1 -ParameterFilter { $Arguments[0] -eq 'stop' }
            Assert-MockCalled New-Item -Exactly -Times 0
        }

        It 'claims ownership when a nonzero start result is followed by service health' {
            Mock Test-Path -MockWith { return $true }
            Mock Get-Service -MockWith { return $null }
            Mock Invoke-WindowsExporterNssm -MockWith {
                param($Arguments)
                if ($Arguments[0] -eq 'start') { throw 'nssm.exe start failed with exit code 1: SERVICE_START_PENDING' }
            }
            Mock Test-WindowsExporterHealth -MockWith { return $true }

            Install-WindowsExporter | Should -Be $true

            Assert-MockCalled Invoke-WindowsExporterNssm -Exactly -Times 1 -ParameterFilter { $Arguments[0] -eq 'start' }
            Assert-MockCalled Test-WindowsExporterHealth -Exactly -Times 1
            Assert-MockCalled New-Item -Exactly -Times 1 -ParameterFilter { $Path -eq $global:WindowsExporterSkipFile }
        }

        It 'leaves ownership with the extension when nssm configuration fails' {
            Mock Test-Path -MockWith { return $true }
            Mock Get-Service -MockWith { return $null }
            Mock Invoke-WindowsExporterNssm -MockWith { throw 'nssm failed' }

            Install-WindowsExporter | Should -Be $false
            Assert-MockCalled New-Item -Exactly -Times 0
        }

        It 'leaves ownership with the extension when the service stays unhealthy' {
            Mock Test-Path -MockWith { return $true }
            Mock Get-Service -MockWith { return $null }
            Mock Invoke-WindowsExporterNssm
            Mock Test-WindowsExporterHealth -MockWith { return $false }
            Mock New-Item

            Install-WindowsExporter | Should -Be $false
            Assert-MockCalled New-Item -Exactly -Times 0 -ParameterFilter {
                $Path -eq $global:WindowsExporterSkipFile
            }
        }
    }

    Context 'Test-WindowsExporterHealth' {
        BeforeEach {
            Mock Get-Service -MockWith { return @{ Status = 'Running' } }
            Mock Start-Sleep
        }

        It 'uses the baked health script when it is present' {
            $global:WindowsExporterHealthScript = Join-Path $TestDrive 'windows-exporter-health.ps1'
            @'
function Get-Health {
    return "ok"
}

function Get-Version {
    return "v0.31.2"
}
'@ | Set-Content -Path $global:WindowsExporterHealthScript -Force

            Test-WindowsExporterHealth -RetryCount 0 -RetryInterval 0 | Should -Be $true
        }

        It 'uses a native PowerShell endpoint probe when the baked health script is absent' {
            $global:WindowsExporterHealthScript = Join-Path $TestDrive 'missing-health.ps1'

            Mock Invoke-WebRequest -MockWith {
                return @{ Content = 'ok' }
            }

            Test-WindowsExporterHealth -RetryCount 0 -RetryInterval 0 | Should -Be $true

            Assert-MockCalled Invoke-WebRequest -Exactly -Times 1
        }

        It 'waits for a pending service even when the health endpoint responds' {
            $global:WindowsExporterHealthScript = Join-Path $TestDrive 'missing-health.ps1'
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
            $global:WindowsExporterHealthScript = Join-Path $TestDrive 'missing-health.ps1'
            $script:healthServiceStatus = $Status
            Mock Get-Service -MockWith {
                if ($null -eq $script:healthServiceStatus) { return $null }
                return @{ Status = $script:healthServiceStatus }
            }
            Mock Invoke-WebRequest -MockWith { return @{ Content = 'ok' } }

            Test-WindowsExporterHealth -RetryCount 1 -RetryInterval 1 | Should -Be $false

            Assert-MockCalled Get-Service -Exactly -Times 2
        }

        It 'does not accept baked health script success for a stopped service' {
            $global:WindowsExporterHealthScript = Join-Path $TestDrive 'healthy-script.ps1'
            'function Get-Health { return "ok" }; function Get-Version { return "0.31.2" }' |
                Set-Content -Path $global:WindowsExporterHealthScript
            Mock Get-Service -MockWith { return @{ Status = 'Stopped' } }

            Test-WindowsExporterHealth -RetryCount 0 -RetryInterval 0 | Should -Be $false
        }
    }

    Context 'CSE function bundle' {
        It 'uses the Linux exporter port for all Windows exporter endpoints' {
            $configPath = Join-Path $PSScriptRoot '..\..\..\parts\windows\windowsexporter\windows-exporter-config.yml'
            $healthScriptPath = Join-Path $PSScriptRoot '..\..\..\parts\windows\windowsexporter\windows-exporter-health.ps1'

            $global:WindowsExporterPort | Should -Be 19100
            Get-Content -Path $configPath -Raw | Should -Match 'listen-address: ":19100"'
            Get-Content -Path $configPath -Raw | Should -Match 'enabled: ".*pagefile.*"'
            Get-Content -Path $configPath -Raw | Should -Match 'include: "\(\?i\)aks-windows-exporter\|kubelet\|kubeproxy\|containerd\|hns\|csi-proxy"'
            Get-Content -Path $healthScriptPath -Raw | Should -Match 'localhost:19100/'
        }

        It 'loads the windows exporter functions' {
            $allScript = Get-Content -Path (Join-Path $PSScriptRoot 'all.ps1') -Raw

            $allScript | Should -Match '(?m)^\. "\$WINDOWS_SCRIPTS_DIRECTORY\\windowsexporterfunc\.ps1"\r?$'
        }

        It 'registers the exporter during NodePrep rather than BasePrep' {
            $templatePath = Join-Path $PSScriptRoot '..\..\..\parts\windows\kuberneteswindowssetup.ps1.template'
            $template = Get-Content -Path $templatePath -Raw
            $basePrep = (($template -split 'function BasePrep \{', 2)[1] -split 'function NodePrep \{', 2)[0]
            $nodePrep = (($template -split 'function NodePrep \{', 2)[1] -split '(?m)^try \{', 2)[0]

            $basePrep | Should -Not -Match 'Install-WindowsExporter'
            $nodePrep | Should -Match 'Install-WindowsExporter'
        }
    }
}
