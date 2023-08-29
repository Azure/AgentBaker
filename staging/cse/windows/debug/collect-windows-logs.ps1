# NOTE: Please also update staging/cse/windows/provisioningscripts/loggenerator.ps1 when collecting new logs.
$ProgressPreference = "SilentlyContinue"

function CollectLogsFromDirectory {
  Param(
      [Parameter(Mandatory=$true)]
      [String] $path,
      [Parameter(Mandatory=$true)]
      [String] $targetFileName
  )
  try {
    $tempFile="$ENV:TEMP\$targetFileName"
    if (Test-Path $path) {
      Write-Host "Collecting logs from $path"
      Compress-Archive -LiteralPath $path -DestinationPath $tempFile
      # Compress-Archive will not generate any target file if the source directory is empty
      if (Test-Path $tempFile) {
        return $tempFile
      }
      Write-Host "Ignore since there is no log in $path"
    } else {
      Write-Host "Path $path does not exist"
    }
  } catch {
    Write-Host "Failed to collect logs from $path"
  }
  return ""
}

$lockedFiles = @(
  "kubelet.err.log",
  "kubelet.log",
  "kubeproxy.log",
  "kubeproxy.err.log",
  "azure-vnet-telemetry.log",
  "azure-vnet.log",
  "csi-proxy.log",
  "csi-proxy.err.log",
  "containerd.log",
  "containerd.err.log",
  "hosts-config-agent.err.log",
  "hosts-config-agent.log"
)

$timeStamp = get-date -format 'yyyyMMdd-hhmmss'
$zipName = "$env:computername-$($timeStamp)_logs.zip"

Write-Host "Collecting logs for various Kubernetes components"
$paths = @()
get-childitem c:\k\*.log* -Exclude $lockedFiles | Foreach-Object {
  $paths += $_
}
$lockedTemp = Join-Path ([System.IO.Path]::GetTempPath()) ([System.IO.Path]::GetRandomFileName())
New-Item -Type Directory $lockedTemp
$lockedFiles | Foreach-Object {
  Write-Host "Copying $_ to temp"
  $src = "c:\k\$_"
  if (Test-Path $src) {
    $tempfile = Copy-Item $src $lockedTemp -Passthru -ErrorAction Ignore
    if ($tempFile) {
      $paths += $tempFile
    }
  }
}

Write-Host "Collecting kubeclusterconfig"
$paths += "c:\k\kubeclusterconfig.json"

if (Test-Path "c:\k\bootstrap-config") {
  Write-Host "Collecting bootstrap-config"
  $paths += "c:\k\bootstrap-config"
}

Write-Host "Collecting Azure CNI configurations"
$paths += "C:\k\azurecni\netconf\10-azure.conflist"
$azureCNIConfigurations = @(
  "azure-vnet.json",
  "azure-vnet-ipam.json"
)
$azureCNIConfigurations | Foreach-Object {
  Write-Host "Copying $_ to temp"
  $src = "c:\k\$_"
  if (Test-Path $src) {
    $tempfile = Copy-Item $src $lockedTemp -Passthru -ErrorAction Ignore
    if ($tempFile) {
      $paths += $tempFile
    }
  }
}

# azure-cni logs currently end up in system32 when called by containerd so check there for logs too
$lockedTemp = Join-Path ([System.IO.Path]::GetTempPath()) ([System.IO.Path]::GetRandomFileName())
New-Item -Type Directory $lockedTemp
$lockedFiles | Foreach-Object {
  Write-Host "Copying $_ to temp"
  $src = "c:\windows\system32\$_"
  if (Test-Path $src) {
    $tempfile = Copy-Item $src $lockedTemp -Passthru -ErrorAction Ignore
    if ($tempFile) {
      $paths += $tempFile
    }
  }
}

