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

        It 'takes ownership of an existing running service' {
            Mock Test-Path -MockWith { return $true }
            Mock Get-Service -MockWith { return @{ Status = 'Running' } }
            Mock Invoke-WindowsExporterNssm
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
    }

    Context 'CSE function bundle' {
        It 'uses the Linux exporter port for all Windows exporter endpoints' {
            $configPath = Join-Path $PSScriptRoot '..\..\..\parts\windows\windowsexporter\windows-exporter-config.yml'
            $healthScriptPath = Join-Path $PSScriptRoot '..\..\..\parts\windows\windowsexporter\windows-exporter-health.ps1'

            $global:WindowsExporterPort | Should -Be 19100
            Get-Content -Path $configPath -Raw | Should -Match 'listen-address: ":19100"'
            Get-Content -Path $configPath -Raw | Should -Match 'include: "\(\?i\)aks-windows-exporter\|kubelet\|kubeproxy\|containerd\|hns\|csi-proxy"'
            Get-Content -Path $healthScriptPath -Raw | Should -Match 'localhost:19100/'
        }

        It 'loads the windows exporter functions' {
            $allScript = Get-Content -Path (Join-Path $PSScriptRoot 'all.ps1') -Raw

            $allScript | Should -Match '(?m)^\. c:\\AzureData\\windows\\windowsexporterfunc\.ps1\r?$'
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
