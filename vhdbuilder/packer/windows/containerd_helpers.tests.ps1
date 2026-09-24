BeforeAll {
    . "$PSScriptRoot/containerd_helpers.ps1"
}

Describe "Test-ContainerdReady" {
    AfterEach {
        Remove-Item Function:\ctr.exe -ErrorAction SilentlyContinue
    }

    It "returns ready when ctr connects successfully" {
        function global:ctr.exe {
            $global:LASTEXITCODE = 0
            return "server version"
        }

        $result = Test-ContainerdReady

        $result.Ready | Should -BeTrue
        $result.Output | Should -Be "server version"
    }

    It "returns not ready when Windows PowerShell promotes native stderr to NativeCommandError" {
        function global:ctr.exe {
            $global:LASTEXITCODE = 1
            $exception = [System.Management.Automation.RemoteException]::new("pipe unavailable")
            $errorRecord = [System.Management.Automation.ErrorRecord]::new(
                $exception,
                "NativeCommandError",
                [System.Management.Automation.ErrorCategory]::NotSpecified,
                $null
            )
            throw $errorRecord
        }

        $result = Test-ContainerdReady

        $result.Ready | Should -BeFalse
        $result.Output | Should -Match "pipe unavailable"
    }

    It "does not hide unexpected ctr invocation errors" {
        function global:ctr.exe {
            throw "unexpected failure"
        }

        { Test-ContainerdReady } | Should -Throw "*unexpected failure*"
    }
}

Describe "Invoke-WithContainerd" {
    BeforeEach {
        $script:job = [pscustomobject]@{
            Id = 1
            State = "Running"
        }

        Mock Start-Job { return $script:job }
        Mock Get-Job { return $script:job }
        Mock Start-Sleep {}
        Mock Receive-ContainerdJobOutput { return "containerd diagnostic output" }
        Mock Remove-ContainerdJob {}
    }

    It "retries until containerd is ready and runs the command" {
        $script:probeCount = 0
        Mock Test-ContainerdReady {
            $script:probeCount++
            return [pscustomobject]@{
                Ready = ($script:probeCount -eq 3)
                Output = "probe $script:probeCount"
            }
        }

        $result = Invoke-WithContainerd -MaxAttempts 3 -DelaySeconds 0 -ScriptBlock { "images" }

        $result | Should -Be "images"
        Should -Invoke Test-ContainerdReady -Times 3
        Should -Invoke Start-Sleep -Times 2
        Should -Invoke Remove-ContainerdJob -Times 1
    }

    It "fails immediately with diagnostics when containerd exits" {
        $script:job.State = "Failed"
        Mock Test-ContainerdReady {
            throw "The readiness probe should not run"
        }

        {
            Invoke-WithContainerd -ScriptBlock { "images" }
        } | Should -Throw "*containerd exited before becoming ready*Failed*containerd diagnostic output*"

        Should -Invoke Test-ContainerdReady -Times 0
        Should -Invoke Start-Sleep -Times 0
        Should -Invoke Remove-ContainerdJob -Times 1
    }

    It "fails with the last probe and job diagnostics after the timeout" {
        Mock Test-ContainerdReady {
            return [pscustomobject]@{
                Ready = $false
                Output = "pipe unavailable"
            }
        }

        {
            Invoke-WithContainerd -MaxAttempts 2 -DelaySeconds 0 -ScriptBlock { "images" }
        } | Should -Throw "*did not become ready after 2 attempts*pipe unavailable*containerd diagnostic output*"

        Should -Invoke Test-ContainerdReady -Times 2
        Should -Invoke Start-Sleep -Times 1
        Should -Invoke Remove-ContainerdJob -Times 1
    }

    It "cleans up containerd when the command fails" {
        Mock Test-ContainerdReady {
            return [pscustomobject]@{
                Ready = $true
                Output = ""
            }
        }

        {
            Invoke-WithContainerd -ScriptBlock { throw "command failed" }
        } | Should -Throw "*command failed*"

        Should -Invoke Remove-ContainerdJob -Times 1
    }
}

Describe "Remove-ContainerdJob" {
    BeforeEach {
        Mock Stop-Job {}
        Mock Remove-Job {}
    }

    It "stops a running job before removing it" {
        $job = Start-Job -ScriptBlock { Start-Sleep -Seconds 30 }
        try
        {
            Remove-ContainerdJob -Job $job

            Should -Invoke Stop-Job -Times 1
            Should -Invoke Remove-Job -Times 1
        }
        finally
        {
            Microsoft.PowerShell.Core\Stop-Job -Job $job -ErrorAction SilentlyContinue
            Microsoft.PowerShell.Core\Remove-Job -Job $job -Force -ErrorAction SilentlyContinue
        }
    }

    It "removes an exited job without stopping it again" {
        $job = Start-Job -ScriptBlock { throw "expected test failure" }
        Microsoft.PowerShell.Core\Wait-Job -Job $job | Out-Null
        try
        {
            Remove-ContainerdJob -Job $job

            Should -Invoke Stop-Job -Times 0
            Should -Invoke Remove-Job -Times 1
        }
        finally
        {
            Microsoft.PowerShell.Core\Remove-Job -Job $job -Force -ErrorAction SilentlyContinue
        }
    }
}
