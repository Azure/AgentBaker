# The cleanup process for `HNS Policy` and `CNI data` should be consistent.

$Global:ClusterConfiguration = ConvertFrom-Json ((Get-Content "c:\k\kubeclusterconfig.json" -ErrorAction Stop) | out-string)

$IsCleanupHnsNetwork = [System.Convert]::ToBoolean($Global:ClusterConfiguration.Services.IsCleanupHnsNetwork) # CleanupHnsNetwork is legacy code

$global:NetworkMode = "L2Bridge"
$global:NetworkPlugin = $Global:ClusterConfiguration.Cni.Name
$global:HNSModule = "c:\k\hns.v2.psm1"

ipmo $global:HNSModule

$networkname = $global:NetworkMode.ToLower()
if ($global:NetworkPlugin -eq "azure") {
    $networkname = "azure"
}

$hnsNetwork = Get-HnsNetwork | ? Name -EQ $networkname
if ($hnsNetwork) {
    # Cleanup all containers
    Write-Host "Cleaning up containers"
    ctr.exe -n k8s.io c ls -q | ForEach-Object { ctr -n k8s.io tasks kill $_ }
    ctr.exe -n k8s.io c ls -q | ForEach-Object { ctr -n k8s.io c rm $_ }
    
    Write-Host "Cleaning up persisted HNS policy lists"
    # Initially a workaround for https://github.com/kubernetes/kubernetes/pull/68923 in < 1.14,
    # and https://github.com/kubernetes/kubernetes/pull/78612 for <= 1.15
    #
    # October patch 10.0.17763.1554 introduced a breaking change 
    # which requires the hns policy list to be removed before network if it gets into a bad state
    # See https://github.com/Azure/aks-engine/pull/3956#issuecomment-720797433 for more info
    # Kubeproxy doesn't fail becuase errors are not handled: 
    # https://github.com/delulu/kubernetes/blob/524de768bb64b7adff76792ca3bf0f0ece1e849f/pkg/proxy/winkernel/proxier.go#L532
    Get-HnsPolicyList | Remove-HnsPolicyList

    Write-Host "Cleaning up old HNS network found"

    if (-not $IsCleanupHnsNetwork) {
        # Original code
        Remove-HnsNetwork $hnsNetwork
        Start-Sleep 10
    } else {
        # New code
        # David Schott: Remove-HnsEndpoint before Remove-HnsNetwork
        Get-HnsEndpoint | Remove-HnsEndpoint
        Remove-HnsNetwork $hnsNetwork
        # Count actively reserved port pools
        $countPortPools = 1
        # David Schott: $maxIteration=23 # Shiqian Tao: I agree to use 240s to get more logs in the private CSE, and reduce the duration in the official CSE.
        $maxIteration=6 # Guarantee (10s <= wait time <= 60s) between HNS removal vs. creation for safety.
        For ($i=1; ($i -le $maxIteration -and $countPortPools -ne 0); $i++) {
            Start-Sleep 10
            $countPortPools = (Invoke-HnsRequest -Method "GET" -Type "portpools" | Select-Object -ExpandProperty PortPoolAllocations | Where-Object -FilterScript {$_.RequiresHostPortReservation -eq "True" -and $_.PortRanges} | Select-Object -ExpandProperty PortRanges).Count
            Write-Host "Waiting for HNS to release $countPortPools held port pools ($i/$maxIteration)..." #TODO, I think we need to log this to a file
        }
    }
    Write-Host "Cleaning up old HNS network completed"
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