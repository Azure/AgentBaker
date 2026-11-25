<#
.DESCRIPTION
    This script extends the OS volume to use the whole OS disk. The aks-expand-volume scheduled task,
    registered during the VHD build, runs it once at startup; the script then removes the task and itself.
    It ships in the CSE scripts package so that it is signed with the other CSE scripts.
#>

$osDrive = ((Get-WmiObject Win32_OperatingSystem -ErrorAction Stop).SystemDrive).TrimEnd(":")
$diskpartScriptPath = "c:\aks-tools\diskpart.script"
[String]::Format("select volume {0}`nextend`nexit", $osDrive) | Out-File -Encoding "UTF8" $diskpartScriptPath -Force
Start-Process -FilePath diskpart.exe -ArgumentList "/s $diskpartScriptPath" -Wait
# Run once and remove the task. Sequence: task invokes ps1, ps1 invokes diskpart.
Unregister-ScheduledTask -TaskName "aks-expand-volume" -Confirm:$false
Remove-Item -Path "c:\aks-tools\expand-volume.ps1" -Force
Remove-Item -Path $diskpartScriptPath -Force
