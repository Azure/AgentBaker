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

if ((Test-Path "c:\k\kubectl.exe") -and (Test-Path "c:\k\config")) {
  try {
    Write-Host "Collecting the information of the node and pods by kubectl"
    function kubectl { c:\k\kubectl.exe --kubeconfig c:\k\config $args }

    $versionResult = kubectl version
    if ($LASTEXITCODE -ne 0) {
      throw "Cannot connect to API Server"
    }

    kubectl get nodes -o wide > "$ENV:TEMP\kubectl-get-nodes-$($timeStamp).log"

    $nodeName = $env:COMPUTERNAME.ToLower()
    kubectl describe node $nodeName > "$ENV:TEMP\kubectl-describe-nodes-$($timeStamp).log"

    $podsJson = & crictl.exe pods --output json | ConvertFrom-Json
    foreach ($pod in $podsJson.items) {
      $podName = $pod.metadata.name
      $namespace = $pod.metadata.namespace
      kubectl describe pod $podName -n $namespace >> "$ENV:TEMP\kubectl-describe-pods-$($timeStamp).log"
    }

    $kubectlLogFiles = Get-ChildItem -Path "$ENV:TEMP\kubectl-*.log"
    foreach ($kFile in $kubectlLogFiles) {
      $paths += $kFile.FullName
    }
  }
  catch {
    Write-Host "Failed to run kubectl. Test connection's verion result: $versionResult. Exception: $($_.Exception.Message)"
  }
}

Write-Host "All logs collected: $paths"
Stop-Transcript

Write-Host "Compressing all logs to $zipName"
$paths | Format-Table FullName, Length -AutoSize
Compress-Archive -LiteralPath $paths -DestinationPath $zipName
Get-ChildItem $zipName # this puts a FileInfo on the pipeline so that another script can get it on the pipeline