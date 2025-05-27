# Script to build the Windows CSE scripts package
# Builds a zip file containing all necessary Windows CSE scripts, excluding test files

param(
    [Parameter(Mandatory=$true)]
    [string]$ImageTag,  # The version tag to use in the package name, e.g., "v0.0.52"
    
    [Parameter(Mandatory=$false)]
    [string]$SourceDir = "staging/cse/windows", # The source directory containing Windows CSE scripts
    
    [Parameter(Mandatory=$false)]
    [string]$OutputDir = "." # The output directory for the zip file
)

$ErrorActionPreference = "Stop"

# Ensure the output directory exists
if (!(Test-Path -Path $OutputDir)) {
    New-Item -ItemType Directory -Path $OutputDir -Force | Out-Null
}

# Define full output file path
$outputFileName = "aks-windows-cse-scripts-$ImageTag.zip"
$outputFilePath = Join-Path -Path $OutputDir -ChildPath $outputFileName

# Define patterns to exclude
$excludePatterns = @(
    "*.tests.ps1",         # Test files
    "*.tests.suites/",     # Test folders
    "debug/",              # debug scripts folder
    "README",              # Documentation
    "*.md"
)

Write-Host "Building Windows CSE scripts package: $outputFileName"
Write-Host "Source directory: $SourceDir"
Write-Host "Output file: $outputFilePath"

# Create a temporary directory to organize files for zipping
$tempDir = Join-Path -Path ([System.IO.Path]::GetTempPath()) -ChildPath ([System.Guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $tempDir -Force | Out-Null

# Copy the entire directory first
Copy-Item -Path "$SourceDir\*" -Destination $tempDir -Recurse -Force

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

# # Also remove all test suite directories
# $testSuiteDirs = Get-ChildItem -Path $tempDir -Directory -Recurse | Where-Object { $_.Name -like "*.tests.suites" }
# foreach ($dir in $testSuiteDirs) {
#     Write-Host "Removing test suite directory: $($dir.FullName)"
#     Remove-Item -Path $dir.FullName -Recurse -Force
# }

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

# Return the file info as an object
return @{
    FilePath = $outputFilePath
}