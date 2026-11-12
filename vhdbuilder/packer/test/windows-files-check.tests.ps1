BeforeAll {
    # windows-files-check.ps1 is a real test-runner script (dot-sources windows-vhd-configuration.ps1
    # and unconditionally calls Test-CompareFiles/Test-ValidateAllSignature at the bottom), not a
    # pure function library, so we extract just the functions under test via regex rather than
    # sourcing the whole file - this avoids executing any of that top-level work.
    $content = Get-Content "$PSScriptRoot\windows-files-check.ps1" -Raw

    $match = [regex]::Match($content, "(?ms)^function Test-ValidateSinglePackageSignature \{.*?\r?\n\}\r?\n")
    if (-not $match.Success)
    {
        throw "Could not extract Test-ValidateSinglePackageSignature from windows-files-check.ps1 - has its signature changed?"
    }
    Invoke-Expression $match.Value

    # Also needed so tests can Mock them instead of the real Get-ChildItem/Get-AuthenticodeSignature
    # pipelines Test-ValidateSinglePackageSignature calls into.
    foreach ($helperName in @("Get-UnsignedBinariesInDirectory", "Get-UnsignedFilesExcludingKnownTypes")) {
        $helperMatch = [regex]::Match($content, "(?ms)^function $helperName \{.*?\r?\n\}\r?\n")
        if (-not $helperMatch.Success)
        {
            throw "Could not extract $helperName from windows-files-check.ps1 - has its signature changed?"
        }
        Invoke-Expression $helperMatch.Value
    }
}

Describe 'Test-ValidateSinglePackageSignature' {
    BeforeEach {
        $global:azCopyUrls = @{ "https://privatestorageaccount.blob.core.windows.net/c/private-package.zip" = $true }
        $script:nonExistentDir = Join-Path ([System.IO.Path]::GetTempPath()) "pester-wfc-doesnotexist-$(New-Guid)"
        # Normally set by windows-vhd-configuration.ps1 (not dot-sourced for these tests) and defined
        # at windows-files-check.ps1's top level (not extracted above, since it's outside the function).
        $global:SkipSignatureCheckForBinaries = @{}
        $SkipMapForSignature = @{}
        $NotSignedResult = @{}
    }

    It 'skips signature validation for an AzCopy-flagged URL instead of failing on a missing archive' {
        # This runner (validate-windows-binary-signature.yaml, windows-latest) has no managed
        # identity, so Test-CompareSingleDir never downloads AzCopy-flagged URLs and the archive
        # genuinely does not exist at $dest here - the regression this test guards against is
        # Test-ValidateSinglePackageSignature previously trying (and failing) to Expand-Archive it
        # anyway.
        $map = @{
            $script:nonExistentDir = @("https://privatestorageaccount.blob.core.windows.net/c/private-package.zip")
        }

        { Test-ValidateSinglePackageSignature -dir $script:nonExistentDir } | Should -Not -Throw
    }

    It 'still attempts signature validation for a non-flagged URL (regression check: the skip is scoped to AzCopy URLs only)' {
        $global:azCopyUrls = @{ }
        $map = @{
            $script:nonExistentDir = @("https://acs-mirror.azureedge.net/public-package.zip")
        }

        # A non-flagged URL should NOT be skipped, so it proceeds to Expand-Archive on a file that
        # (deliberately, in this test) doesn't exist either, and fails there instead of being
        # silently skipped - proving the skip really is scoped to AzCopy-flagged URLs only. Match
        # broadly on "archive" rather than the specific throw text ("Expand-Archive failed for..."):
        # if $ErrorActionPreference is "Stop" in the ambient session, the preceding Write-Error call
        # becomes terminating first, surfacing its own message ("Failed to expand archive...")
        # instead - both indicate the same "didn't skip, attempted extraction" outcome this test
        # wants to verify.
        { Test-ValidateSinglePackageSignature -dir $script:nonExistentDir } | Should -Throw "*archive*"
    }

    It 'skips an unsigned binary that matches both the allowlisted directory and filename with NotSigned status' {
        $global:azCopyUrls = @{ }
        $map = @{
            $script:nonExistentDir = @("https://acs-mirror.azureedge.net/win-k8s-package.zip")
        }
        $global:SkipSignatureCheckForBinaries = @{ $script:nonExistentDir = @("win-bridge.exe") }

        Mock Test-Path {}
        Mock New-Item {}
        Mock Expand-Archive {}
        Mock Remove-Item {}
        Mock Get-UnsignedFilesExcludingKnownTypes { @() } # the second (all-file-types) check; not under test here
        Mock Get-UnsignedBinariesInDirectory { @( [PSCustomObject]@{ Path = "win-bridge.exe"; Status = "NotSigned" } ) }

        Test-ValidateSinglePackageSignature -dir $script:nonExistentDir

        $NotSignedResult.Count | Should -Be 0
    }

    It 'still fails an unsigned allowlisted filename found in a different (non-matching) directory' {
        $global:azCopyUrls = @{ }
        $map = @{
            $script:nonExistentDir = @("https://acs-mirror.azureedge.net/win-k8s-package.zip")
        }
        # Allowlist entry exists, but for a different directory than the one being validated.
        $global:SkipSignatureCheckForBinaries = @{ "c:\akse-cache\win-k8s\" = @("win-bridge.exe") }

        Mock Test-Path {}
        Mock New-Item {}
        Mock Expand-Archive {}
        Mock Remove-Item {}
        Mock Get-UnsignedFilesExcludingKnownTypes { @() }
        Mock Get-UnsignedBinariesInDirectory { @( [PSCustomObject]@{ Path = "win-bridge.exe"; Status = "NotSigned" } ) }

        Test-ValidateSinglePackageSignature -dir $script:nonExistentDir

        $NotSignedResult.ContainsKey($script:nonExistentDir) | Should -Be $true
    }

    It 'still fails an allowlisted filename in the matching directory when its status is not NotSigned (e.g. HashMismatch/tampered)' {
        $global:azCopyUrls = @{ }
        $map = @{
            $script:nonExistentDir = @("https://acs-mirror.azureedge.net/win-k8s-package.zip")
        }
        $global:SkipSignatureCheckForBinaries = @{ $script:nonExistentDir = @("win-bridge.exe") }

        Mock Test-Path {}
        Mock New-Item {}
        Mock Expand-Archive {}
        Mock Remove-Item {}
        Mock Get-UnsignedFilesExcludingKnownTypes { @() }
        Mock Get-UnsignedBinariesInDirectory { @( [PSCustomObject]@{ Path = "win-bridge.exe"; Status = "HashMismatch" } ) }

        Test-ValidateSinglePackageSignature -dir $script:nonExistentDir

        $NotSignedResult.ContainsKey($script:nonExistentDir) | Should -Be $true
    }
}

