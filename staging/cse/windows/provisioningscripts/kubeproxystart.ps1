$Global:ClusterConfiguration = ConvertFrom-Json ((Get-Content "c:\k\kubeclusterconfig.json" -ErrorAction Stop) | out-string)
$KubeproxyFeatureGates = $Global:ClusterConfiguration.Kubernetes.Kubeproxy.FeatureGates # This is the initial feature list passed in from aks-engine
$KubernetesVersion = $Global:ClusterConfiguration.Kubernetes.Source.Release
$global:IsSkipCleanupNetwork = [System.Convert]::ToBoolean($Global:ClusterConfiguration.Services.IsSkipCleanupNetwork)

# comparison function for 2 semantic versions
# returns 1 if Version1 > Version 2, -1 if Version1 < Version2, and 0 if equal
function Compare-SemanticVersion {
    [CmdletBinding()]
    param (
        [Parameter(Mandatory=$true, Position=0)]
        [string]$Version1,
        [Parameter(Mandatory=$true, Position=1)]
        [string]$Version2
    )

    $version1Parts = $Version1.Split(".")
    $version2Parts = $Version2.Split(".")

    $maxCount = 0
    if ($version1Parts.Count > $version2Parts.Count) {
        $maxCount = $version1Parts.Count
    } else {
        $maxCount = $version2Parts.Count
    }

    for ($i = 0; $i -lt $maxCount; $i++) {
        $version1Part = if ($i -lt $version1Parts.Count) { [int]$version1Parts[$i] } else { 0 }
        $version2Part = if ($i -lt $version2Parts.Count) { [int]$version2Parts[$i] } else { 0 }

        if ($version1Part -lt $version2Part) {
            return -1
        }
        elseif ($version1Part -gt $version2Part) {
            return 1
        }
    }

    return 0
}

$KubeNetwork = "azure"

$env:KUBE_NETWORK = $KubeNetwork
$global:HNSModule = "c:\k\hns.v2.psm1"
$global:KubeDir = $Global:ClusterConfiguration.Install.Destination
$global:KubeproxyArgList = @("--v=3", "--proxy-mode=kernelspace", "--hostname-override=$env:computername", "--kubeconfig=$KubeDir\config")

if ($Global:ClusterConfiguration.Kubernetes.Kubeproxy.ConfigArgs) {
    Write-Host "Customized args: $($Global:ClusterConfiguration.Kubernetes.Kubeproxy.ConfigArgs)"
    $global:KubeproxyArgList += $Global:ClusterConfiguration.Kubernetes.Kubeproxy.ConfigArgs
}

if ($global:IsSkipCleanupNetwork) {
    Write-Host "Skipping legacy code: kube-proxy waits for network to be created"
} else {
    # Legacy codes
    $hnsNetwork = Get-HnsNetwork | ? Name -EQ $KubeNetwork
    while (!$hnsNetwork) {
        Write-Host "$(Get-Date -Format o) Waiting for Network [$KubeNetwork] to be created . . ."
        Start-Sleep 10
        $hnsNetwork = Get-HnsNetwork | ? Name -EQ $KubeNetwork
    }
}

# enable WinDsr if WinDsr feature gate is enabled
if ($KubeproxyFeatureGates -contains "WinDSR=true") {
    $global:KubeproxyArgList += @("--enable-dsr=true")
}

$featureGateArgs = ""
foreach ($feature in $KubeproxyFeatureGates) {
    # IPv6DualStack feature gate should not be passed to kube-proxy in >= 1.25.0
    # https://github.com/kubernetes/kubernetes/blob/ef70d260f3d036fc22b30538576bbf6b36329995/pkg/features/kube_features.go#L945
    if (($feature -like "IPv6DualStack=*") -and ((Compare-SemanticVersion -Version1 $KubernetesVersion -Version2 "1.25.0") -ge 0)) {
        continue
    }

    if ($featureGateArgs -ne "") {
        $featureGateArgs += ","
    }

    $featureGateArgs += $feature
}

if ($featureGateArgs -ne "") {
    $global:KubeproxyArgList += @("--feature-gates=" + $featureGateArgs)
}

if ($global:IsSkipCleanupNetwork) {
    Write-Host "Skipping legacy code: kube-proxy Remove-HnsPolicyList"
} else {
    # Legacy codes
    # cleanup the persisted policy lists
    Import-Module $global:HNSModule
    # Workaround for https://github.com/kubernetes/kubernetes/pull/68923 in < 1.14,
    # and https://github.com/kubernetes/kubernetes/pull/78612 for <= 1.15
    Get-HnsPolicyList | Remove-HnsPolicyList
}

# Use run-process.cs to set process priority class as 'AboveNormal'
# Load a signed version of runprocess.dll if it exists for Azure SysLock compliance
# otherwise load class from cs file (for CI/testing)
if (Test-Path "$global:KubeDir\runprocess.dll") {
    [System.Reflection.Assembly]::LoadFrom("$global:KubeDir\runprocess.dll")
} else {
    Add-Type -Path "$global:KubeDir\run-process.cs"
}
$exe = "$global:KubeDir\kube-proxy.exe"
$args = $global:KubeproxyArgList -join " "
[RunProcess.exec]::RunProcess($exe, $args, [System.Diagnostics.ProcessPriorityClass]::AboveNormal)
