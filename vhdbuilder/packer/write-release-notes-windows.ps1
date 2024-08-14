<#
    .SYNOPSIS
        Produces a release notes file for a Windows VHD

    .DESCRIPTION
        Produces a release notes file for a Windows VHD
#>

$ErrorActionPreference = "Stop"

$releaseNotesFilePath = "c:\release-notes.txt"

function Log($Message) {
    # Write-Output $Message
    $Message | Tee-Object -FilePath $releaseNotesFilePath -Append
}

Log "Build Number: $env:BUILD_NUMBER"
Log "Build Id:     $env:BUILD_ID"
Log "Build Repo:   $env:BUILD_REPO"
Log "Build Branch: $env:BUILD_BRANCH"
Log "Commit:       $env:BUILD_COMMIT"
Log ""

$vhdId = Get-Content 'c:\vhd-id.txt'
Log ("VHD ID:      $vhdId")
Log ""

Log "System Info"
$systemInfo = Get-ItemProperty -Path 'HKLM:SOFTWARE\Microsoft\Windows NT\CurrentVersion'
Log ("`t{0,-14} : {1}" -f "OS Name", $systemInfo.ProductName)
Log ("`t{0,-14} : {1}" -f "OS Version", "$($systemInfo.CurrentBuildNumber).$($systemInfo.UBR)")
Log ("`t{0,-14} : {1}" -f "OS InstallType", $systemInfo.InstallationType)
Log ""

$allowedSecurityProtocols = [System.Net.ServicePointManager]::SecurityProtocol
Log "Allowed security protocols: $allowedSecurityProtocols"
Log ""

Log "Installed Features"
if ($systemInfo.InstallationType -ne 'client') {
    Log (Get-WindowsFeature | Where-Object Installed)
}
else {
    Log "`t<Cannot enumerate installed features on client skus>"
}
Log ""


Log "Installed Packages"
$packages = Get-WindowsCapability -Online | Where-Object { $_.State -eq 'Installed' }
foreach ($package in $packages) {
    Log ("`t{0}" -f $package.Name)
}
Log ""

Log "Installed QFEs"
$qfes = Get-HotFix
foreach ($qfe in $qfes) {
    $link = "https://support.microsoft.com/kb/{0}" -f ($qfe.HotFixID.Replace("KB", ""))
    Log ("`t{0,-9} : {1, -15} : {2}" -f $qfe.HotFixID, $Qfe.Description, $link)
}
Log ""

Log "Installed Updates"
$updateSession = New-Object -ComObject Microsoft.Update.Session
$updateSearcher = $UpdateSession.CreateUpdateSearcher()
$updates = $updateSearcher.Search("IsInstalled=1").Updates
foreach ($update in $updates) {
    Log ("`t{0}" -f $update.Title)
}
Log ""

Log "Windows Update Registry Settings"
Log "`thttps://docs.microsoft.com/en-us/windows/deployment/update/waas-wu-settings"

$wuRegistryKeys = @(
    "HKLM:SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate",
    "HKLM:SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate\AU",
    "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State",
    "HKLM:\SYSTEM\CurrentControlSet\Services\wcifs",
    "HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft\FeatureManagement\Overrides",
    "HKLM:\SYSTEM\CurrentControlSet\Services\VfpExt\Parameters",
    "HKLM:\SYSTEM\CurrentControlSet\Control\Windows Containers"
)

$wuRegistryNames = @(
    "NoAutoUpdate",
    "EnableCompartmentNamespace",
    "HNSControlFlag",
    "HnsPolicyUpdateChange",
    "HnsNatAllowRuleUpdateChange",
    "HnsAclUpdateChange",
    "HNSFixExtensionUponRehydration",
    "HnsNpmRefresh",
    "HnsNodeToClusterIpv6",
    "HNSNpmIpsetLimitChange",
    "HNSLbNatDupRuleChange",
    "HNSUpdatePolicyForEndpointChange",
    "WcifsSOPCountDisabled",
    "3105872524",
    "2629306509",
    "3508525708",
    "1995963020",
    "189519500",
    "VfpEvenPodDistributionIsEnabled",
    "VfpIpv6DipsPrintingIsEnabled",
    "3230913164",
    "3398685324",
    "87798413",
    "4289201804",
    "1355135117",
    "RemoveSourcePortPreservationForRest",
    "2214038156",
    "VfpNotReuseTcpOneWayFlowIsEnabled",
    "1673770637",
    "FwPerfImprovementChange",
    "CleanupReservedPorts",
    "652313229",
    "2059235981",
    "3767762061",
    "527922829",
    "DeltaHivePolicy",
    "2193453709",
    "3331554445",
    "1102009996",
    "OverrideReceiveRoutingForLocalAddressesIpv4",
    "OverrideReceiveRoutingForLocalAddressesIpv6",
    "1327590028",
    "1114842764",
    "HnsPreallocatePortRange",
    "4154935436",
    "124082829",
    "PortExclusionChange",
    "2290715789",
    "3152880268",
    "3744292492",
    "3838270605",
    "851795084",
    "26691724",
    "3834988172",
    "1535854221",
    "3632636556",
    "1552261773",
    "4186914956",
    "3173070476",
    "3958450316",
    "1605443213",
    "2540111500",
    "50261647",
    "1475968140",
    "747051149",
    "260097166",
    "1800977551"
)

foreach ($key in $wuRegistryKeys) {
    # Windows 2019 does not have the Windows Containers key
    if ($($systemInfo.CurrentBuildNumber) -eq 17763 -and $key -eq "HKLM:\SYSTEM\CurrentControlSet\Control\Windows Containers") {
        continue
    }
    $regPath=(Get-Item -Path $key -ErrorAction Ignore)
    if ($regPath) {
        Log ("`t{0}" -f $key)
        Get-Item -Path $key |
        Select-Object -ExpandProperty property |
        ForEach-Object {
            if ($wuRegistryNames -contains $_) {
                Log ("`t`t{0} : {1}" -f $_, (Get-ItemProperty -Path $key -Name $_).$_)
            }
        }
    }
}
Log ""

Log "ContainerD Info"
# starting containerd for printing containerD info, the same way as we pre-pull containerD images in configure-windows-vhd.ps1
Start-Job -Name containerd -ScriptBlock { containerd.exe }
$containerDVersion = (ctr.exe --version) | Out-String
Log ("Version: {0}" -f $containerDVersion)
Log "Images:"
Log (ctr.exe -n k8s.io image ls)
Stop-Job  -Name containerd
Remove-Job -Name containerd
Log ""

Log "Cached Files:"
$displayObjects = @()
foreach ($file in [IO.Directory]::GetFiles('c:\akse-cache', '*', [IO.SearchOption]::AllDirectories))
{
    $attributes = Get-Item $file
    $hash = Get-FileHash $file -Algorithm SHA256
    $displayObjects += New-Object psobject -property @{
        File = $file;
        SizeBytes = $attributes.Length;
        Sha256 = $hash.Hash
    }
}

Log ($displayObjects | Format-Table -Property File, Sha256, SizeBytes | Out-String -Width 4096)

# Ensure proper encoding is set for release notes file
[IO.File]::ReadAllText($releaseNotesFilePath) | Out-File -Encoding utf8 $releaseNotesFilePath
