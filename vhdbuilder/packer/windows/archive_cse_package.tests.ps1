Describe "archive_cse_package.ps1" {
    BeforeAll {
        # Path to the script we want to test
        $scriptPath = Join-Path -Path $PSScriptRoot -ChildPath "..\archive_cse_package.ps1"
        $testTag = "vtest123"
        $tempDir = Join-Path -Path $env:TEMP -ChildPath "test-cse-$(Get-Random)"
        $sourceDir = "$(Split-Path -Parent $PSScriptRoot)\..\..\..\staging\cse\windows"
        
        # Create temp directory
        New-Item -ItemType Directory -Path $tempDir -Force | Out-Null
    }

    AfterAll {
        # Cleanup
        if (Test-Path -Path $tempDir) {
            Remove-Item -Path $tempDir -Recurse -Force
        }
    }

    It "Script file should exist" {
        Test-Path $scriptPath | Should -Be $true
    }

    It "Should create a zip file with the expected name" {
        $result = & $scriptPath -ImageTag $testTag -OutputDir $tempDir -SourceDir $sourceDir
        
        $expectedZipPath = Join-Path -Path $tempDir -ChildPath "aks-windows-cse-scripts-$testTag.zip"
        Test-Path $expectedZipPath | Should -Be $true
        
        $result.FilePath | Should -Be $expectedZipPath
    }

    It "Should exclude test files and test suites" {
        $zipPath = Join-Path -Path $tempDir -ChildPath "aks-windows-cse-scripts-$testTag.zip"
        
        # Extract zip file to temp directory for inspection
        $extractPath = Join-Path -Path $tempDir -ChildPath "extracted"
        New-Item -ItemType Directory -Path $extractPath -Force | Out-Null
        
        # Use .NET API to extract the zip
        Add-Type -AssemblyName System.IO.Compression.FileSystem
        [System.IO.Compression.ZipFile]::ExtractToDirectory($zipPath, $extractPath)
        
        # Check that test files are excluded
        $testFiles = Get-ChildItem -Path $extractPath -Recurse -Filter "*.tests.ps1"
        $testFiles.Count | Should -Be 0
        
        # Check that test suite directories are excluded
        $testDirs = Get-ChildItem -Path $extractPath -Recurse -Directory | Where-Object { $_.Name -like "*.tests.suites" }
        $testDirs.Count | Should -Be 0
        
        # But make sure we still have core functionality files
        $tomlFiles = Get-ChildItem -Path $extractPath -Recurse -Filter "*.toml"
        $tomlFiles.Count | Should -Be 2

        $powerShellFiles = Get-ChildItem -Path  $extractPath -Recurse -Filter "*.ps1"
        $powerShellFiles.Count | Should -Be 16

        $csFiles = Get-ChildItem -Path $extractPath -Recurse -Filter "*.cs"
        $csFiles.Count | Should -Be 1
        
        # Clean up extraction directory
        Remove-Item -Path $extractPath -Recurse -Force
    }
}
