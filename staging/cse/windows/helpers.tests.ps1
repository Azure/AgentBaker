BeforeAll {
    . $PSScriptRoot\..\..\..\parts\windows\windowscsehelper.ps1
    . $PSCommandPath.Replace('.tests.ps1', '.ps1')

    # Service cmdlets and sc.exe are Windows-only; stub them for isolated non-Windows test runs.
    function Get-Service {}
    function Stop-Service { $script:stopServiceCallCount++ }
    function sc.exe {}
}

Describe 'Remove-ServiceIfExists' {
    BeforeEach {
        $script:getServiceCallCount = 0
        $script:stopServiceCallCount = 0
        Mock Start-Sleep
    }

    Context 'when the service does not exist' {
        BeforeEach {
            $script:scExeCallCount = 0
            Mock Get-Service -MockWith { return $null }
            Mock sc.exe -MockWith { $script:scExeCallCount++ }
        }

        It 'does not call sc.exe' {
            Remove-ServiceIfExists -ServiceName 'some-service'

            $script:scExeCallCount | Should -Be 0
            $script:stopServiceCallCount | Should -Be 0
            Assert-MockCalled -CommandName Start-Sleep -Exactly -Times 0
        }
    }

    Context 'when the service already exists' {
        BeforeEach {
            $script:scExeCallCount = 0
            $script:serviceStatus = 'Stopped'
            Mock Get-Service -MockWith {
                $script:getServiceCallCount++
                if ($script:getServiceCallCount -eq 1) {
                    return [PSCustomObject]@{Name = 'some-service'; Status = $script:serviceStatus}
                }
                return $null
            }
        }

        It 'calls sc.exe delete to remove the existing service' {
            Mock sc.exe -MockWith { $script:scExeCallCount++; $global:LASTEXITCODE = 0 }

            Remove-ServiceIfExists -ServiceName 'some-service'

            $script:scExeCallCount | Should -Be 1
        }

        It 'does not throw when sc.exe delete succeeds' {
            Mock sc.exe -MockWith { $global:LASTEXITCODE = 0 }

            { Remove-ServiceIfExists -ServiceName 'some-service' } | Should -Not -Throw
        }

        It 'stops a running service before deleting it' {
            $script:serviceStatus = 'Running'
            Mock sc.exe -MockWith { $global:LASTEXITCODE = 0 }

            Remove-ServiceIfExists -ServiceName 'some-service'

            $script:stopServiceCallCount | Should -Be 1
        }

        It 'waits when the service is already marked for deletion' {
            Mock sc.exe -MockWith { $global:LASTEXITCODE = 1072 }

            { Remove-ServiceIfExists -ServiceName 'some-service' } | Should -Not -Throw

            Assert-MockCalled -CommandName Get-Service -Exactly -Times 2
        }

        It 'throws when sc.exe delete fails unexpectedly' {
            Mock sc.exe -MockWith { $global:LASTEXITCODE = 1 }

            { Remove-ServiceIfExists -ServiceName 'some-service' } | Should -Throw '*exit code 1*'
        }

        It 'waits until the service is no longer registered' {
            Mock Get-Service -MockWith {
                $script:getServiceCallCount++
                if ($script:getServiceCallCount -le 3) {
                    return [PSCustomObject]@{Name = 'some-service'; Status = $script:serviceStatus}
                }
                return $null
            }
            Mock sc.exe -MockWith { $global:LASTEXITCODE = 0 }

            Remove-ServiceIfExists -ServiceName 'some-service'

            Assert-MockCalled -CommandName Start-Sleep -Exactly -Times 2 -ParameterFilter {
                $Seconds -eq 1
            }
        }

        It 'throws when the service remains registered' {
            Mock Get-Service -MockWith {
                return [PSCustomObject]@{Name = 'some-service'; Status = $script:serviceStatus}
            }
            Mock sc.exe -MockWith { $global:LASTEXITCODE = 0 }

            { Remove-ServiceIfExists -ServiceName 'some-service' } | Should -Throw '*Timed out*'

            Assert-MockCalled -CommandName Start-Sleep -Exactly -Times 30
        }
    }
}

Describe 'Invoke-Nssm' {
    BeforeEach {
        $script:nssmInvocations = @()
    }

    It 'does not throw when nssm.exe succeeds' {
        Mock Invoke-NssmExe -MockWith {
            $script:nssmInvocations += , @($NssmArguments)
            $global:LASTEXITCODE = 0
            return 'ok'
        }

        { Invoke-Nssm -KubeDir 'C:\k' -NssmArguments 'install', 'some-service', 'C:\k\some-service.exe' } | Should -Not -Throw

        $script:nssmInvocations.Count | Should -Be 1
        $script:nssmInvocations[0] | Should -Be @('install', 'some-service', 'C:\k\some-service.exe')
    }

    It 'throws with the exit code when nssm.exe fails' {
        Mock Invoke-NssmExe -MockWith {
            $global:LASTEXITCODE = 1
            return $null
        }

        { Invoke-Nssm -KubeDir 'C:\k' -NssmArguments 'install', 'some-service', 'C:\k\some-service.exe' } |
            Should -Throw '*failed (exit code 1)*'
    }
}
