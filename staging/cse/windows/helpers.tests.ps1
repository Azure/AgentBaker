BeforeAll {
    . $PSScriptRoot\..\..\..\parts\windows\windowscsehelper.ps1
    . $PSCommandPath.Replace('.tests.ps1', '.ps1')

    # Get-Service is a Windows-only cmdlet; stub it so Mock can override it when
    # tests run in isolation (e.g. locally on non-Windows, outside the full suite).
    function Get-Service {}
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
