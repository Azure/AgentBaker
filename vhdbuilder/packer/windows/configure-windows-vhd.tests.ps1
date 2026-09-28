BeforeAll {
    # configure-windows-vhd.ps1 is the real VHD-provisioning script, not a pure function library:
    # it has a top-level ". c:/k/windows-vhd-configuration.ps1" (only present on a real build VM)
    # and, at the very end, an unconditional switch on $env:ProvisioningPhase that throws in its
    # default branch. Both are executed top-to-bottom only *after* every function in the file has
    # already been defined, so stripping the dot-source line and swallowing the expected trailing
    # throw is sufficient to safely load every function for testing without needing a real VHD
    # build context.
    #
    # The script also sets $ErrorActionPreference = "Stop" at its top level with no scoping, which
    # otherwise leaks into the global session (confirmed: $ErrorActionPreference is "Continue"
    # before this BeforeAll and "Stop" after) and breaks other, unrelated *.tests.ps1 files that
    # run later in the same Pester/PowerShell process (e.g. in CI, where all of
    # vhdbuilder/packer/windows/ plus windows-files-check.tests.ps1 run together) - save and
    # restore it explicitly so this file's loading never has that side effect.
    $originalErrorActionPreference = $ErrorActionPreference
    $content = Get-Content "$PSScriptRoot\configure-windows-vhd.ps1" -Raw
    $content = $content -replace [regex]::Escape(". c:/k/windows-vhd-configuration.ps1"), ""
    try
    {
        Invoke-Expression $content
    }
    catch
    {
        # Expected: the trailing switch on $env:ProvisioningPhase (default branch) throws, and its
        # own `finally` block calls Get-SystemDriveDiskInfo/Get-DefenderPreferenceInfo, which use
        # Get-CimInstance and aren't available cross-platform (e.g. on Linux pwsh) - that failure
        # can override the original exception. Either way, every function above that point in the
        # file has already been defined by the time we get here, which is all this test file needs.
    }
    finally
    {
        $ErrorActionPreference = $originalErrorActionPreference
    }
}

