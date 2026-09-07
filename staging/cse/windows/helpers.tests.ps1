BeforeAll {
    . $PSScriptRoot\..\..\..\parts\windows\windowscsehelper.ps1
    . $PSCommandPath.Replace('.tests.ps1', '.ps1')

    # Get-Service and sc.exe are Windows-only; stub them so Mock can override them when
    # tests run in isolation (e.g. locally on non-Windows, outside the full suite).
    function Get-Service {}
    function sc.exe {}
}

Describe 'Remove-ServiceIfExists' {
    Context 'when the service does not exist' {
        BeforeEach {
            $script:scExeCallCount = 0
            Mock Get-Service -MockWith { return $null }
            Mock sc.exe -MockWith { $script:scExeCallCount++ }
        }

        It 'does not call sc.exe' {
            Remove-ServiceIfExists -ServiceName 'some-service'

            $script:scExeCallCount | Should -Be 0
        }
    }

    Context 'when the service already exists' {
        BeforeEach {
            $script:scExeCallCount = 0
            $mockExistingSvc = [PSCustomObject]@{Name = 'some-service'; Status = 'Stopped'}
            Mock Get-Service -MockWith { return $mockExistingSvc }
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

        It 'does not throw when sc.exe delete fails (best-effort cleanup)' {
            Mock sc.exe -MockWith { $global:LASTEXITCODE = 1 }

            { Remove-ServiceIfExists -ServiceName 'some-service' } | Should -Not -Throw
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
