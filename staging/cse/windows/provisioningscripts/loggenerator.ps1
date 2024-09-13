# NOTE: We do not log in this script since we do not have log rotation for the generated logs now.

$Global:ClusterConfiguration = ConvertFrom-Json ((Get-Content "c:\k\kubeclusterconfig.json" -ErrorAction Stop) | out-string)
$aksLogFolder="C:\WindowsAzure\Logs\aks"
$isInitializing=$False
$LogPath="c:\k\loggenerator.log"
$isEnableLog=$False

filter Timestamp { "$(Get-Date -Format o): $_" }

function Write-Log ($message) {
    if ($isEnableLog) {
        $message | Timestamp | Tee-Object -FilePath $LogPath -Append
    }
}

if (!(Test-Path $aksLogFolder)) {
    $isInitializing=$True
    Write-Log "Creating $aksLogFolder"
    New-Item -ItemType Directory -Path $aksLogFolder > $null
}

function Create-SymbolLinkFile {
    Param(
        [Parameter(Mandatory = $true)][string]
        $SrcFile,
        [Parameter(Mandatory = $true)][string]
        $DestFile
    )

    if ((Test-Path $SrcFile) -and !(Test-Path $DestFile)) {
        Write-Log "Creating SymbolicLink $DestFile for $SrcFile"
        New-Item -ItemType SymbolicLink -Path $DestFile -Target $SrcFile
    }
}

function Collect-OldLogFiles {
    Param(
        [Parameter(Mandatory = $true)][string]
        $Folder,
        [Parameter(Mandatory = $true)][string]
        $LogFilePattern
    )

    $oldSymbolLinkFiles=Get-ChildItem (Join-Path $aksLogFolder $LogFilePattern)
    $oldSymbolLinkFiles | Foreach-Object {
        Write-Log "Removing $_"
        Remove-Item $_
    }

    $oldLogFiles=Get-ChildItem (Join-Path $Folder $LogFilePattern)
    $oldLogFiles | Foreach-Object {
        $fileName = [IO.Path]::GetFileName($_)
        Create-SymbolLinkFile -SrcFile $_ -DestFile (Join-Path $aksLogFolder $fileName)
    }
}

