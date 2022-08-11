<#
.DESCRIPTION
    HNS service may crash and HNS policies will be purged after it is restarted.
    We use this script to restart kubeproxy to recover the node from the hns crash.

    Start sequence:
    1. when $Global:ClusterConfiguration.Services.HNSRemediator.Enabled in c:\k\kubeclusterconfig.json is true:
      a) windowsnodereset.ps1 deletes hns-remediator-task
      b) windowsnodereset.ps1 deletes "C:\k\hns.pid"
    2. windowsnodereset.ps1 resets all services, hns, csi, kubeproxy, kubelet, etc.
    3. when $Global:ClusterConfiguration.Services.HNSRemediator.Enabled in c:\k\kubeclusterconfig.json is true:
      a) windowsnodereset.ps1 creates hns-remediator-task with $Global:ClusterConfiguration.Services.HNSRemediator.IntervalInMinutes in c:\k\kubeclusterconfig.json

    NOTES:
    1. We cannot run hns-remediator-task with an interval less than 1 minute since the RepetitionInterval parameter value in New-JobTrigger must be greater than 1 minute.
    2. When the node crashes or is rebooted, hns-remediator-task may restart kubeproxy before windowsnodereset.ps1 is executed.
       It should have no impact since windowsnodereset.ps1 always deletes hns-remediator-task and then deletes "C:\k\hns.pid" before stopping kubeproxy
#>

$LogPath = "c:\k\hnsremediator.log"
$hnsPIDFilePath="C:\k\hns.pid"
$isInitialized=$False

filter Timestamp { "$(Get-Date -Format o): $_" }

function Write-Log ($message) {
    $message | Timestamp | Tee-Object -FilePath $LogPath -Append
}

if (Test-Path -Path $hnsPIDFilePath) {
    $isInitialized=$True
}

$id = Get-WmiObject -Class Win32_Service -Filter "name='hns'" | Select-Object -ExpandProperty ProcessId
if (!$isInitialized) {
    Write-Log "Initializing with creating $hnsPIDFilePath. PID of HNS service is $id"
    echo $id > $hnsPIDFilePath
    $isInitialized=$True
}

$lastId=Get-Content $hnsPIDFilePath
if ($lastId -ne $id) {
    Write-Log "The PID of HNS service was changed from $lastId to $id"
    echo $id > $hnsPIDFilePath

    Write-Log "Restarting kubeproxy service"
    Restart-Service kubeproxy
}
