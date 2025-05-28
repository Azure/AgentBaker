
BeforeAll {


    . $PSScriptRoot\archive_cse_package.ps1
}

Describe "archive_cse_package.ps1" {   
    BeforeEach {
        $tempDir = Join-Path -Path $env:TEMP -ChildPath "test-cse-$(Get-Random)"
        New-Item -ItemType Directory -Path $tempDir -Force | Out-Null    
        $mockWindowsCSEDir = Join-Path -Path $tempDir -ChildPath "windows"
        New-Item -ItemType Directory -Path $mockWindowsCSEDir -Force | Out-Null
        
        # Create some test files
        "# Main script" | Out-File -FilePath "$mockWindowsCSEDir/main.ps1"
        "# Tests to remove" | Out-File -FilePath "$mockWindowsCSEDir/main.tests.ps1"
        New-Item -ItemType Directory -Path "$mockWindowsCSEDir/debug" -Force | Out-Null
        "# Update script to remove" | Out-File -FilePath "$mockWindowsCSEDir/debug/update-scripts.ps1"
        "# Debug script to keep" | Out-File -FilePath "$mockWindowsCSEDir/debug/debug.ps1"
        $componentsConfigDir =Join-Path -Path $PSScriptRoot -ChildPath "..\..\..\parts\common" 
    }

    AfterEach {
        if (Test-Path -Path $tempDir) {
            Remove-Item -Path $tempDir -Recurse -Force
        }
    }
    It "Should create a zip file with the expected name" {
        $result = Build-WindowsCSEPackage -ScriptDir $PSScriptRoot -ComponentConfigDir $componentsConfigDir -CSEScriptDir $tempDir -OutputDir $tempDir -Inplace $false
        
        # Expected file path based on the mocked filename
        $outputFilePath = $result.CSEOutputFilePath
        Test-Path $outputFilePath | Should -Be $true

        $outputFilePath | Should -BeLike "*aks-windows-cse-scripts-*.zip"
        Test-Path $mockWindowsCSEDir | Should -Be $true
        
        # Extract zip file to temp directory for inspection
        $extractPath = Join-Path -Path $tempDir -ChildPath "extracted"
        New-Item -ItemType Directory -Path $extractPath -Force | Out-Null
        
        # Use .NET API to extract the zip
        Add-Type -AssemblyName System.IO.Compression.FileSystem
        [System.IO.Compression.ZipFile]::ExtractToDirectory($outputFilePath, $extractPath)
        
        # Check file counts - using flexible tests since exact counts depend on the test setup
        $powerShellFiles = Get-ChildItem -Path $extractPath -Recurse -Filter "*.ps1"
        $powerShellFiles.Count | Should -Be 2
        
        # Clean up extraction directory
        Remove-Item -Path $extractPath -Recurse -Force
    }

    It "Should handle inplace mode correctly" {
        $inplaceWindowsDir = Join-Path -Path $tempDir -ChildPath "windows"
        New-Item -ItemType Directory -Path $inplaceWindowsDir -Force | Out-Null
        
        $result = Build-WindowsCSEPackage -ScriptDir $PSScriptRoot -ComponentConfigDir $componentsConfigDir -CSEScriptDir $tempDir -OutputDir $tempDir -Inplace $true
        
        Test-Path $result.CSEOutputFilePath | Should -Be $true
        Test-Path $inplaceWindowsDir | Should -Be $false
    }
}