# Log files in c:\AzureData
$kLogFiles = @(
    "CustomDataSetupScript.log"
)
$kLogFiles | Foreach-Object {
    Create-SymbolLinkFile -SrcFile (Join-Path "C:\AzureData\" $_) -DestFile (Join-Path $aksLogFolder $_)
}

# Log files in c:\k
$kLogFiles = @(
    "azure-vnet.json",
    "azure-vnet-ipam.json",
    "kubeclusterconfig.json",
    "config.toml",
    "bootstrap-config",
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
    "hnsremediator.log",
    "windowslogscleanup.log",
    "windowsnodereset.log",
    "credential-provider-config.yaml",
    "windows-exporter.err.log",
    "windows-exporter.log"
)
$kLogFiles | Foreach-Object {
    Create-SymbolLinkFile -SrcFile (Join-Path "C:\k\" $_) -DestFile (Join-Path $aksLogFolder $_)
}

$nvidiaInstallLogFolder="C:\AzureData\NvidiaInstallLog"
if (Test-Path $nvidiaInstallLogFolder) {
    $logFiles=Get-ChildItem (Join-Path $nvidiaInstallLogFolder *.log)
    $logFiles | Foreach-Object {
        $fileName = [IO.Path]::GetFileName($_)
        Create-SymbolLinkFile -SrcFile (Join-Path $nvidiaInstallLogFolder $fileName) -DestFile (Join-Path $aksLogFolder $fileName)
    }
}

$calicoLogFolder="C:\CalicoWindows\logs\"
if (Test-Path $calicoLogFolder) {
    $calicoLogFiles = @(
        "calico-felix.err.log",
        "calico-felix.log",
        "calico-node.err.log",
        "calico-node.log"
    )
    $calicoLogFiles | Foreach-Object {
        Create-SymbolLinkFile -SrcFile (Join-Path $calicoLogFolder $_) -DestFile (Join-Path $aksLogFolder $_)
    }
    Collect-OldLogFiles -Folder $calicoLogFolder -LogFilePattern calico-felix-*.*.log
    Collect-OldLogFiles -Folder $calicoLogFolder -LogFilePattern calico-node-*.*.log
}

# Misc files
$miscLogFiles = @(
    "C:\k\azurecni\netconf\10-azure.conflist",
    "c:\ProgramData\containerd\root\panic.log",
    "C:\windows\system32\winevt\Logs\Microsoft-AKSGMSAPlugin%4Admin.evtx",
    "C:\windows\system32\winevt\Logs\Microsoft-Windows-Containers-CCG%4Admin.evtx"
)
$miscLogFiles | Foreach-Object {
    $fileName = [IO.Path]::GetFileName($_)
    Create-SymbolLinkFile -SrcFile $_ -DestFile (Join-Path $aksLogFolder $fileName)
}

# Collect old log files
Collect-OldLogFiles -Folder "C:\k\" -LogFilePattern kubeproxy.err-*.*.log
Collect-OldLogFiles -Folder "C:\k\" -LogFilePattern kubelet.err-*.*.log
Collect-OldLogFiles -Folder "C:\k\" -LogFilePattern containerd.err-*.*.log
Collect-OldLogFiles -Folder "C:\k\" -LogFilePattern azure-vnet.log.*
Collect-OldLogFiles -Folder "C:\k\" -LogFilePattern azure-vnet-ipam.log.*
Collect-OldLogFiles -Folder "C:\k\" -LogFilePattern windows-exporter.err-*.*.log

# Collect running containers
$res = Get-Command containerd.exe -ErrorAction SilentlyContinue
if ($res) {
    Write-Log "Generating containerd-info.txt"
    containerd.exe --v > (Join-Path $aksLogFolder "containerd-info.txt")
}

$res = Get-Command ctr.exe -ErrorAction SilentlyContinue
if ($res) {
    Write-Log "Generating containerd-containers.txt"
    ctr.exe -n k8s.io c ls > (Join-Path $aksLogFolder "containerd-containers.txt")

    Write-Log "Generating containerd-tasks.txt"
    ctr.exe -n k8s.io t ls > (Join-Path $aksLogFolder "containerd-tasks.txt")
  
    Write-Log "Generating containerd-snapshot.txt"
    ctr.exe -n k8s.io snapshot ls > (Join-Path $aksLogFolder "containerd-snapshot.txt")
}


$res = Get-Command crictl.exe -ErrorAction SilentlyContinue
if ($res) {
    Write-Log "Genearting cri-containerd-containers.txt"
    crictl.exe ps -a > (Join-Path $aksLogFolder "cri-containerd-containers.txt")

    Write-Log "Generating cri-containerd-pods.txt"
    crictl.exe pods > (Join-Path $aksLogFolder "cri-containerd-pods.txt")

    Write-Log "Generating cri-containerd-images.txt"
    crictl.exe images > (Join-Path $aksLogFolder "cri-containerd-images.txt")
}

# Collect disk usage
Write-Log "Genearting disk-usage.txt"
$diskUsageFile = Join-Path $aksLogFolder ("disk-usage.txt")
Get-CimInstance -Class CIM_LogicalDisk | Select-Object @{Name="Size(GB)";Expression={$_.size/1gb}}, @{Name="Free Space(GB)";Expression={$_.freespace/1gb}}, @{Name="Free (%)";Expression={"{0,6:P0}" -f(($_.freespace/1gb) / ($_.size/1gb))}}, DeviceID, DriveType | Where-Object DriveType -EQ '3' > $diskUsageFile

# Collect available memory
Write-Log "Genearting available-memory.txt"
$availableMemoryFile = Join-Path $aksLogFolder ("available-memory.txt")
Get-Counter '\Memory\Available MBytes' > $availableMemoryFile

# Collect process info
$res = Get-Process containerd-shim-runhcs-v1 -ErrorAction SilentlyContinue
if ($res) {
    Write-Log "Generating process-containerd-shim-runhcs-v1.txt"
    Get-Process containerd-shim-runhcs-v1 > (Join-Path $aksLogFolder "process-containerd-shim-runhcs-v1.txt")
}

$res = Get-Process CExecSvc -ErrorAction SilentlyContinue
if ($res) {
    Write-Log "Generating process-CExecSvc.txt"
    Get-Process CExecSvc > (Join-Path $aksLogFolder "process-CExecSvc.txt")
}

$res = Get-Process vmcompute -ErrorAction SilentlyContinue
if ($res) {
    Write-Log "Generating process-vmcompute.txt"
    Get-Process vmcompute > (Join-Path $aksLogFolder "process-vmcompute.txt")
}


# We only need to generate and upload the logs after the node is provisioned
# WAWindowsAgent will generate and upload the logs every 15 minutes so we do not need to do it again
if ($isInitializing) {
    Write-Log "Start to upload guestvmlogs when initializing"
    $tempWorkFoler = [Io.path]::Combine($env:TEMP, "guestvmlogs")
    try {
        # Create a work folder
        Write-Log "Creating $tempWorkFoler"
        New-Item -ItemType Directory -Path $tempWorkFoler
        cd $tempWorkFoler

        # Generate logs
        Write-Log "Generating guestvmlogs"
        Invoke-Expression(Get-Childitem -Path "C:\WindowsAzure\" -Filter "CollectGuestLogs.exe" -Recurse | sort LastAccessTime -desc | select -first 1).FullName

        # Get the output
        $logFile=(Get-Childitem -Path $tempWorkFoler  -Filter "*.zip").FullName

        # Upload logs
        Write-Log "Start to uploading $logFile"
        C:\AzureData\windows\sendlogs.ps1 -Path $logFile
    } finally {
        if (Test-Path $tempWorkFoler) {
            Write-Log "Removing $tempWorkFoler"
            Remove-Item -Path $tempWorkFoler -Force -Recurse > $null
        }
    }
}