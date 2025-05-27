Describe "archive_cse_package.ps1" {
    BeforeEach {
        # Path to the script we want to test
        $scriptPath = Join-Path -Path $PSScriptRoot -ChildPath "archive_cse_package.ps1"

        $tempDir = Join-Path -Path $env:TEMP -ChildPath "test-cse-$(Get-Random)"
        # Create temp directory
        New-Item -ItemType Directory -Path $tempDir -Force | Out-Null
    }

    AfterEach {
        # Cleanup
        if (Test-Path -Path $tempDir) {
            Remove-Item -Path $tempDir -Recurse -Force
        }
    }

    It "Script file should exist" {
        Test-Path $scriptPath | Should -Be $true
    }

    It "Should create a zip file with the expected name" {
        # Use dot-sourcing to make the function available
        . $scriptPath
        
        # Call the function directly with the test parameters
        $fileName = "aks-windows-cse-scripts-vtest123.zip"
        $result = Build-WindowsCSEPackage -FileName $fileName -OutputDir $tempDir 
        
        $expectedZipPath = Join-Path -Path $tempDir -ChildPath $fileName
        Test-Path $expectedZipPath | Should -Be $true
        
        # Test the returned object properties
        $result.FilePath | Should -Be $expectedZipPath
        
        # Extract zip file to temp directory for inspection
        $extractPath = Join-Path -Path $tempDir -ChildPath "extracted"
        New-Item -ItemType Directory -Path $extractPath -Force | Out-Null
        
        # Use .NET API to extract the zip
        Add-Type -AssemblyName System.IO.Compression.FileSystem
        [System.IO.Compression.ZipFile]::ExtractToDirectory($expectedZipPath, $extractPath)
        
        $powerShellFiles = Get-ChildItem -Path $extractPath -Recurse -Filter "*.ps1"
        $powerShellFiles.Count | Should -Be 24

        $tomlFiles = Get-ChildItem -Path $extractPath -Recurse -Filter "*.toml"
        $tomlFiles.Count | Should -Be 2  # More flexible test for file counts

        $csFiles = Get-ChildItem -Path $extractPath -Recurse -Filter "*.cs"
        $csFiles.Count | Should -Be 1  # We expect multiple PS1 files but exact count may change

        # Clean up extraction directory
        Remove-Item -Path $extractPath -Recurse -Force

    }

    It "Should handle default parameters correctly" {
        # Test with minimal parameters
        . $scriptPath
        
        # Mock the Test-Path function to return true for the helper files
        Mock Test-Path { return $true } -ParameterFilter { $Path -like "*components_json_helpers.ps1" -or $Path -like "*components.json" }
        
        # Mock Get-Content to return a simplified components.json
        Mock Get-Content { 
            return '{
"Packages": [
{
  "windowsDownloadLocation": "c:\\akse-cache\\",
  "downloadLocation": null,
  "downloadUris": {
    "windows": {
      "default": {
        "versionsV2": [
          {
            "renovateTag": "<DO_NOT_UPDATE>",
            "latestVersion": "1.2.3",
            "previousLatestVersion": "0.0.51"
          }
        ],
        "downloadURL": "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v${version}.zip"
      }
    }
  }
}
]}'
        } -ParameterFilter { $Path -like "*components.json" }
        
        # Just specify the output directory for the test
        $result = Build-WindowsCSEPackage -OutputDir $tempDir
        
        $result.FilePath | Should -Not -BeNullOrEmpty
        Test-Path $result.FilePath | Should -Be $true
        $expectedZipPath = Join-Path -Path $tempDir -ChildPath "aks-windows-cse-scripts-v1.2.3.zip"
        $result.FilePath | Should -Be $expectedZipPath
    }
}