Write-Host "Exporting ETW events to CSV files"
$scm = Get-WinEvent -FilterHashtable @{logname = 'System'; ProviderName = 'Service Control Manager' } | Where-Object { $_.Message -Like "*containerd*" -or $_.Message -Like "*kub*" } | Select-Object -Property TimeCreated, Id, LevelDisplayName, Message
# 2004 = resource exhaustion, other 5 events related to reboots
$reboots = Get-WinEvent -ErrorAction Ignore -FilterHashtable @{logname = 'System'; id = 1074, 1076, 2004, 6005, 6006, 6008 } | Select-Object -Property TimeCreated, Id, LevelDisplayName, Message
$crashes = Get-WinEvent -ErrorAction Ignore -FilterHashtable @{logname = 'Application'; ProviderName = 'Windows Error Reporting' } | Select-Object -Property TimeCreated, Id, LevelDisplayName, Message
$scm + $reboots + $crashes | Sort-Object TimeCreated | Export-CSV -Path "$ENV:TEMP\\$($timeStamp)_services.csv"
$paths += "$ENV:TEMP\\$($timeStamp)_services.csv"
Get-WinEvent -LogName Microsoft-Windows-Hyper-V-Compute-Operational | Select-Object -Property TimeCreated, Id, LevelDisplayName, Message | Sort-Object TimeCreated | Export-Csv -Path "$ENV:TEMP\\$($timeStamp)_hyper-v-compute-operational.csv"
$paths += "$ENV:TEMP\\$($timeStamp)_hyper-v-compute-operational.csv"

Write-Host "Collecting gMSAv2 related logs"
# CCGPlugin (Windows gMSAv2)
$EventSession = [System.Diagnostics.Eventing.Reader.EventLogSession]::GlobalSession
$EventProviderNames = $EventSession.GetProviderNames()
if ($EventProviderNames -contains "Microsoft-Windows-Containers-CCG") {
  if (Test-Path "C:\\windows\\system32\\winevt\\Logs\\Microsoft-Windows-Containers-CCG%4Admin.evtx" -PathType Leaf) {
    cp "C:\\windows\\system32\\winevt\\Logs\\Microsoft-Windows-Containers-CCG%4Admin.evtx" "$ENV:TEMP\\Microsoft-Windows-Containers-CCG%4Admin.evtx"
    $paths += "$ENV:TEMP\\Microsoft-Windows-Containers-CCG%4Admin.evtx"
  }
  else {
    Write-Host "Microsoft-Windows-Containers-CCG%4Admin.evtx does not exist"
  }
}
else {
  Write-Host "Microsoft-Windows-Containers-CCG events are not available"
}
# Introduced from CCGAKVPlugin v1.1.3
if ($EventProviderNames -contains "Microsoft-AKSGMSAPlugin") {
  if (Test-Path "C:\\windows\\system32\\winevt\\Logs\\Microsoft-AKSGMSAPlugin%4Admin.evtx" -PathType Leaf) {
    cp "C:\\windows\\system32\\winevt\\Logs\\Microsoft-AKSGMSAPlugin%4Admin.evtx" "$ENV:TEMP\\Microsoft-AKSGMSAPlugin%4Admin.evtx"
    $paths += "$ENV:TEMP\\Microsoft-AKSGMSAPlugin%4Admin.evtx"
  }
  else {
    Write-Host "Microsoft-AKSGMSAPlugin%4Admin.evtx does not exist"
  }
}
else {
  Write-Host "AKSGMSAPlugin events are not available"
}

Get-CimInstance win32_pagefileusage | Format-List * | Out-File -Append "$ENV:TEMP\\$($timeStamp)_pagefile.txt"
Get-CimInstance win32_computersystem | Format-List AutomaticManagedPagefile | Out-File -Append "$ENV:TEMP\\$($timeStamp)_pagefile.txt"
$paths += "$ENV:TEMP\\$($timeStamp)_pagefile.txt"

Write-Host "Collecting networking related logs"
& 'c:\k\debug\collectlogs.ps1' | write-Host
$netLogs = Get-ChildItem (Get-ChildItem -Path c:\k\debug -Directory | Sort-Object LastWriteTime -Descending | Select-Object -First 1).FullName | Select-Object -ExpandProperty FullName
$paths += $netLogs
$paths += "c:\AzureData\CustomDataSetupScript.log"

