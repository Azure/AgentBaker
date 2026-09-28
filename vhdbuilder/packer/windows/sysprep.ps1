# Stop and remove Azure Agents to enable use in Azure Stack
# If deploying an Azure VM the agents will be re-added to the VMs at deployment time
$serviceName = 'WindowsAzureGuestAgent'
$gotService = Get-Service -Name $serviceName -ErrorAction Ignore
if ($gotService.Status -eq 'Running'){
    echo "$serviceName is still running, trying to stop now."
    Stop-Service WindowsAzureGuestAgent
}
Stop-Service RdAgent
& sc.exe delete WindowsAzureGuestAgent
& sc.exe delete RdAgent

# Remove the WindowsAzureGuestAgent registry key for sysprep
# This removes AzureGuestAgent from participating in sysprep
# There was an update that is missing VMAgentDisabler.dll
$path = "Registry::HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Windows\CurrentVersion\Setup\SysPrepExternal\Generalize"
$generalizeKey = Get-Item -Path $path
$generalizeProperties = $generalizeKey | Select-Object -ExpandProperty property
$values = $generalizeProperties | ForEach-Object {
    New-Object psobject -Property @{"Name"=$_;
    "Value" = (Get-ItemProperty -Path $path -Name $_).$_}
}

$values | ForEach-Object {
    $item = $_;
    if( $item.Value.Contains("VMAgentDisabler.dll")) {
            Write-HOST "Removing " $item.Name - $item.Value;
            Remove-ItemProperty -Path $path -Name $item.Name;
    }
}

# run Sysprep
if( Test-Path $Env:SystemRoot\\system32\\Sysprep\\unattend.xml ) {  Remove-Item $Env:SystemRoot\\system32\\Sysprep\\unattend.xml -Force }
& $env:SystemRoot\\System32\\Sysprep\\Sysprep.exe /oobe /generalize /mode:vm /quiet /quit

# when done clean up
while($true) { $imageState = Get-ItemProperty HKLM:\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Setup\\State | Select ImageState; if($imageState.ImageState -ne 'IMAGE_STATE_GENERALIZE_RESEAL_TO_OOBE') { Write-Host $imageState.ImageState; Start-Sleep -s 10  } else { break } }
Get-ChildItem c:\\WindowsAzure -Force | Sort-Object -Property FullName -Descending | ForEach-Object { try { Remove-Item -Path $_.FullName -Force -Recurse -ErrorAction SilentlyContinue; } catch { } }

# Clean up Packer's temp scripts and remove the WinRM listener only after Packer has closed its WinRM shell. Removing it inline resets the
# in-flight WinRM connection, which packer-plugin-sdk >= v0.6.3 reports as an error, causing Packer to
# re-run this script against a VM with no listener (https://github.com/hashicorp/packer-plugin-azure/issues/560).
# The provisioner's pause_after in windows-vhd-builder-sig.json must exceed this task's 2-minute wait so it finishes before capture.
$removeListenerTaskName = 'aks-remove-winrm-listener'
$removeListenerScript = {
    $deadline = (Get-Date).AddMinutes(2)
    $shellPids = @(Get-Process -Name winrshost -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Id)
    while ($shellPids.Count -gt 0 -and (Get-Date) -lt $deadline -and (Get-Process -Id $shellPids -ErrorAction SilentlyContinue)) { Start-Sleep -Milliseconds 500 }
    # Packer's own cleanup can't reach the VM once the listener is gone, so remove its uploaded scripts here.
    Remove-Item -Path "$env:SystemRoot\Temp\packer-*", "$env:SystemRoot\Temp\script-*.ps1" -Force -ErrorAction SilentlyContinue
    Remove-Item -Path WSMan:\Localhost\listener\listener* -Recurse
    Unregister-ScheduledTask -TaskName 'aks-remove-winrm-listener' -Confirm:$false
}
$encodedRemoveListenerScript = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($removeListenerScript.ToString()))
$removeListenerAction = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument "-NoProfile -NonInteractive -ExecutionPolicy Bypass -EncodedCommand $encodedRemoveListenerScript"
Register-ScheduledTask -TaskName $removeListenerTaskName -Action $removeListenerAction -User 'NT AUTHORITY\SYSTEM' -RunLevel Highest -Force | Out-Null
Start-ScheduledTask -TaskName $removeListenerTaskName
