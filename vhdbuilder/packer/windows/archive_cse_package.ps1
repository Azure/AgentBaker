<#
    .SYNOPSIS
        Used to produce Windows AKS CSE pacakges.

    .DESCRIPTION
        This script is used by packer to cahce necessary Windows CSE scripts on Windows AKS images.
#>
param(
    [Parameter(Mandatory=$false)]
    [string]$ScriptDir = "$env:HelperScriptDir",
    [Parameter(Mandatory=$false)]
    [string]$ComponentConfigDir = "$env:ComponentConfigDir",
    [Parameter(Mandatory=$false)]
    [string]$CSEScriptDir = "$env:CSEScriptDir", # The source directory containing Windows CSE scripts
    [Parameter(Mandatory=$false)]
    [string]$OutputDir = "$env:CSEOutputDir",
    [Parameter(Mandatory=$false)]
    [bool]$Inplace = $false # If true, the package will be created in the same directory to replace the CSE scripts 
)

# Define Write-Log function if it doesn't exist in the calling context
if (-not (Get-Command Write-Log -ErrorAction SilentlyContinue)) {
    function Write-Log {
        param(
            [Parameter(Mandatory=$true)]
            [string]$Message
        )
        
        $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
        Write-Host "[$timestamp] $Message"
    }
}
function Get-WindowsCSEEnvironment {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory=$false)]
        [string]$ScriptDir = "$env:HelperScriptDir",
        [Parameter(Mandatory=$false)]
        [string]$ComponentConfigDir = "$env:ComponentConfigDir",
        [Parameter(Mandatory=$false)]
        [string]$CSEScriptDir = "$env:CSEScriptDir", # The source directory containing Windows CSE scripts
        [Parameter(Mandatory=$false)]
        [string]$OutputDir = "$env:CSEOutputDir",
        [Parameter(Mandatory=$false)]
        [bool]$Inplace = $false # If true, the package will be created in the same directory to replace the CSE scripts 
    )

    $ComponentsJsonFile = Join-Path -Path $ComponentConfigDir -ChildPath "components.json"
    $HelpersFile = Join-Path -Path $ScriptDir -ChildPath "components_json_helpers.ps1"
    
    if (-not (Test-Path $HelpersFile)) {
        throw "Helper script not found at $HelpersFile"
    }
    
    if (-not (Test-Path $ComponentsJsonFile)) {
        throw "Components JSON file not found at $ComponentsJsonFile"
    }
    
    . $HelpersFile

    $componentsJson = Get-Content $ComponentsJsonFile | Out-String | ConvertFrom-Json
    $packages = GetPackagesFromComponentsJson $componentsJson

    $cseDownloadLocation = "c:\akse-cache\"
    $csePackageFileName = Split-Path -Path $packages[$cseDownloadLocation][0] -Leaf

    # Validate and set the output directory
    if ([string]::IsNullOrEmpty($OutputDir)) {
        if ($Inplace) {
            # when not specified, put zip at the same level as the cse windows/ folder
            $OutputDir = $CSEScriptDir
        }
        else {
            throw "Missing output directory parameter. Please specify an output directory -OutputDir."
        }
    }

    $outputFilePath = Join-Path -Path $OutputDir -ChildPath $csePackageFileName

    # Return the parsed environment variables
    return @{
        CSEScriptDir = $CSEScriptDir
        CSEOutputFilePath = $outputFilePath
    }
}

function Build-WindowsCSEPackage {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory=$false)]
        [string]$ScriptDir = "$env:HelperScriptDir",
        [Parameter(Mandatory=$false)]
        [string]$ComponentConfigDir = "$env:ComponentConfigDir",
        [Parameter(Mandatory=$false)]
        [string]$CSEScriptDir = "$env:CSEScriptDir", # The source directory containing Windows CSE scripts
        [Parameter(Mandatory=$false)]
        [string]$OutputDir = "$env:CSEOutputDir",
        [Parameter(Mandatory=$false)]
        [bool]$Inplace = $false # If true, the package will be created in the same directory to replace the CSE scripts 
    )
    # Parse environment variables
    $env = Get-WindowsCSEEnvironment -ScriptDir $ScriptDir -ComponentConfigDir $ComponentConfigDir -CSEScriptDir $CSEScriptDir -OutputDir $OutputDir -Inplace $Inplace
    
    # Extract the needed variables from the returned object
    $CSEScriptDir = $env.CSEScriptDir
    $CSEOutputFilePath = $env.CSEOutputFilePath

    $cseScriptPath = Join-Path -Path $CSEScriptDir -ChildPath "windows"
    if (-not (Test-Path -Path $cseScriptPath)) {
        throw "The CSE script path '$cseScriptPath' does not exist."
    }

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
    
    Write-Log  "Building Windows CSE scripts package: $CSEOutputFilePath"
    if (Test-Path -Path $CSEOutputFilePath) {
        Remove-Item -Path $CSEOutputFilePath -Force
    }

    # Create the zip file
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [System.IO.Compression.ZipFile]::CreateFromDirectory($workingDir, $CSEOutputFilePath)

    Remove-Item -Path $workingDir -Recurse -Force
    Write-Log "Package created successfully at: $CSEOutputFilePath"

    return @{
        CSEOutputFilePath = $CSEOutputFilePath
    }
}

# # Execute the function when the script is run directly
# Build-WindowsCSEPackage -ScriptDir $ScriptDir -ComponentConfigDir $ComponentConfigDir -CSEScriptDir $CSEScriptDir -OutputDir $OutputDir -Inplace $Inplace