# log containerd containers (this is done for docker via networking collectlogs.ps1)
Write-Host "Collecting Containerd running containers"
$res = Get-Command ctr.exe -ErrorAction SilentlyContinue
if ($res) {
  Write-Host "Collecting Containerd running containers - containers"
  & ctr.exe -n k8s.io c ls > "$ENV:TEMP\$timeStamp-containerd-containers.txt"
  $paths += "$ENV:TEMP\$timeStamp-containerd-containers.txt"

  Write-Host "Collecting Containerd running containers - tasks"
  & ctr.exe -n k8s.io t ls > "$ENV:TEMP\$timeStamp-containerd-tasks.txt"
  $paths += "$ENV:TEMP\$timeStamp-containerd-tasks.txt"

  Write-Host "Collecting Containerd running containers - snapshot"
  & ctr.exe -n k8s.io snapshot ls > "$ENV:TEMP\$timeStamp-containerd-snapshot.txt"
  $paths += "$ENV:TEMP\$timeStamp-containerd-snapshot.txt"
}
else {
  Write-Host "ctr.exe command not available"
}

# log containers, pods and images the CRI plugin is aware of, and their state.
$res = Get-Command crictl.exe -ErrorAction SilentlyContinue
if ($res) {
  Write-Host "Collecting CRI plugin containers"
  & crictl.exe ps -a > "$ENV:TEMP\$timeStamp-cri-containerd-containers.txt"
  $paths += "$ENV:TEMP\$timeStamp-cri-containerd-containers.txt"

  Write-Host "Collecting CRI plugin pods"
  & crictl.exe pods > "$ENV:TEMP\$timeStamp-cri-containerd-pods.txt"
  $paths += "$ENV:TEMP\$timeStamp-cri-containerd-pods.txt"

  Write-Host "Collecting CRI plugin images"
  & crictl.exe images > "$ENV:TEMP\$timeStamp-cri-containerd-images.txt"
  $paths += "$ENV:TEMP\$timeStamp-cri-containerd-images.txt"
}
else {
  Write-Host "crictl.exe command not available"
}

# use runhcs shim diagnostic tool 
$res = Get-Command shimdiag.exe -ErrorAction SilentlyContinue
if ($res) {
  Write-Host "Collecting logs of runhcs shim diagnostic tool"
  $tempShimdiagFile = Join-Path ([System.IO.Path]::GetTempPath()) ("shimdiag.txt")
  $shimdiagList = shimdiag.exe list
  Set-Content -Path $tempShimdiagFile -Value $shimdiagList
  foreach ($line in $shimdiagList) {
    $tempResult = shimdiag.exe stacks $line
    Add-Content -Path $tempShimdiagFile -Value ""
    Add-Content -Path $tempShimdiagFile -Value $line
    Add-Content -Path $tempShimdiagFile -Value $tempResult
  }
  $paths += $tempShimdiagFile
}
else {
  Write-Host "shimdiag.exe command not available"
}

# log containerd info
$res = Get-Command containerd.exe -ErrorAction SilentlyContinue
if ($res) {
  Write-Host "Collecting logs of containerd info"
  & containerd.exe --v > "$ENV:TEMP\$timeStamp-containerd-info.txt"
  $paths += "$ENV:TEMP\$timeStamp-containerd-info.txt"
}
else {
  Write-Host "containerd.exe command not available"
}

# Containerd panic log is outside the c:\k folder
Write-Host "Collecting containerd panic logs"
$containerdPanicLog = "c:\ProgramData\containerd\root\panic.log"
if (Test-Path $containerdPanicLog) {
  $tempfile = Copy-Item $containerdPanicLog $lockedTemp -Passthru -ErrorAction Ignore
  if ($tempFile) {
    $paths += $tempFile
  }
}
else {
  Write-Host "Containerd panic logs not available"
}

