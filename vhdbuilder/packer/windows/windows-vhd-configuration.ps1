# TODO - over time this file should contain less and less info, and really just source the json and helpers file. Then that logic can be moved into
# the scripts that use this file and this file can be deleted.


$global:windowsSKU = $env:WindowsSKU

# We use the same temp dir for all temp tools that will be used for vhd build
$global:aksTempDir = "c:\akstemp"

# We use the same dir for all tools that will be used in AKS Windows nodes
$global:aksToolsDir = "c:\aks-tools"

# We cache images and packages in this directory.
$global:cacheDir = "c:\akse-cache"

# We need to guarantee that the node provisioning will not fail because the vhd is full before resize-osdisk is called in AKS Windows CSE script.
$global:lowestFreeSpace = 1*1024*1024*1024 # 1GB

$cpu = Get-WmiObject -Class Win32_Processor
$CPU_ARCH = switch ($cpu.Architecture) {
    0 { "amd64" } # x86
    1 { "" } # MIPS
    2 { "" } # Alpha
    3 { "" } # PowerPC
    5 { "arm64" } # ARM
    6 { "amd64" } # Itanium
    9 { "amd64" } # x64
    default { "" }
}

if ([string]::IsNullOrEmpty($CPU_ARCH)) {
    $cpuName = $cpu.Name
    $cpuArch = $cpu.Architecture
    Write-Host "Unknown architecture for CPU $cpuName with arch $cpuArch"
    throw "Unsupported architecture for SKU $windowsSKU for CPU $cpuName with arch $cpuArch"
}

$HelpersFile = "c:/k/components_json_helpers.ps1"
$ComponentsJsonFile = "c:/k/components.json"
$WindowsSettingsFile = "c:/k/windows_settings.json"

# fallback in case we're running in a test pipeline or locally
if (!(Test-Path $HelpersFile))
{
    $HelpersFile = "vhdbuilder/packer/windows/components_json_helpers.ps1"
}

if (!(Test-Path $WindowsSettingsFile))
{
    $WindowsSettingsFile = "vhdbuilder/packer/windows/windows_settings.json"
}

if (!(Test-Path $ComponentsJsonFile))
{
    $ComponentsJsonFile = "parts/common/components.json"
}

Write-Host "Components JSON: $ComponentsJsonFile"
Write-Host "Helpers Ps1: $HelpersFile"
Write-Host "WindowsSettingsFile: $WindowsSettingsFile"

. "$HelpersFile"

$componentsJson = Get-Content $ComponentsJsonFile | Out-String | ConvertFrom-Json
$windowsSettingsJson = Get-Content $WindowsSettingsFile | Out-String | ConvertFrom-Json
$patch_data = GetPatchInfo $windowsSKU $windowsSettingsJson
$global:patchUrls = $patch_data | % { $_.url }
$global:patchIDs = $patch_data | % { $_.id }

$global:imagesToPull = GetComponentsFromComponentsJson $componentsJson
$global:ociArtifactsToPull = GetOCIArtifactsFromComponentsJson $componentsJson
$global:keysToSet = GetRegKeysToApply $windowsSettingsJson
$global:map = GetPackagesFromComponentsJson $componentsJson
$global:releaseNotesToSet = GetKeyMapForReleaseNotes $windowsSettingsJson

$validSKU = GetWindowsBaseVersions $windowsSettingsJson
if (-not ($validSKU -contains $windowsSKU))
{
    throw "Unsupported windows image SKU: $windowsSKU"
}

# Different from other packages which are downloaded/cached and used later only during CSE, windows containerd is installed
# during building the Windows VHD to cache container images.
# We use the latest containerd package to start containerd then cache images, and the latest one is expected to be
# specified by AKS PR for most of the cases. BUT as long as there's a new unpacked image version, we should keep the
# versions synced.
$global:defaultContainerdPackageUrl = GetDefaultContainerDFromComponentsJson $componentsJson

# defenderUpdateUrl refers to the latest windows defender platform update
$global:defenderUpdateUrl = GetDefenderUpdateUrl $windowsSettingsJson
# defenderUpdateInfoUrl refers to the info of latest windows defender platform update
$global:defenderUpdateInfoUrl = GetDefenderUpdateInfoUrl $windowsSettingsJson

# The following items still need to be migrated into the windows_settings file.
$global:excludeHashComparisionListInAzureChinaCloud = @(
    "calico-windows",
    "azure-vnet-cni-singletenancy-windows-amd64",
    "azure-vnet-cni-singletenancy-swift-windows-amd64",
    "azure-vnet-cni-singletenancy-overlay-windows-amd64",
    # We need upstream's help to republish this package. Before that, it does not impact functionality and 1.26 is only in public preview
    # so we can ignore the different hash values.
    "v1.26.0-1int.zip"
)

