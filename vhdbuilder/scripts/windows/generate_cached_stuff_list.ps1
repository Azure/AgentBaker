
$HelpersFile = "vhdbuilder/packer/windows/components_json_helpers.ps1"
$WindowsSettingsFile = "vhdbuilder/packer/windows/windows_settings.json"
$ComponentsJsonFile = "parts/common/components.json"

. "$HelpersFile"

$componentsJson = Get-Content $ComponentsJsonFile | Out-String | ConvertFrom-Json
$windowsSettingsJson = Get-Content $WindowsSettingsFile | Out-String | ConvertFrom-Json

$BaseVersions = GetWindowsBaseVersions $windowsSettingsJson

foreach ($WindowsSku in $BaseVersions)
{

    $patch_data = GetPatchInfo $windowsSKU $windowsSettingsJson
    $patchUrls = $patch_data | % { $_.url }
    $patchIDs = $patch_data | % { $_.id }

    $imagesToPull = GetComponentsFromComponentsJson $componentsJson
    $keysToSet = GetRegKeysToApply $windowsSettingsJson
    $map = GetPackagesFromComponentsJson $componentsJson
    $releaseNotesToSet = GetKeyMapForReleaseNotes $windowsSettingsJson

    $fileName = "parts/common/components_per_vhd/${WindowsSku}.txt"
    Write-Output $WindowsSku > $fileName

    Write-Output "---- Patch Data ----" >> $fileName
    echo $patchData | ConvertTo-Json | Write-Output >> $fileName
    Write-Output "">> $fileName
    Write-Output  "---- Container Images to Pull ----" >> $fileName
    echo $imagesToPull | ConvertTo-Json | Write-Output >> $fileName
    Write-Output "" >> $fileName
    Write-Output  "---- Packages to Download ----" >> $fileName
    echo $map | ConvertTo-Json | Write-Output >> $fileName
    Write-Output  "---- Win Reg Keys ----" >> $fileName
    echo $keysToSet | ConvertTo-Json | Write-Output >> $fileName
    Write-Output "" >> $fileName
}