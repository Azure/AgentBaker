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
            $script:LastExitCode = $null
            $script:LastErrorMessage = $null

            Mock Set-ExitCode -MockWith {
                param($ExitCode, $ErrorMessage)
                $script:LastExitCode = $ExitCode
                $script:LastErrorMessage = $ErrorMessage
            }
        }

        It 'no-ops when the VHD sentinel is absent' {
            Mock Test-Path -MockWith { return $false }

            { Install-WindowsExporter } | Should -Not -Throw

            Assert-MockCalled Set-ExitCode -Exactly -Times 0
        }

        It 'no-ops when the sentinel is present but the binary is absent' {
            Mock Test-Path -MockWith {
                param($Path)
                return $Path -eq $global:WindowsExporterSkipFile
            }

            { Install-WindowsExporter } | Should -Not -Throw

            Assert-MockCalled Set-ExitCode -Exactly -Times 0
        }

        It 'sets the windows-exporter error code when nssm is absent after assets are present' {
            Mock Test-Path -MockWith {
                param($Path)
                return $Path -ne $global:WindowsExporterNssm
            }

            { Install-WindowsExporter } | Should -Not -Throw

            Assert-MockCalled Set-ExitCode -Exactly -Times 1 -ParameterFilter {
                $ExitCode -eq $global:WINDOWS_CSE_ERROR_WINDOWS_EXPORTER_START_FAIL
            }
            $script:LastExitCode | Should -Be $global:WINDOWS_CSE_ERROR_WINDOWS_EXPORTER_START_FAIL
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
}
