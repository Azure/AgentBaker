# The cleanup process for `HNS Network` and `CNI data` should be consistent.

$Global:ClusterConfiguration = ConvertFrom-Json ((Get-Content "c:\k\kubeclusterconfig.json" -ErrorAction Stop) | out-string)

$global:IsSkipCleanupNetwork = [System.Convert]::ToBoolean($Global:ClusterConfiguration.Services.IsSkipCleanupNetwork)

filter Timestamp { "$(Get-Date -Format o): $_" }

function Write-Log($message) {
    $message | Timestamp | Write-Host
}

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
    Write-Log "Cleaning up containers"
    ctr.exe -n k8s.io c ls -q | ForEach-Object { ctr -n k8s.io tasks kill $_ }
    ctr.exe -n k8s.io c ls -q | ForEach-Object { ctr -n k8s.io c rm $_ }

    Write-Log "Cleaning up persisted HNS policy lists"
    # Initially a workaround for https://github.com/kubernetes/kubernetes/pull/68923 in < 1.14,
    # and https://github.com/kubernetes/kubernetes/pull/78612 for <= 1.15
    #
    # October patch 10.0.17763.1554 introduced a breaking change
    # which requires the hns policy list to be removed before network if it gets into a bad state
    # See https://github.com/Azure/aks-engine/pull/3956#issuecomment-720797433 for more info
    # Kubeproxy doesn't fail because errors are not handled:
    # https://github.com/delulu/kubernetes/blob/524de768bb64b7adff76792ca3bf0f0ece1e849f/pkg/proxy/winkernel/proxier.go#L532
    Get-HnsPolicyList | Remove-HnsPolicyList

    if ($global:IsSkipCleanupNetwork) {
        Write-Log "Remove HNS Endpoint before removing HNS Network."
        Get-HnsEndpoint | Remove-HnsEndpoint
        Remove-HnsNetwork $hnsNetwork

        Write-Log  "Count actively reserved port pools to log."
        # Guarantee (10s <= wait time <= 60s) between HNS removal vs. creation for safety.
        # Usually, it takes 10s to sleep like before, but it may take longer in some cases (e.g., COSMIC).
        $maxIteration=6
        $countPortPools = 1
        For ($i=1; ($i -le $maxIteration -and $countPortPools -ne 0); $i++) {
            try {
                $countPortPools = (Invoke-HnsRequest -Method "GET" -Type "portpools" | Select-Object -ExpandProperty PortPoolAllocations | Where-Object -FilterScript {$_.RequiresHostPortReservation -eq "True" -and $_.PortRanges} | Select-Object -ExpandProperty PortRanges).Count
                Write-Log "Waiting for HNS to release $countPortPools held port pools ($i/$maxIteration)..."
            } catch {
                Write-Log "Failed to get port pools. Error: $_"
            }
            Start-Sleep 10
        }
    } else {
        # Legacy codes
        Write-Log "Original code. Remove HNS Network and sleep 10s."
        Remove-HnsNetwork $hnsNetwork
        Start-Sleep 10
    }

    Write-Log "Cleaning up old HNS network completed"
} else {
    Write-Log "No hnsNetwork" # Log: No hnsNetwork when provisioning a new node
}


if ($global:NetworkPlugin -eq "azure") {
    Write-Log "NetworkPlugin azure, starting kubelet."

    Write-Log "Cleaning stale CNI data"
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
            Write-Log "Deleting stale file at $file"
            Remove-Item $file
        }
    }
}
