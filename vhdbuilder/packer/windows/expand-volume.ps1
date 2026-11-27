$osDrive = ((Get-WmiObject Win32_OperatingSystem -ErrorAction Stop).SystemDrive).TrimEnd(":")
$diskpartScriptPath = "c:\aks-tools\diskpart.script"
[String]::Format("select volume {0}`nextend`nexit", $osDrive) | Out-File -Encoding "UTF8" $diskpartScriptPath -Force
Start-Process -FilePath diskpart.exe -ArgumentList "/s $diskpartScriptPath" -Wait
Unregister-ScheduledTask -TaskName "aks-expand-volume" -Confirm:$false
Remove-Item -Path "c:\aks-tools\expand-volume.ps1" -Force
Remove-Item -Path $diskpartScriptPath -Force
