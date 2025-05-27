# Builds a zip file containing all necessary Windows CSE scripts, excluding test files

param(
    [Parameter(Mandatory=$false)]
    [string]$OutputDir = "vhdbuilder/packer/windows",
    
    [Parameter(Mandatory=$false)]
    [string]$FileName = ""
)

function Build-WindowsCSEPackage {
    param(
        [Parameter(Mandatory=$false)]
        [string]$OutputDir = "vhdbuilder/packer/windows",
        
        [Parameter(Mandatory=$false)]
        [string]$FileName = ""
    )

    if ([string]::IsNullOrEmpty($FileName)) {
        # Get version from components.json if not provided
        $HelpersFile = "vhdbuilder/packer/windows/components_json_helpers.ps1"
        if (Test-Path $HelpersFile) {
            . "$HelpersFile"

            $componentsJsonFile = "parts/common/components.json"
            if (Test-Path $componentsJsonFile) {
                $componentsJson = Get-Content $componentsJsonFile | Out-String | ConvertFrom-Json
                $packages = GetPackagesFromComponentsJson $componentsJson
                $csePackages = $packages["c:\akse-cache\"]
                $url = $csePackages[0]
                # Get the filename from the URL
                $FileName = Split-Path -Path $url -Leaf
            }
            else {
                Write-Error "Components JSON file not found: $componentsJsonFile"
                throw "Unable to determine package version. Components JSON file not found: $componentsJsonFile"
            }
        }
        else {
            Write-Error "Helpers file not found: $HelpersFile"
            throw "Unable to determine package version. Helpers file not found: $HelpersFile"
        }
    }

    # Define full output file path
    $outputFilePath = Join-Path -Path $OutputDir -ChildPath $FileName
    Write-Host "Building Windows CSE scripts package: $outputFilePath"

    $tempDir = Join-Path -Path ([System.IO.Path]::GetTempPath()) -ChildPath ([System.Guid]::NewGuid().ToString())
    New-Item -ItemType Directory -Path $tempDir -Force | Out-Null

    $SourceDir = "staging/cse/windows"
    Copy-Item -Path "$SourceDir/*" -Destination $tempDir -Recurse -Force

    # Remove files and directories that match exclude patterns
    Write-Host "Removing unnecessary files and directories..."

    Get-ChildItem -Path $tempDir -Filter "*.tests.ps1" -Recurse | ForEach-Object {
        Remove-Item -Path $_.FullName -Force
    }
    
    Get-ChildItem -Path $tempDir -Directory -Filter "*.tests.suites" -Recurse | ForEach-Object {
        Remove-Item -Path $_.FullName -Recurse -Force
    }
    
    Get-ChildItem -Path "$tempDir\debug" -Filter "update-scripts.ps1" | ForEach-Object {
        Remove-Item -Path $_.FullName -Force
    }

    Get-ChildItem -Path $tempDir -Filter "README*" -Recurse | ForEach-Object {
        Remove-Item -Path $_.FullName -Force
    }

    Get-ChildItem -Path $tempDir -Filter "*.md" -Recurse | ForEach-Object {
        Remove-Item -Path $_.FullName -Force
    }
    
    # Check if an existing output file exists and remove it
    if (Test-Path -Path $outputFilePath) {
        Remove-Item -Path $outputFilePath -Force
    }

    # Create the zip file
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [System.IO.Compression.ZipFile]::CreateFromDirectory($tempDir, $outputFilePath)
    Remove-Item -Path $tempDir -Recurse -Force

    Write-Host "Package created successfully at: $outputFilePath"
    
    # Return an object with file information
    return @{
        FilePath = $outputFilePath
    }
}

# Execute the function when the script is run directly
$result = Build-WindowsCSEPackage -OutputDir $OutputDir -FileName $FileName
return $result
