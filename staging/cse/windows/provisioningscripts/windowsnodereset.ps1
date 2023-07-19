<#
.DESCRIPTION
    This script is intended to be run each time a windows nodes is restarted and performs
    cleanup actions to help ensure the node comes up cleanly.
#>

$global:LogPath = "c:\k\windowsnodereset.log"

$Global:ClusterConfiguration = ConvertFrom-Json ((Get-Content "c:\k\kubeclusterconfig.json" -ErrorAction Stop) | out-string)

$global:HNSRemediatorIntervalInMinutes = [System.Convert]::ToUInt32($Global:ClusterConfiguration.Services.HNSRemediator.IntervalInMinutes)
$global:CsiProxyEnabled = [System.Convert]::ToBoolean($Global:ClusterConfiguration.Csi.EnableProxy)
$global:MasterSubnet = $Global:ClusterConfiguration.Kubernetes.ControlPlane.MasterSubnet
$global:NetworkMode = "L2Bridge"
$global:NetworkPlugin = $Global:ClusterConfiguration.Cni.Name
# if dual-stack is enabled, the clusterCidr will have an IPv6 CIDR in the comma separated list
# we can split the entire string by ":" to get a count of how many ":" there are. If there are
# at least 3 groups (which means there are at least 2 ":") then we know there is an IPv6 CIDR
# in the list. We cannot just rely on `ClusterCidr -like "*::*" because there are IPv6 CIDRs that
# don't have "::", e.g. fe80:0:0:0:0:0:0:0/64
$IsDualStackEnabled = ($Global:ClusterConfiguration.Kubernetes.Kubeproxy.FeatureGates -contains "IPv6DualStack=true") -Or `
                        (($Global:ClusterConfiguration.Kubernetes.Network.ClusterCidr -split ":").Count -ge 3)

$global:HNSModule = "c:\k\hns.v2.psm1"

filter Timestamp { "$(Get-Date -Format o): $_" }

function Write-Log ($message) {
    $message | Timestamp | Tee-Object -FilePath $global:LogPath -Append
}

function Register-HNSRemediatorScriptTask {
    if ($global:HNSRemediatorIntervalInMinutes -ne 0) {
        Write-Log "Creating a scheduled task to run hnsremediator.ps1"

        $action = New-ScheduledTaskAction -Execute "powershell.exe" -Argument "-File `"c:\k\hnsremediator.ps1`""
        $principal = New-ScheduledTaskPrincipal -UserId SYSTEM -LogonType ServiceAccount -RunLevel Highest
        $trigger = New-JobTrigger -Once -At (Get-Date).Date -RepeatIndefinitely -RepetitionInterval (New-TimeSpan -Minutes $global:HNSRemediatorIntervalInMinutes)
        $definition = New-ScheduledTask -Action $action -Principal $principal -Trigger $trigger -Description "hns-remediator-task"
        Register-ScheduledTask -TaskName "hns-remediator-task" -InputObject $definition
    }
}

function Unregister-HNSRemediatorScriptTask {
    # We do not check whether $global:HNSRemediatorIntervalInMinutes is not 0 sicne we may need to set it to 0 in the node for test purpose
    if (Get-ScheduledTask -TaskName "hns-remediator-task" -ErrorAction Ignore) {
        Write-Log "Deleting the scheduled task hns-remediator-task"
        Unregister-ScheduledTask -TaskName "hns-remediator-task" -Confirm:$false
    }

    $hnsPIDFile="C:\k\hns.pid"
    if (Test-Path $hnsPIDFile) {
        # Remove this file since PID of HNS service may have been changed after node crashes or is rebooted
        # It should not always fail since hns-remediator-task is unregistered.
        # We set the max retry count to 20 to avoid dead loop for unknown issues.
        $maxRetries=20
        $retryCount=0
        while ($retryCount -lt $maxRetries) {
            Write-Log "Deleting $hnsPIDFile"
            Remove-Item -Path $hnsPIDFile -Force -Confirm:$false -ErrorAction Ignore

            # The file may not be deleted successfully because hnsremediator.ps1 is still writing the logs
            if (Test-Path $hnsPIDFile) {
                # Do not log the failure to reduce log
                Start-Sleep -Milliseconds 500
                $retryCount=$retryCount+1
            } else {
                Write-Log "$hnsPIDFile is deleted"
                break
            }
        }
    }
}

Write-Log "Entering windowsnodereset.ps1"

Import-Module $global:HNSModule

Unregister-HNSRemediatorScriptTask

#
# Stop services
#
Write-Log "Stopping kubeproxy service"
Stop-Service kubeproxy

Write-Log "Stopping kubelet service"
Stop-Service kubelet

if ($global:CsiProxyEnabled) {
    Write-Log "Stopping csi-proxy service"
    Stop-Service csi-proxy
}

if ($global:EnableHostsConfigAgent) {
    Write-Log "Stopping hosts-config-agent service"
    Stop-Service hosts-config-agent
}

# Due to a bug in hns there is a race where it picks up the incorrect IPv6 address from the node in some cases.
# Hns service has to be restarted after the node internal IPv6 address is available when dual-stack is enabled.
# TODO Remove this once the bug is fixed in hns.
function Restart-HnsService {
    do {
        Start-Sleep -Seconds 1
        $nodeInternalIPv6Address = (Get-NetIPAddress | Where-Object {$_.PrefixOrigin -eq "Dhcp" -and $_.AddressFamily -eq "IPv6"}).IPAddress 
    } while ($nodeInternalIPv6Address -eq $null)
    Write-Log "Got node internal IPv6 address: $nodeInternalIPv6Address"
    
    $hnsManagementIPv6Address = (Get-HnsNetwork | Where-Object {$_.IPv6 -eq $true}).ManagementIPv6
    Write-Log "Got hns ManagementIPv6: $hnsManagementIPv6Address"

    if ($hnsManagementIPv6Address -ne $nodeInternalIPv6Address) {
        Restart-Service hns
        Write-Log "Restarted hns service"

        $hnsManagementIPv6Address = (Get-HnsNetwork | Where-Object {$_.IPv6 -eq $true}).ManagementIPv6
        Write-Log "Got hns ManagementIPv6: $hnsManagementIPv6Address after restart"
    }
    else {
        Write-Log "Hns network has correct IPv6 address, not restarting"
    }
}

if ($IsDualStackEnabled) {
    Restart-HnsService
}

#
# Perform cleanup
#

& "c:\k\cleanupnetwork.ps1"

#
# Start Services
#

if ($global:CsiProxyEnabled) {
    Write-Log "Starting csi-proxy service"
    Start-Service csi-proxy
}

Write-Log "Starting kubelet service"
Start-Service kubelet

Write-Log "Do not start kubeproxy service since kubelet will restart kubeproxy"

Register-HNSRemediatorScriptTask

Write-Log "Exiting windowsnodereset.ps1"
