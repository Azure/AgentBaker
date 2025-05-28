<#
    .SYNOPSIS
        Used to produce Windows AKS CSE pacakges.

    .DESCRIPTION
        This script is used by packer to cahce necessary Windows CSE scripts on Windows AKS images.
#>

param(
    [Parameter(Mandatory=$false)]
    [string]$ScriptDir = "vhdbuilder/packer/windows",
    [Parameter(Mandatory=$false)]
    [string]$PartsDir = "parts/common",
    [Parameter(Mandatory=$false)]
    [string]$CSEScriptDir = "staging/cse", # The source directory containing Windows CSE scripts
    [Parameter(Mandatory=$false)]
    [string]$Inplace = false,
    [Parameter(Mandatory=$false)]
    [string]$OutputDir = ""   
)

$ComponentsJsonFile = Join-Path -Path $PartsDir -ChildPath "components.json"
$HelpersFile =  Join-Path -Path $ScriptDir -ChildPath "components_json_helpers.ps1"
. "$HelpersFile"

# Get version from components.json if not provided      
$componentsJson = Get-Content $componentsJsonFile | Out-String | ConvertFrom-Json
$packages = GetPackagesFromComponentsJson $componentsJson

$cseDownloadLocation = "c:/akse-cache/"
$csePackages = $packages[$CSEDownloadLocation]
$cseDownloadurl = $csePackages[0]
$csePackageFileName = Split-Path -Path $cseDownloadurl -Leaf

function Build-WindowsCSEPackage {
    param(
        [Parameter(Mandatory=$true)]
        [string]$CSEScriptDir,
        [Parameter(Mandatory=$true)]
        [bool]$Inplace,
        [Parameter(Mandatory=$false)]
        [string]$OutputDir=""
    )

    $cseScriptPath = Join-Path -Path $CSEScriptDir -ChildPath "windows"
    $workingDir = $cseScriptPath

    if (!$Inplace) {
        $workingDir = Join-Path -Path ([System.IO.Path]::GetTempPath()) -ChildPath ([System.Guid]::NewGuid().ToString())
        New-Item -ItemType Directory -Path $workingDir -Force | Out-Null
        Copy-Item -Path "$cseScriptPath/*" -Destination $workingDir -Recurse -Force
    }
    
    Get-ChildItem -Path $workingDir -Filter "*.tests.ps1" -Recurse | ForEach-Object {
        Remove-Item -Path $_.FullName -Force
    }
    
    Get-ChildItem -Path $workingDir -Directory -Filter "*.tests.suites" -Recurse | ForEach-Object {
        Remove-Item -Path $_.FullName -Recurse -Force
    }
    
    Get-ChildItem -Path "$workingDir\debug" -Filter "update-scripts.ps1" | ForEach-Object {
        Remove-Item -Path $_.FullName -Force
    }

    Get-ChildItem -Path $workingDir -Filter "README*" -Recurse | ForEach-Object {
        Remove-Item -Path $_.FullName -Force
    }

    Get-ChildItem -Path $workingDir -Filter "*.md" -Recurse | ForEach-Object {
        Remove-Item -Path $_.FullName -Force
    }

    if ([string]::IsNullOrEmpty($OutputDir)) {
        if ($Inplace) {
            # when not specfied, put zip at the same level as the cse windows/ folder
            $OutputDir = $CSEScriptDir
        }
        else {
            throw "Missing output directory parameter. Please specify an output directory -OutputDir."
        }
    }
    
    $outputFilePath = Join-Path -Path $OutputDir -ChildPath $csePackageFileName
    Write-Log  "Building Windows CSE scripts package: $outputFilePath"

    if (Test-Path -Path $outputFilePath) {
        Remove-Item -Path $outputFilePath -Force
    }

    # Create the zip file
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [System.IO.Compression.ZipFile]::CreateFromDirectory($workingDir, $outputFilePath)

    Remove-Item -Path $workingDir -Recurse -Force
    Write-Log "Package created successfully at: $outputFilePath"
    
    # Return an object with file information
    return @{
        FilePath = $outputFilePath
    }
}

# Execute the function when the script is run directly
$result = Build-WindowsCSEPackage -CSESourceDir $CSEScriptDir -Inplace $Inplace -OutputDir $OutputDir
return $result
