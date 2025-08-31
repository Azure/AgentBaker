
param(
    [string]
    $outputDirParam,
    [string]
    $helpersFileParam,
    [string]
    $windowsSettingsFileParam,
    [string]
    $componentsJsonFileParam
)

$HelpersFile = "vhdbuilder/packer/windows/components_json_helpers.ps1"
$WindowsSettingsFile = "vhdbuilder/packer/windows/windows_settings.json"
$ComponentsJsonFile = "parts/common/components.json"
$outputDir = "temp"

if (![string]::IsNullOrEmpty($outputDirParam))
{
    Write-Output "Setting output dir to to to $outputDirParam"
    $outputDir = $outputDirParam
} else {
    Write-Output "using default output dir: $outputDir"
}

if (![string]::IsNullOrEmpty($helpersFileParam))
{
    Write-Output "Setting helpers file to $helpersFileParam"
    $HelpersFile = $helpersFileParam
} else {
    Write-Output "using default helpers file: $HelpersFile"
}

if (![string]::IsNullOrEmpty($windowsSettingsFileParam))
{
    Write-Output "Setting windows settings file to $windowsSettingsFileParam"
    $WindowsSettingsFile = $windowsSettingsFileParam
} else {
    Write-Output "using default windows settings: $WindowsSettingsFile"
}

if (![string]::IsNullOrEmpty($componentsJsonFileParam))
{
    Write-Output "Setting components json file to to $componentsJsonFileParam"
    $ComponentsJsonFile = $componentsJsonFileParam
} else {
    Write-Output "using default components json: $ComponentsJsonFile"
}

. "$HelpersFile"

$CPU_ARCH="cpu-arch"

$componentsJson = Get-Content $ComponentsJsonFile | Out-String | ConvertFrom-Json
$windowsSettingsJson = Get-Content $WindowsSettingsFile | Out-String | ConvertFrom-Json
$BaseVersions = GetWindowsBaseVersions $windowsSettingsJson

foreach ($WindowsSku in $BaseVersions)
{
    $cachedThings = GetAllCachedThings $componentsJson $windowsSettingsJson

    $fileName = "${outputDir}/${WindowsSku}.txt"
    Write-Output "Creating file $fileName"
    Write-Output $WindowsSku > $fileName

    echo $cachedThings | Set-Content -Path $fileName
}