Describe 'Download-FileWithAzCopy' {
    BeforeEach {
        $script:tempDir = Join-Path ([System.IO.Path]::GetTempPath()) "pester-configure-windows-vhd-$(New-Guid)"
        New-Item -ItemType Directory -Path $script:tempDir -Force | Out-Null

        $global:aksTempDir = (Join-Path $script:tempDir "aksTemp")
        New-Item -ItemType Directory -Path $global:aksTempDir -Force | Out-Null
        New-Item -ItemType File -Path "$global:aksTempDir\azcopy.exe" -Force | Out-Null

        Mock Write-Log {}
        Mock dir {}
        Mock Get-Content {} -ParameterFilter { $Path -like "*azcopy*.log*" }
    }

    AfterEach {
        Remove-Item -Path $script:tempDir -Recurse -Force -ErrorAction SilentlyContinue
    }

    Context 'without -RequireMSILogin (legacy callers: Get-PrivatePackagesToCacheOnVHD, Get-ContainerImages base image override)' {
        It 'falls through to copy when login fails, and succeeds if copy succeeds (e.g. a SAS-bearing URL)' {
            Mock Invoke-AzCopyLogin { $global:LASTEXITCODE = 1 }
            Mock Invoke-AzCopyCopy { $global:LASTEXITCODE = 0 }

            { Download-FileWithAzCopy -URL "https://example.com/f.zip?sv=sas" -Dest (Join-Path $script:tempDir "out.zip") } | Should -Not -Throw

            Should -Invoke Invoke-AzCopyLogin -Times 1
            Should -Invoke Invoke-AzCopyCopy -Times 1
        }

        It 'still throws if copy itself fails, even though login failure alone was tolerated' {
            Mock Invoke-AzCopyLogin { $global:LASTEXITCODE = 1 }
            Mock Invoke-AzCopyCopy { $global:LASTEXITCODE = 1 }

            { Download-FileWithAzCopy -URL "https://example.com/f.zip?sv=sas" -Dest (Join-Path $script:tempDir "out.zip") } | Should -Throw "*azcopy copy*failed*"
        }

        It 'succeeds normally when both login and copy succeed' {
            Mock Invoke-AzCopyLogin { $global:LASTEXITCODE = 0 }
            Mock Invoke-AzCopyCopy { $global:LASTEXITCODE = 0 }

            { Download-FileWithAzCopy -URL "https://example.com/f.zip" -Dest (Join-Path $script:tempDir "out.zip") } | Should -Not -Throw
        }
    }

    Context 'with -RequireMSILogin (the new windowsDownloadRequiresAzCopy component path, via Invoke-PackageDownload)' {
        It 'throws immediately when login fails, before copy is ever attempted' {
            Mock Invoke-AzCopyLogin { $global:LASTEXITCODE = 1 }
            Mock Invoke-AzCopyCopy { $global:LASTEXITCODE = 0 }

            { Download-FileWithAzCopy -URL "https://privatestorageaccount.blob.core.windows.net/c/f.zip" -Dest (Join-Path $script:tempDir "out.zip") -RequireMSILogin } | Should -Throw "*MSI-only*"

            Should -Invoke Invoke-AzCopyCopy -Times 0
        }

        It 'succeeds when login succeeds and copy succeeds' {
            Mock Invoke-AzCopyLogin { $global:LASTEXITCODE = 0 }
            Mock Invoke-AzCopyCopy { $global:LASTEXITCODE = 0 }

            { Download-FileWithAzCopy -URL "https://privatestorageaccount.blob.core.windows.net/c/f.zip" -Dest (Join-Path $script:tempDir "out.zip") -RequireMSILogin } | Should -Not -Throw
        }

        It 'throws if copy fails even when login succeeded' {
            Mock Invoke-AzCopyLogin { $global:LASTEXITCODE = 0 }
            Mock Invoke-AzCopyCopy { $global:LASTEXITCODE = 1 }

            { Download-FileWithAzCopy -URL "https://privatestorageaccount.blob.core.windows.net/c/f.zip" -Dest (Join-Path $script:tempDir "out.zip") -RequireMSILogin } | Should -Throw "*azcopy copy*failed*"
        }
    }
}

Describe 'Invoke-PackageDownload' {
    BeforeEach {
        $script:tempDir = Join-Path ([System.IO.Path]::GetTempPath()) "pester-configure-windows-vhd-$(New-Guid)"
        Mock Write-Log {}
        Mock Download-File {}
        Mock Download-FileWithAzCopy {}
    }

    It 'dispatches to Download-FileWithAzCopy with -RequireMSILogin when the URL is in the AzCopy set' {
        $global:azCopyUrls = @{ "https://privatestorageaccount.blob.core.windows.net/c/f.zip" = $true }

        Invoke-PackageDownload -URL "https://privatestorageaccount.blob.core.windows.net/c/f.zip" -Dest (Join-Path $script:tempDir "out.zip")

        Should -Invoke Download-FileWithAzCopy -Times 1 -ParameterFilter { $RequireMSILogin -eq $true }
        Should -Invoke Download-File -Times 0
    }

    It 'dispatches to Download-File when the URL is not in the AzCopy set' {
        $global:azCopyUrls = @{ "https://privatestorageaccount.blob.core.windows.net/c/f.zip" = $true }

        Invoke-PackageDownload -URL "https://acs-mirror.azureedge.net/f.zip" -Dest (Join-Path $script:tempDir "out.zip")

        Should -Invoke Download-File -Times 1
        Should -Invoke Download-FileWithAzCopy -Times 0
    }

    It 'dispatches to Download-File when the AzCopy set is empty/unset' {
        $global:azCopyUrls = @{ }

        Invoke-PackageDownload -URL "https://acs-mirror.azureedge.net/f.zip" -Dest (Join-Path $script:tempDir "out.zip")

        Should -Invoke Download-File -Times 1
        Should -Invoke Download-FileWithAzCopy -Times 0
    }
}
