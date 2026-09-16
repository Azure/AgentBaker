BeforeAll {
    # windows-files-check.ps1 is a real test-runner script (dot-sources windows-vhd-configuration.ps1
    # and unconditionally calls Test-CompareFiles/Test-ValidateAllSignature at the bottom), not a
    # pure function library, so we extract just the function under test via regex rather than
    # sourcing the whole file - this avoids executing any of that top-level work.
    $content = Get-Content "$PSScriptRoot\windows-files-check.ps1" -Raw
    $match = [regex]::Match($content, "(?ms)^function Test-ValidateSinglePackageSignature \{.*?\r?\n\}\r?\n")
    if (-not $match.Success)
    {
        throw "Could not extract Test-ValidateSinglePackageSignature from windows-files-check.ps1 - has its signature changed?"
    }
    Invoke-Expression $match.Value
}

Describe 'Test-ValidateSinglePackageSignature' {
    BeforeEach {
        $global:azCopyUrls = @{ "https://privatestorageaccount.blob.core.windows.net/c/private-package.zip" = $true }
    }

    It 'skips signature validation for an AzCopy-flagged URL instead of failing on a missing archive' {
        # This runner (validate-windows-binary-signature.yaml, windows-latest) has no managed
        # identity, so Test-CompareSingleDir never downloads AzCopy-flagged URLs and the archive
        # genuinely does not exist at $dest here - the regression this test guards against is
        # Test-ValidateSinglePackageSignature previously trying (and failing) to Expand-Archive it
        # anyway.
        $map = @{
            "TestDrive:\doesnotexist\" = @("https://privatestorageaccount.blob.core.windows.net/c/private-package.zip")
        }

        { Test-ValidateSinglePackageSignature -dir "TestDrive:\doesnotexist\" } | Should -Not -Throw
    }

    It 'still attempts signature validation for a non-flagged URL (regression check: the skip is scoped to AzCopy URLs only)' {
        $global:azCopyUrls = @{ }
        $map = @{
            "TestDrive:\doesnotexist\" = @("https://acs-mirror.azureedge.net/public-package.zip")
        }

        # A non-flagged URL should NOT be skipped, so it proceeds to Expand-Archive on a file that
        # (deliberately, in this test) doesn't exist either, and fails there instead of being
        # silently skipped - proving the skip really is scoped to AzCopy-flagged URLs only.
        { Test-ValidateSinglePackageSignature -dir "TestDrive:\doesnotexist\" } | Should -Throw "*Expand-Archive*"
    }
}
