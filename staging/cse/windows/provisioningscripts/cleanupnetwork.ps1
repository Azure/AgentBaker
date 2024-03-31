$Global:ClusterConfiguration = ConvertFrom-Json ((Get-Content "c:\k\kubeclusterconfig.json" -ErrorAction Stop) | out-string)

$global:NetworkMode = "L2Bridge"
$global:NetworkPlugin = $Global:ClusterConfiguration.Cni.Name
$global:HNSModule = "c:\k\hns.v2.psm1"

ipmo $global:HNSModule

# This script will remove the azure HNS network when provisioning node / restarting node
$networkname = $global:NetworkMode.ToLower()
if ($global:NetworkPlugin -eq "azure") {
    $networkname = "azure" #
}
$hnsNetwork = Get-HnsNetwork | ? Name -EQ $networkname
if ($hnsNetwork) {
    # Cleanup all containers
    Write-Host "Cleaning up containers" @
    ctr.exe -n k8s.io c ls -q | ForEach-Object { ctr -n k8s.io tasks kill $_ }
    ctr.exe -n k8s.io c ls -q | ForEach-Object { ctr -n k8s.io c rm $_ }
    
    Write-Host "Cleaning up persisted HNS policy lists" 
    # reasons:
    # Initially a workaround for https://github.com/kubernetes/kubernetes/pull/68923 in < 1.14,
    # and https://github.com/kubernetes/kubernetes/pull/78612 for <= 1.15
    #
    # October patch 10.0.17763.1554 introduced a breaking change 
    # which requires the hns policy list to be removed before network if it gets into a bad state
    # See https://github.com/Azure/aks-engine/pull/3956#issuecomment-720797433 for more info
    # Kubeproxy doesn't fail becuase errors are not handled: 
    # https://github.com/delulu/kubernetes/blob/524de768bb64b7adff76792ca3bf0f0ece1e849f/pkg/proxy/winkernel/proxier.go#L532
    # 
    Get-HnsPolicyList | Remove-HnsPolicyList

    Write-Host "Cleaning up old HNS network found"
    Remove-HnsNetwork $hnsNetwork # Remove the azure HNS network
    Start-Sleep 10
}
# When provsioning node / restarting node, the azure HNS network will be removed,
# so I need to recreate the azure HNS network in somewhere else, for example:
function Create-AzureHNSNetwork {
    # Read configuration from "C:\k\azurehnsnetwork.config"
    $azurehnsnetworkconfig = Get-Content "C:\k\azurehnsnetwork.config" -ErrorAction Stop
    # Create the azure HNS network with the configuration
    $hnsNetwork = New-HnsNetwork -JsonConfig $azurehnsnetworkconfig
    # Update the network DNS
    Update-HnsNetworkDns -ID $hnsNetwork.ID -DnsServers @("DNS1", "DNS2")
    # Other configurations can be updated here..........
    Write-Log "Created Azure HNS network: $($hnsNetwork)"
}

if ($global:NetworkPlugin -eq "azure") {
    Write-Host "NetworkPlugin azure, starting kubelet."

    Write-Host "Cleaning stale CNI data"
    # Kill all cni instances & stale data left by cni
    # Cleanup all files related to cni
    taskkill /IM azure-vnet.exe /f
    taskkill /IM azure-vnet-ipam.exe /f

    # azure-cni logs currently end up in c:\windows\system32 when machines are configured with containerd.
    # https://github.com/containerd/containerd/issues/4928
    $filesToRemove = @(
        "c:\k\azure-vnet.json",
        "c:\k\azure-vnet.json.lock",
        "c:\k\azure-vnet-ipam.json",
        "c:\k\azure-vnet-ipam.json.lock"
        "c:\k\azure-vnet-ipamv6.json",
        "c:\k\azure-vnet-ipamv6.json.lock"
        "c:\windows\system32\azure-vnet.json",
        "c:\windows\system32\azure-vnet.json.lock",
        "c:\windows\system32\azure-vnet-ipam.json",
        "c:\windows\system32\azure-vnet-ipam.json.lock"
        "c:\windows\system32\azure-vnet-ipamv6.json",
        "c:\windows\system32\azure-vnet-ipamv6.json.lock",
        "c:\k\azurecns\azure-endpoints.json"
    )

    foreach ($file in $filesToRemove) {
        if (Test-Path $file) {
            Write-Host "Deleting stale file at $file"
            Remove-Item $file
        }
    }
}