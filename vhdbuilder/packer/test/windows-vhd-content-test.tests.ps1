BeforeAll {
    # windows-vhd-content-test.ps1 is a real test-runner script (dot-sources
    # windows-vhd-configuration.ps1, then unconditionally runs every Test-* function starting at
    # its "Starting Tests" marker), not a pure function library, so we strip both the dot-source
    # and that entire trailing invocation block before loading it - every function we need,
    # including Test-PrivatePackageSignature, is defined above that marker. We can't just wrap
    # Invoke-Expression in try/catch and let the invocation block run: several of those Test-*
    # functions call `exit` directly on a real failure (e.g. Test-PatchInstalled on a plain CI
    # runner that isn't an actual built VHD), and `exit` isn't a catchable exception - it would
    # kill the whole Pester process before Test-PrivatePackageSignature is even defined.
    $content = Get-Content "$PSScriptRoot\windows-vhd-content-test.ps1" -Raw
    $content = $content -replace [regex]::Escape(". c:\k\windows-vhd-configuration.ps1"), ""
    $content = $content -replace '(?s)Write-OutputWithTimestamp "Starting Tests".*', ""
    Invoke-Expression $content
}

Describe 'Test-PrivatePackageSignature' {
    BeforeEach {
        Mock Write-ErrorWithTimestamp {}
        Mock Write-OutputWithTimestamp {}
        $script:realTempDir = $null
        # Normally set by windows-vhd-configuration.ps1 (dot-source stripped above for tests).
        $global:SkipSignatureCheckForBinaries = @{ "win-bridge.exe" = $True }
    }

    AfterEach {
        if ($script:realTempDir)
        {
            # Raw .NET call, not the Remove-Item cmdlet (which may be mocked above), for cleanup.
            try { [System.IO.Directory]::Delete($script:realTempDir, $true) } catch { }
        }
    }

    It 'returns immediately without attempting any validation when there are no AzCopy-flagged packages' {
        Mock Expand-Archive {}
        $global:azCopyUrls = @{ }
        $map = @{ "c:\akse-cache\" = @("https://acs-mirror.azureedge.net/public-package.zip") }

        { Test-PrivatePackageSignature } | Should -Not -Throw

        Should -Invoke Expand-Archive -Times 0
    }

    It 'only attempts validation for URLs present in the AzCopy set, skipping public ones in the same directory' {
        # Real files first, before mocking Test-Path/New-Item/Remove-Item below - once mocked, only
        # calls those mocks explicitly handle are allowed through, so our own setup must run first.
        $script:realTempDir = Join-Path ([System.IO.Path]::GetTempPath()) "pester-vhd-content-test-$(New-Guid)"
        New-Item -ItemType Directory -Path $script:realTempDir -Force | Out-Null
        Set-Content -Path (Join-Path $script:realTempDir "private-package.zip") -Value "placeholder"
        Set-Content -Path (Join-Path $script:realTempDir "public-package.zip") -Value "placeholder"

        $map = @{
            $script:realTempDir = @(
                "https://privatestorageaccount.blob.core.windows.net/c/private-package.zip",
                "https://acs-mirror.azureedge.net/public-package.zip"
            )
        }
        $global:azCopyUrls = @{ "https://privatestorageaccount.blob.core.windows.net/c/private-package.zip" = $true }

        # $installDir ("c:\PrivatePackageSignatureCheck") is a hardcoded Windows path - mock every
        # operation against it so this doesn't depend on a real c: drive existing.
        Mock Test-Path {}
        Mock Remove-Item {}
        Mock New-Item {}
        Mock Expand-Archive {}
        Mock Get-UnsignedBinariesInDirectory { @() }

        Test-PrivatePackageSignature

        Should -Invoke Expand-Archive -Times 1 -ParameterFilter { $Path -like "*private-package.zip" }
        Should -Invoke Expand-Archive -Times 0 -ParameterFilter { $Path -like "*public-package.zip" }
    }

    It 'completes without exiting and without logging any errors when all extracted binaries are validly signed' {
        $script:realTempDir = Join-Path ([System.IO.Path]::GetTempPath()) "pester-vhd-content-test-$(New-Guid)"
        New-Item -ItemType Directory -Path $script:realTempDir -Force | Out-Null
        Set-Content -Path (Join-Path $script:realTempDir "private-package.zip") -Value "placeholder"

        $map = @{
            $script:realTempDir = @("https://privatestorageaccount.blob.core.windows.net/c/private-package.zip")
        }
        $global:azCopyUrls = @{ "https://privatestorageaccount.blob.core.windows.net/c/private-package.zip" = $true }

        Mock Test-Path {}
        Mock Remove-Item {}
        Mock New-Item {}
        Mock Expand-Archive {}
        Mock Get-UnsignedBinariesInDirectory { @() } # empty = nothing unsigned = no invalid files

        Test-PrivatePackageSignature

        Should -Invoke Expand-Archive -Times 1
        Should -Invoke Write-ErrorWithTimestamp -Times 0
    }

    It 'does not report the package as invalid when the only unsigned binary is an allowlisted one (e.g. win-bridge.exe)' {
        $script:realTempDir = Join-Path ([System.IO.Path]::GetTempPath()) "pester-vhd-content-test-$(New-Guid)"
        New-Item -ItemType Directory -Path $script:realTempDir -Force | Out-Null
        Set-Content -Path (Join-Path $script:realTempDir "private-package.zip") -Value "placeholder"

        $map = @{
            $script:realTempDir = @("https://privatestorageaccount.blob.core.windows.net/c/private-package.zip")
        }
        $global:azCopyUrls = @{ "https://privatestorageaccount.blob.core.windows.net/c/private-package.zip" = $true }

        Mock Test-Path {}
        Mock Remove-Item {}
        Mock New-Item {}
        Mock Expand-Archive {}
        Mock Get-UnsignedBinariesInDirectory { @( [PSCustomObject]@{ Path = "win-bridge.exe"; Status = "NotSigned" } ) }

        { Test-PrivatePackageSignature } | Should -Not -Throw

        Should -Invoke Write-ErrorWithTimestamp -Times 0
        Should -Invoke Write-OutputWithTimestamp -Times 1 -ParameterFilter { $Message -like "win-bridge.exe*ignore list*" }
    }

    It 'still fails an allowlisted filename when its unsigned status is not NotSigned (e.g. HashMismatch/tampered)' {
        $script:realTempDir = Join-Path ([System.IO.Path]::GetTempPath()) "pester-vhd-content-test-$(New-Guid)"
        New-Item -ItemType Directory -Path $script:realTempDir -Force | Out-Null
        Set-Content -Path (Join-Path $script:realTempDir "private-package.zip") -Value "placeholder"

        $map = @{
            $script:realTempDir = @("https://privatestorageaccount.blob.core.windows.net/c/private-package.zip")
        }
        $global:azCopyUrls = @{ "https://privatestorageaccount.blob.core.windows.net/c/private-package.zip" = $true }

        # Test-PrivatePackageSignature calls `exit 1` once it detects an invalid file - run in a
        # child pwsh process, same as the other exit-triggering test below.
        $childScript = @"
`$content = Get-Content '$PSScriptRoot\windows-vhd-content-test.ps1' -Raw
`$content = `$content -replace [regex]::Escape('. c:\k\windows-vhd-configuration.ps1'), ''
`$content = `$content -replace '(?s)Write-OutputWithTimestamp "Starting Tests".*', ''
Invoke-Expression `$content
function Write-ErrorWithTimestamp(`$m) { Write-Host `$m }
function Write-OutputWithTimestamp(`$m) { Write-Host `$m }
function Test-Path { param(`$Path) `$true }
function New-Item { param(`$ItemType, `$Path, [switch]`$Force) }
function Remove-Item { param(`$Path, [switch]`$Recurse, [switch]`$Force) }
function Expand-Archive { param(`$Path, `$DestinationPath, [switch]`$Force, `$ErrorAction) }
function Get-UnsignedBinariesInDirectory { param(`$Directory, `$IncludeList) @( [PSCustomObject]@{ Path = 'win-bridge.exe'; Status = 'HashMismatch' } ) }
`$global:azCopyUrls = @{ 'https://privatestorageaccount.blob.core.windows.net/c/private-package.zip' = `$true }
`$global:SkipSignatureCheckForBinaries = @{ 'win-bridge.exe' = `$True }
`$map = @{ '$($script:realTempDir)' = @('https://privatestorageaccount.blob.core.windows.net/c/private-package.zip') }
Test-PrivatePackageSignature
"@
        $result = & pwsh -NoProfile -Command $childScript 2>&1
        $LASTEXITCODE | Should -Be 1
        ($result -join "`n") | Should -Match "not signed"
    }

    It 'reports the package as invalid (via Write-ErrorWithTimestamp) when an extracted binary is not signed' {
        $script:realTempDir = Join-Path ([System.IO.Path]::GetTempPath()) "pester-vhd-content-test-$(New-Guid)"
        New-Item -ItemType Directory -Path $script:realTempDir -Force | Out-Null
        Set-Content -Path (Join-Path $script:realTempDir "private-package.zip") -Value "placeholder"

        $map = @{
            $script:realTempDir = @("https://privatestorageaccount.blob.core.windows.net/c/private-package.zip")
        }
        $global:azCopyUrls = @{ "https://privatestorageaccount.blob.core.windows.net/c/private-package.zip" = $true }

        # Test-PrivatePackageSignature calls `exit 1` once it detects an invalid file, which would
        # terminate this whole Pester process rather than just failing an assertion - run the real
        # function in a child pwsh process instead, so only that process exits, and assert on its
        # exit code and output.
        $childScript = @"
`$content = Get-Content '$PSScriptRoot\windows-vhd-content-test.ps1' -Raw
`$content = `$content -replace [regex]::Escape('. c:\k\windows-vhd-configuration.ps1'), ''
`$content = `$content -replace '(?s)Write-OutputWithTimestamp "Starting Tests".*', ''
Invoke-Expression `$content
function Write-ErrorWithTimestamp(`$m) { Write-Host `$m }
function Write-OutputWithTimestamp(`$m) { Write-Host `$m }
function Test-Path { param(`$Path) `$true }
function New-Item { param(`$ItemType, `$Path, [switch]`$Force) }
function Remove-Item { param(`$Path, [switch]`$Recurse, [switch]`$Force) }
function Expand-Archive { param(`$Path, `$DestinationPath, [switch]`$Force, `$ErrorAction) }
function Get-UnsignedBinariesInDirectory { param(`$Directory, `$IncludeList) @( [PSCustomObject]@{ Path = 'tool.exe'; Status = 'NotSigned' } ) }
`$global:azCopyUrls = @{ 'https://privatestorageaccount.blob.core.windows.net/c/private-package.zip' = `$true }
`$global:SkipSignatureCheckForBinaries = @{ 'win-bridge.exe' = `$True }
`$map = @{ '$($script:realTempDir)' = @('https://privatestorageaccount.blob.core.windows.net/c/private-package.zip') }
Test-PrivatePackageSignature
"@
        $result = & pwsh -NoProfile -Command $childScript 2>&1
        $LASTEXITCODE | Should -Be 1
        ($result -join "`n") | Should -Match "not signed"
    }
}