Write-Host "Collecting containerd configuration"
$containerdConfig = "$Env:ProgramFiles\containerd\config.toml"
if (Test-Path $containerdConfig) {
  $tempfile = Copy-Item $containerdConfig $lockedTemp -Passthru -ErrorAction Ignore
  if ($tempFile) {
    $paths += $tempFile
  }
}

Write-Host "Collecting calico logs"
if (Test-Path "c:\CalicoWindows\logs") {
  $tempCalico = Join-Path ([System.IO.Path]::GetTempPath()) ([System.IO.Path]::GetRandomFileName())
  New-Item -Type Directory $tempCalico
  Get-ChildItem c:\CalicoWindows\logs\*.log* | Foreach-Object {
    Write-Host "Copying $_ to temp"
    $tempfile = Copy-Item $_ $tempCalico -Passthru -ErrorAction Ignore
    if ($tempFile) {
      $paths += $tempFile
    }
  }
}
else {
  Write-Host "Calico logs not available"
}

Write-Host "Collecting disk usage"
$tempDiskUsageFile = Join-Path ([System.IO.Path]::GetTempPath()) ("disk-usage.txt")
Get-CimInstance -Class CIM_LogicalDisk | Select-Object @{Name="Size(GB)";Expression={$_.size/1gb}}, @{Name="Free Space(GB)";Expression={$_.freespace/1gb}}, @{Name="Free (%)";Expression={"{0,6:P0}" -f(($_.freespace/1gb) / ($_.size/1gb))}}, DeviceID, DriveType | Where-Object DriveType -EQ '3' > $tempDiskUsageFile
$paths += $tempDiskUsageFile

# Collect process info
$rest = Get-Process containerd-shim-runhcs-v1 -ErrorAction SilentlyContinue
if ($res) {
  Write-Host "Collecting process info for containerd-shim-runhcs-v1"
  Get-Process containerd-shim-runhcs-v1 > "$ENV:TEMP\process-containerd-shim-runhcs-v1.txt"
  $paths += "$ENV:TEMP\process-containerd-shim-runhcs-v1.txt"
}
else {
  Write-Host "containerd-shim-runhcs-v1 process not available"
}

$res = Get-Process CExecSvc -ErrorAction SilentlyContinue
if ($res) {
  Write-Host "Collecting process info for CExecSvc"
  Get-Process CExecSvc > "$ENV:TEMP\process-CExecSvc.txt"
  $paths += "$ENV:TEMP\process-CExecSvc.txt"
}
else {
  Write-Host "CExecSvc process not available"
}

$res = Get-Process vmcompute -ErrorAction SilentlyContinue
if ($res) {
  Write-Host "Collecting process info for vmcompute"
  Get-Process vmcompute > "$ENV:TEMP\process-vmcompute.txt"
  $paths += "$ENV:TEMP\process-vmcompute.txt"
}
else {
  Write-Host "vmcompute process not availabel"
}

# Collect dump files
$tempFile=(CollectLogsFromDirectory -path "C:\ProgramData\Microsoft\Windows\WER" -targetFileName "WER-$($timeStamp).zip")
if ($tempFile -ne "") {
  $paths += $tempFile
}
$tempFile=(CollectLogsFromDirectory -path "C:\Windows\Minidump" -targetFileName "Minidump-$($timeStamp).zip")
if ($tempFile -ne "") {
  $paths += $tempFile
}
$tempFile=(CollectLogsFromDirectory -path "C:\Windows\SystemTemp" -targetFileName "SystemTemp-$($timeStamp).zip")
if ($tempFile -ne "") {
  $paths += $tempFile
}

Write-Host "Compressing all logs to $zipName"
$paths | Format-Table FullName, Length -AutoSize
Compress-Archive -LiteralPath $paths -DestinationPath $zipName
Get-ChildItem $zipName # this puts a FileInfo on the pipeline so that another script can get it on the pipeline