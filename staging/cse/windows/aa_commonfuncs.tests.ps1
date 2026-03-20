BeforeAll {
    # RemoveNulls filter is defined in windowscsehelper.ps1 and is available at runtime
    # via dot-sourcing in kuberneteswindowssetup.ps1, but we need to define it here for tests.
    filter RemoveNulls { $_ -replace '\0', '' }

    . $PSCommandPath.Replace('.tests.ps1', '.ps1')
}

Describe 'Invoke-Nssm' {
    BeforeEach {
        $script:NssmInvocations = @()
        Mock Invoke-NssmExe -MockWith {
            param(
                [Parameter(Mandatory=$true)][string]$KubeDir,
                [Parameter(Mandatory=$true, ValueFromRemainingArguments=$true)][string[]]$NssmArguments
            )
            $script:NssmInvocations += [PSCustomObject]@{
                KubeDir       = $KubeDir
                NssmArguments = $NssmArguments
            }
            $global:LASTEXITCODE = 0
        }
    }

    Context 'when nssm.exe succeeds (exit code 0)' {
        It 'does not throw for an install command' {
            { Invoke-Nssm -KubeDir 'C:\k' install myservice 'C:\path\to\service.exe' } | Should -Not -Throw
        }

        It 'does not throw for a set command' {
            { Invoke-Nssm -KubeDir 'C:\k' set myservice AppDirectory 'C:\k' } | Should -Not -Throw
        }

        It 'passes KubeDir and all arguments to Invoke-NssmExe' {
            Invoke-Nssm -KubeDir 'C:\k' set myservice Description 'my service'

            Assert-MockCalled -CommandName Invoke-NssmExe -Exactly -Times 1 -ParameterFilter {
                $KubeDir -eq 'C:\k' -and
                $NssmArguments[0] -eq 'set' -and
                $NssmArguments[1] -eq 'myservice' -and
                $NssmArguments[2] -eq 'Description' -and
                $NssmArguments[3] -eq 'my service'
            }
        }
    }

    Context 'when nssm.exe fails (non-zero exit code)' {
        BeforeEach {
            Mock Invoke-NssmExe -MockWith {
                param(
                    [Parameter(Mandatory=$true)][string]$KubeDir,
                    [Parameter(Mandatory=$true, ValueFromRemainingArguments=$true)][string[]]$NssmArguments
                )
                $global:LASTEXITCODE = 1
            }
        }

        It 'throws when install fails' {
            { Invoke-Nssm -KubeDir 'C:\k' install myservice 'C:\path\to\service.exe' } |
                Should -Throw "*nssm.exe install myservice*failed*exit code 1*"
        }

        It 'throws when a set command fails' {
            { Invoke-Nssm -KubeDir 'C:\k' set myservice AppDirectory 'C:\k' } |
                Should -Throw "*nssm.exe set myservice AppDirectory*failed*exit code 1*"
        }

        It 'includes the exit code in the error message' {
            Mock Invoke-NssmExe -MockWith { $global:LASTEXITCODE = 5 }

            { Invoke-Nssm -KubeDir 'C:\k' set myservice Start SERVICE_DEMAND_START } |
                Should -Throw "*exit code 5*"
        }

        It 'includes the nssm arguments in the error message' {
            Mock Invoke-NssmExe -MockWith { $global:LASTEXITCODE = 1 }

            { Invoke-Nssm -KubeDir 'C:\k' set containerd AppRotateBytes 10485760 } |
                Should -Throw "*set containerd AppRotateBytes 10485760*"
        }
    }

    Context 'Invoke-NssmExe delegation' {
        It 'calls Invoke-NssmExe exactly once per Invoke-Nssm call' {
            Invoke-Nssm -KubeDir 'C:\k' set myservice DisplayName 'My Service'
            Assert-MockCalled -CommandName Invoke-NssmExe -Exactly -Times 1
        }

        It 'forwards the correct KubeDir to Invoke-NssmExe' {
            Invoke-Nssm -KubeDir 'C:\custom\dir' set myservice Start SERVICE_AUTO_START
            Assert-MockCalled -CommandName Invoke-NssmExe -Exactly -Times 1 -ParameterFilter {
                $KubeDir -eq 'C:\custom\dir'
            }
        }

        It 'forwards a path argument that contains spaces to Invoke-NssmExe as a single element' {
            Invoke-Nssm -KubeDir 'C:\k' install myservice 'C:\Program Files\myapp\service.exe'
            Assert-MockCalled -CommandName Invoke-NssmExe -Exactly -Times 1 -ParameterFilter {
                $NssmArguments[2] -eq 'C:\Program Files\myapp\service.exe'
            }
        }
    }
}
