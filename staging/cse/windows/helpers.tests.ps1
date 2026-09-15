BeforeAll {
    . $PSScriptRoot\..\..\..\parts\windows\windowscsehelper.ps1
    . $PSCommandPath.Replace('.tests.ps1', '.ps1')

    # Service cmdlets and sc.exe are Windows-only; stub them for isolated non-Windows test runs.
    function Get-Service {}
    function sc.exe {}
}

Describe 'Remove-ServiceIfExists' {
    BeforeEach {
        $script:getServiceCallCount = 0
        Mock Start-Sleep
    }

    Context 'when the service does not exist' {
        BeforeEach {
            $script:scExeCallCount = 0
            $script:scQueryCallCount = 0
            Mock Get-Service -MockWith { return $null }
            Mock sc.exe -MockWith {
                $script:scExeCallCount++
                if ($args[0] -eq 'query') { $script:scQueryCallCount++ }
                $global:LASTEXITCODE = 1060
            }
        }

        It 'probes with sc.exe query and returns without calling delete when truly absent' {
            Remove-ServiceIfExists -ServiceName 'some-service'

            $script:scQueryCallCount | Should -Be 1
            $script:scExeCallCount | Should -Be 1
            Assert-MockCalled -CommandName Start-Sleep -Exactly -Times 0
        }

        It 'waits out a service still marked for deletion from a prior attempt, then returns' {
            Mock sc.exe -MockWith {
                $script:scExeCallCount++
                $script:scQueryCallCount++
                $global:LASTEXITCODE = if ($script:scQueryCallCount -lt 3) { 1072 } else { 1060 }
            }

            { Remove-ServiceIfExists -ServiceName 'some-service' } | Should -Not -Throw

            $script:scQueryCallCount | Should -Be 3
            Assert-MockCalled -CommandName Start-Sleep -Exactly -Times 2
        }

        It 'throws when a service marked for deletion never clears' {
            Mock sc.exe -MockWith {
                $script:scExeCallCount++
                $global:LASTEXITCODE = 1072
            }

            { Remove-ServiceIfExists -ServiceName 'some-service' } | Should -Throw '*Timed out*'

            Assert-MockCalled -CommandName Start-Sleep -Exactly -Times 30
        }

        It 'throws immediately, without waiting, when sc.exe query returns an unexpected exit code' {
            Mock sc.exe -MockWith {
                $script:scExeCallCount++
                $global:LASTEXITCODE = 5
            }

            { Remove-ServiceIfExists -ServiceName 'some-service' } | Should -Throw '*exit code 5*'

            $script:scExeCallCount | Should -Be 1
            Assert-MockCalled -CommandName Start-Sleep -Exactly -Times 0
        }
    }

    Context 'when the service already exists' {
        BeforeEach {
            $script:scExeCallCount = 0
            $script:scStopCallCount = 0
            $script:scDeleteCallCount = 0
            $script:scQueryCallCount = 0
            $script:serviceStatus = 'Stopped'
            Mock Get-Service -MockWith {
                $script:getServiceCallCount++
                if ($script:getServiceCallCount -eq 1) {
                    return [PSCustomObject]@{Name = 'some-service'; Status = $script:serviceStatus}
                }
                return $null
            }
            Mock sc.exe -MockWith {
                $script:scExeCallCount++
                if ($args[0] -eq 'stop') { $script:scStopCallCount++ }
                if ($args[0] -eq 'delete') { $script:scDeleteCallCount++ }
                if ($args[0] -eq 'query') { $script:scQueryCallCount++ }
                # By default, treat the service as already fully removed once we get to the
                # post-delete query loop, so unrelated tests don't need to care about it.
                $global:LASTEXITCODE = if ($args[0] -eq 'query') { 1060 } else { 0 }
            }
        }

        It 'calls sc.exe delete to remove the existing service' {
            Remove-ServiceIfExists -ServiceName 'some-service'

            $script:scDeleteCallCount | Should -Be 1
        }

        It 'does not throw when sc.exe delete succeeds' {
            { Remove-ServiceIfExists -ServiceName 'some-service' } | Should -Not -Throw
        }

        It 'stops a running service before deleting it' {
            $script:serviceStatus = 'Running'

            Remove-ServiceIfExists -ServiceName 'some-service'

            $script:scStopCallCount | Should -Be 1
        }

        It 'does not stop a service that is already stopping' {
            $script:serviceStatus = 'StopPending'

            Remove-ServiceIfExists -ServiceName 'some-service'

            $script:scStopCallCount | Should -Be 0
            $script:scDeleteCallCount | Should -Be 1
        }

        It 'waits for another pending state before stopping the service' {
            Mock Get-Service -MockWith {
                $script:getServiceCallCount++
                switch ($script:getServiceCallCount) {
                    1 { return [PSCustomObject]@{Name = 'some-service'; Status = 'StartPending'} }
                    2 { return [PSCustomObject]@{Name = 'some-service'; Status = 'Running'} }
                    default { return $null }
                }
            }

            Remove-ServiceIfExists -ServiceName 'some-service'

            $script:scStopCallCount | Should -Be 1
            Assert-MockCalled -CommandName Start-Sleep -Exactly -Times 1
        }

        It 'waits out marked-for-deletion when the service disappears from Get-Service during the pending-state wait' {
            Mock Get-Service -MockWith {
                $script:getServiceCallCount++
                switch ($script:getServiceCallCount) {
                    1 { return [PSCustomObject]@{Name = 'some-service'; Status = 'StartPending'} }
                    default { return $null }
                }
            }
            Mock sc.exe -MockWith {
                $script:scExeCallCount++
                if ($args[0] -eq 'query') {
                    $script:scQueryCallCount++
                    $global:LASTEXITCODE = if ($script:scQueryCallCount -lt 2) { 1072 } else { 1060 }
                }
            }

            { Remove-ServiceIfExists -ServiceName 'some-service' } | Should -Not -Throw

            $script:scStopCallCount | Should -Be 0
            $script:scDeleteCallCount | Should -Be 0
            $script:scQueryCallCount | Should -Be 2
            Assert-MockCalled -CommandName Start-Sleep -Exactly -Times 2
        }

        It 'polls without blocking while the service stops, up to 60 seconds' {
            $script:serviceStatus = 'Running'
            Mock Get-Service -MockWith {
                $script:getServiceCallCount++
                if ($script:getServiceCallCount -le 5) {
                    return [PSCustomObject]@{Name = 'some-service'; Status = 'Running'}
                }
                if ($script:getServiceCallCount -eq 6) {
                    return [PSCustomObject]@{Name = 'some-service'; Status = 'Stopped'}
                }
                return $null
            }

            Remove-ServiceIfExists -ServiceName 'some-service'

            $script:scStopCallCount | Should -Be 1
            $script:scDeleteCallCount | Should -Be 1
            Assert-MockCalled -CommandName Start-Sleep -Exactly -Times 4 -ParameterFilter {
                $Seconds -eq 1
            }
        }

        It 'throws when the service does not stop within 60 seconds' {
            $script:serviceStatus = 'Running'
            Mock Get-Service -MockWith {
                return [PSCustomObject]@{Name = 'some-service'; Status = 'Running'}
            }

            { Remove-ServiceIfExists -ServiceName 'some-service' } | Should -Throw '*Timed out*to stop*'

            $script:scStopCallCount | Should -Be 1
            $script:scDeleteCallCount | Should -Be 0
            Assert-MockCalled -CommandName Start-Sleep -Exactly -Times 60
        }

        It 'retries the stop request when the service was mid-transition (1061) and later becomes controllable' {
            $script:serviceStatus = 'Running'
            Mock Get-Service -MockWith {
                $script:getServiceCallCount++
                switch ($script:getServiceCallCount) {
                    1 { return [PSCustomObject]@{Name = 'some-service'; Status = 'Running'} }
                    2 { return [PSCustomObject]@{Name = 'some-service'; Status = 'StartPending'} }
                    3 { return [PSCustomObject]@{Name = 'some-service'; Status = 'Running'} }
                    4 { return [PSCustomObject]@{Name = 'some-service'; Status = 'Stopped'} }
                    default { return $null }
                }
            }
            Mock sc.exe -MockWith {
                $script:scExeCallCount++
                if ($args[0] -eq 'stop') {
                    $script:scStopCallCount++
                    # First stop request races with the service entering a pending state.
                    $global:LASTEXITCODE = if ($script:scStopCallCount -eq 1) { 1061 } else { 0 }
                }
                if ($args[0] -eq 'delete') { $script:scDeleteCallCount++; $global:LASTEXITCODE = 0 }
                if ($args[0] -eq 'query') { $global:LASTEXITCODE = 1060 }
            }

            Remove-ServiceIfExists -ServiceName 'some-service'

            $script:scStopCallCount | Should -Be 2
            $script:scDeleteCallCount | Should -Be 1
        }

        It 'keeps waiting while the service is marked for deletion, then succeeds once fully removed' {
            Mock sc.exe -MockWith {
                $script:scExeCallCount++
                if ($args[0] -eq 'delete') {
                    $script:scDeleteCallCount++
                    $global:LASTEXITCODE = 1072
                }
                if ($args[0] -eq 'query') {
                    $script:scQueryCallCount++
                    $global:LASTEXITCODE = if ($script:scQueryCallCount -lt 3) { 1072 } else { 1060 }
                }
            }

            { Remove-ServiceIfExists -ServiceName 'some-service' } | Should -Not -Throw

            $script:scQueryCallCount | Should -Be 3
            Assert-MockCalled -CommandName Start-Sleep -Exactly -Times 2
        }

        It 'returns immediately when the service is already fully removed' {
            Mock sc.exe -MockWith {
                $script:scExeCallCount++
                $global:LASTEXITCODE = 1060
            }

            { Remove-ServiceIfExists -ServiceName 'some-service' } | Should -Not -Throw

            Assert-MockCalled -CommandName Start-Sleep -Exactly -Times 0
        }

        It 'throws when sc.exe delete fails unexpectedly' {
            Mock sc.exe -MockWith { $global:LASTEXITCODE = 1 }

            { Remove-ServiceIfExists -ServiceName 'some-service' } | Should -Throw '*exit code 1*'
        }

        It 'waits until sc.exe query reports the service is no longer registered' {
            Mock sc.exe -MockWith {
                $script:scExeCallCount++
                if ($args[0] -eq 'delete') { $global:LASTEXITCODE = 0 }
                if ($args[0] -eq 'query') {
                    $script:scQueryCallCount++
                    $global:LASTEXITCODE = if ($script:scQueryCallCount -le 3) { 0 } else { 1060 }
                }
            }

            Remove-ServiceIfExists -ServiceName 'some-service'

            Assert-MockCalled -CommandName Start-Sleep -Exactly -Times 3 -ParameterFilter {
                $Seconds -eq 1
            }
        }

        It 'accepts deletion during the final wait interval' {
            Mock sc.exe -MockWith {
                $script:scExeCallCount++
                if ($args[0] -eq 'delete') { $global:LASTEXITCODE = 0 }
                if ($args[0] -eq 'query') {
                    $script:scQueryCallCount++
                    $global:LASTEXITCODE = if ($script:scQueryCallCount -le 30) { 0 } else { 1060 }
                }
            }

            { Remove-ServiceIfExists -ServiceName 'some-service' } | Should -Not -Throw

            Assert-MockCalled -CommandName Start-Sleep -Exactly -Times 30
        }

        It 'throws when the service remains registered' {
            Mock sc.exe -MockWith {
                $script:scExeCallCount++
                if ($args[0] -eq 'delete') { $global:LASTEXITCODE = 0 }
                if ($args[0] -eq 'query') { $global:LASTEXITCODE = 0 }
            }

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
