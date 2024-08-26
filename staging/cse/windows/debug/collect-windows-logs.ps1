param (
  [switch]$enableAll,
  [switch]$enableSnapshotSize,
  [switch]$disableContainerdInfo
)
# param must be at the beginning of the script, add more param if needed

Write-Host "parameters: enableAll=$enableAll, enableSnapshotSize=$enableSnapshotSize, disableContainerdInfo=$disableContainerdInfo"

# NOTE: Please also update staging/cse/windows/provisioningscripts/loggenerator.ps1 when collecting new logs.

# SilentlyContinue mode suppresses errors and continues the script execution.
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
  "hosts-config-agent.log",
  "windows-exporter.err.log",
  "windows-exporter.log"
)

$timeStamp = get-date -format 'yyyyMMdd-hhmmss'
$zipName = "$env:computername-$($timeStamp)_logs.zip"

$paths = @() # All log file paths will be collected 

# Log the script output. It is the first log file to avoid other impact.
$outputLogFile = "$ENV:TEMP\collect-windows-logs-output.log"
Start-Transcript -Path $outputLogFile
$paths += $outputLogFile

Write-Host "Collecting logs for various Kubernetes components"
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


if (Test-Path "c:\k\credential-provider-config.yaml") {
  Write-Host "Collecting credential provider config"
  $paths += "c:\k\credential-provider-config.yaml"
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

# log containerd containers (this is done for docker via networking collectlogs.ps1)
if ($disableContainerdInfo) {
  Write-Host "Skipping collecting containerd info since it costs time in some cases. E.g. .\collect-windows-logs.ps1 --disableContainerdInfo"
} else {
  Write-Host "Collecting Containerd info from ctr"
  $ctrLogsDirectory = "$ENV:TEMP\$timeStamp-ctr-logs"
  $res = Get-Command ctr.exe -ErrorAction SilentlyContinue
  if ($res) {
    New-Item -Type Directory $ctrLogsDirectory

    Write-Host "Collecting ctr plugin ls"
    & ctr.exe -n k8s.io plugin ls > "$ctrLogsDirectory\containerd-plugin.txt"

    Write-Host "Collecting ctr containers"
    & ctr.exe -n k8s.io c ls > "$ctrLogsDirectory\containerd-containers.txt"

    Write-Host "Collecting ctr tasks"
    & ctr.exe -n k8s.io t ls > "$ctrLogsDirectory\containerd-tasks.txt"

    Write-Host "Collecting ctr content ls"
    & ctr.exe -n k8s.io content ls > "$ctrLogsDirectory\containerd-content.txt"

    Write-Host "Collecting ctr image ls"
    & ctr.exe -n k8s.io image ls > "$ctrLogsDirectory\containerd-image.txt"

    Write-Host "Collecting ctr snapshot ls"
    & ctr.exe -n k8s.io snapshot ls > "$ctrLogsDirectory\containerd-snapshot.txt"

    Write-Host "Collecting ctr snapshot tree"
    & ctr.exe -n k8s.io snapshot tree > "$ctrLogsDirectory\containerd-snapshot-tree.txt"

    Write-Host "Collecting ctr snapshot info for each snapshot"
    $snapshotsList = (& ctr.exe -n k8s.io snapshot ls)
    foreach ($snapshot in $snapshotsList) {
      $snapshotId = ($snapshot.Split(" ")[0])
      $fileName = ($snapshotId.Split(":")[1])
      if ($fileName.length -gt 0) {
        & ctr.exe -n k8s.io snapshot info $snapshotId > "$ctrLogsDirectory\containerd-snapshot-info-$fileName.txt"
      }
    }
    $paths += $ctrLogsDirectory
  }
  else {
    Write-Host "ctr.exe command not available"
  }
}

# Collect extensions logs
if (Test-Path "C:\WindowsAzure\Logs\Plugins") {
  $pluginLogsTempFolder="$ENV:TEMP\Extension-Logs-$timeStamp"
  New-Item -ItemType Directory -Path $pluginLogsTempFolder > $null

  Copy-Item -Recurse "C:\WindowsAzure\Logs\Plugins\*" $pluginLogsTempFolder -Passthru -ErrorAction Ignore

  $tempFile=(CollectLogsFromDirectory -path $pluginLogsTempFolder -targetFileName "Extension-Logs.zip")
  if ($tempFile -ne "") {
    $paths += $tempFile
  }
}

Write-Host "All logs collected: $paths"
Stop-Transcript

Write-Host "Compressing all logs to $zipName"
$paths | Format-Table FullName, Length -AutoSize
Compress-Archive -LiteralPath $paths -DestinationPath $zipName
Get-ChildItem $zipName # this puts a FileInfo on the pipeline so that another script can get it on the pipeline