# Script to build the Windows CSE scripts package
# Builds a zip file containing all necessary Windows CSE scripts, excluding test files
$HelpersFile = "vhdbuilder/packer/windows/components_json_helpers.ps1"
. "$HelpersFile"

$componentsJsonFile = "parts/common/components.json"
$componentsJson = Get-Content $componentsJsonFile | Out-String | ConvertFrom-Json

$packages = GetPackagesFromComponentsJson $componentsJson
$csePackages = $packages["c:\akse-cache\"]


$csePackageVersion = $csePackages[0]

$sourceDir = "staging/cse/windows"
$outputDir = "vhdbuilder/packer/windows"
# Define full output file path

$outputFileName = "aks-windows-cse-scripts-$csePackageVersion.zip"
$outputFilePath = Join-Path -Path $outputDir -ChildPath $outputFileName
Write-Host "Building Windows CSE scripts package: $outputFileName"

# Define patterns to exclude
$excludePatterns = @(
    "*.tests.ps1",                  # Test files
    "*.tests.suites/",              # Test folders
    "debug/update-scripts.ps1",     # debug scripts folder
    "README",              # Documentation
    "provisioningscripts/*.md"
)

# Create a temporary directory to organize files for zipping
$tempDir = Join-Path -Path ([System.IO.Path]::GetTempPath()) -ChildPath ([System.Guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $tempDir -Force | Out-Null

# Copy the entire directory first
Copy-Item -Path "$SourceDir/*" -Destination $tempDir -Recurse -Force

# Remove files and directories that match exclude patterns
foreach ($pattern in $excludePatterns) {
    # Handle directory patterns (ending with /)
    if ($pattern.EndsWith("/") -or $pattern.EndsWith("\")) {
        $dirPattern = $pattern.TrimEnd('/', '\')
        $dirsToRemove = Get-ChildItem -Path $tempDir -Directory -Recurse | Where-Object { $_.Name -like $dirPattern }
        foreach ($dir in $dirsToRemove) {
            Write-Host "Removing directory: $($dir.FullName)"
            Remove-Item -Path $dir.FullName -Recurse -Force
        }
    }
    # Handle file patterns
    else {
        $filesToRemove = Get-ChildItem -Path $tempDir -File -Recurse | Where-Object { $_.Name -like $pattern }
        foreach ($file in $filesToRemove) {
            Write-Host "Removing file: $($file.FullName)"
            Remove-Item -Path $file.FullName -Force
        }
    }
}

# Check if an existing output file exists and remove it
if (Test-Path -Path $outputFilePath) {
    Remove-Item -Path $outputFilePath -Force
}

# Create the zip file
Add-Type -AssemblyName System.IO.Compression.FileSystem
[System.IO.Compression.ZipFile]::CreateFromDirectory($tempDir, $outputFilePath)

# Clean up the temporary directory
Remove-Item -Path $tempDir -Recurse -Force

Write-Host "Package created successfully at: $outputFilePath"
