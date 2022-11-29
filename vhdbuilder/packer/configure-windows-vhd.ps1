<#
    .SYNOPSIS
        Used to produce Windows AKS images.

    .DESCRIPTION
        This script is used by packer to produce Windows AKS images.
#>

$ErrorActionPreference = "Stop"

. c:\windows-vhd-configuration.ps1

filter Timestamp { "$(Get-Date -Format o): $_" }

function Write-Log($Message) {
    $msg = $message | Timestamp
    Write-Output $msg
}

function DownloadFileWithRetry {
    param (
        $URL,
        $Dest,
        $retryCount = 5,
        $retryDelay = 0,
        [Switch]$redactUrl = $false
    )
    curl.exe -f --retry $retryCount --retry-delay $retryDelay -L $URL -o $Dest
    if ($LASTEXITCODE) {
        $logURL = $URL
        if ($redactUrl) {
            $logURL = $logURL.Split("?")[0]
        }
        throw "Curl exited with '$LASTEXITCODE' while attemping to download '$logURL'"
    }
}

function Retry-Command {
    [CmdletBinding()]
    Param(
        [Parameter(Position=0, Mandatory=$true)]
        [scriptblock]$ScriptBlock,

        [Parameter(Position=1, Mandatory=$true)]
        [string]$ErrorMessage,

        [Parameter(Position=2, Mandatory=$false)]
        [int]$Maximum = 5,

        [Parameter(Position=3, Mandatory=$false)]
        [int]$Delay = 10
    )

    Begin {
        $cnt = 0
    }

    Process {
        do {
            $cnt++
            try {
                $ScriptBlock.Invoke()
                if ($LASTEXITCODE) {
                    throw "Retry $cnt : $ErrorMessage"
                }
                return
            } catch {
                Write-Error $_.Exception.InnerException.Message -ErrorAction Continue
                if ($_.Exception.InnerException.Message.Contains("There is not enough space on the disk. (0x70)")) {
                    Write-Error "Exit retry since there is not enough space on the disk"
                    break
                }
                Start-Sleep $Delay
            }
        } while ($cnt -lt $Maximum)

        # Throw an error after $Maximum unsuccessful invocations. Doesn't need
        # a condition, since the function returns upon successful invocation.
        throw 'All retries failed. $ErrorMessage'
    }
}

function Invoke-Executable {
    Param(
        [string]
        $Executable,
        [string[]]
        $ArgList,
        [int]
        $Retries = 0,
        [int]
        $RetryDelaySeconds = 1
    )

    for ($i = 0; $i -le $Retries; $i++) {
        Write-Log "$i - Running $Executable $ArgList ..."
        & $Executable $ArgList
        if ($LASTEXITCODE) {
            Write-Log "$Executable returned unsuccessfully with exit code $LASTEXITCODE"
            Start-Sleep -Seconds $RetryDelaySeconds
            continue
        }
        else {
            Write-Log "$Executable returned successfully"
            return
        }
    }

    Write-Log "Exhausted retries for $Executable $ArgList"
    exit 1
}

function Expand-OS-Partition {
    $customizedDiskSize = $env:CustomizedDiskSize
    if ([string]::IsNullOrEmpty($customizedDiskSize)) {
        Write-Log "No need to expand the OS partition size"
        return
    }

    Write-Log "Customized OS disk size is $customizedDiskSize GB"
    [Int32]$osPartitionSize = 0
    if ([Int32]::TryParse($customizedDiskSize, [ref]$osPartitionSize)) {
        # The supportedMaxSize less than the customizedDiskSize because some system usages will occupy disks (about 500M).
        $supportedMaxSize = (Get-PartitionSupportedSize -DriveLetter C).sizeMax
        $currentSize = (Get-Partition -DriveLetter C).Size
        if ($supportedMaxSize -gt $currentSize) {
            Write-Log "Resizing the OS partition size from $currentSize to $supportedMaxSize"
            Resize-Partition -DriveLetter C -Size $supportedMaxSize
            Get-Disk
            Get-Partition
        } else {
            Write-Log "The current size is the max size $currentSize"
        }
    } else {
        Throw "$customizedDiskSize is not a valid customized OS disk size"
    }
}

function Disable-WindowsUpdates {
    # See https://docs.microsoft.com/en-us/windows/deployment/update/waas-wu-settings
    # for additional information on WU related registry settings

    Write-Log "Disabling automatic windows upates"
    $WindowsUpdatePath = "HKLM:SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate"
    $AutoUpdatePath = "HKLM:SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate\AU"

    if (Test-Path -Path $WindowsUpdatePath) {
        Remove-Item -Path $WindowsUpdatePath -Recurse
    }

    New-Item -Path $WindowsUpdatePath | Out-Null
    New-Item -Path $AutoUpdatePath | Out-Null
    Set-ItemProperty -Path $AutoUpdatePath -Name NoAutoUpdate -Value 1 | Out-Null
}

function Get-ContainerImages {
    if ($containerRuntime -eq 'containerd') {
        Write-Log "Pulling images for windows server $windowsSKU" # The variable $windowsSKU will be "2019-containerd", "2022-containerd", ...
        foreach ($image in $imagesToPull) {
            if (($image.Contains("mcr.microsoft.com/windows/servercore") -and ![string]::IsNullOrEmpty($env:WindowsServerCoreImageURL)) -or
                ($image.Contains("mcr.microsoft.com/windows/nanoserver") -and ![string]::IsNullOrEmpty($env:WindowsNanoServerImageURL))) {
                $url=""
                if ($image.Contains("mcr.microsoft.com/windows/servercore")) {
                    $url=$env:WindowsServerCoreImageURL
                } elseif ($image.Contains("mcr.microsoft.com/windows/nanoserver")) {
                    $url=$env:WindowsNanoServerImageURL
                }
                $fileName = [IO.Path]::GetFileName($url.Split("?")[0])
                $tmpDest = [IO.Path]::Combine([System.IO.Path]::GetTempPath(), $fileName)
                Write-Log "Downloading image $image to $tmpDest"
                DownloadFileWithRetry -URL $url -Dest $tmpDest -redactUrl

                Write-Log "Loading image $image from $tmpDest"
                Retry-Command -ScriptBlock {
                    & ctr -n k8s.io images import $tmpDest
                } -ErrorMessage "Failed to load image $image from $tmpDest"

                Write-Log "Removing tmp tar file $tmpDest"
                Remove-Item -Path $tmpDest
            } else {
                Write-Log "Pulling image $image"
                Retry-Command -ScriptBlock {
                    & crictl.exe pull $image
                } -ErrorMessage "Failed to pull image $image"
            }
        }
        Stop-Job  -Name containerd
        Remove-Job -Name containerd
    }
    else {
        Write-Log "Pulling images for windows server 2019 with docker"
        foreach ($image in $imagesToPull) {
            if (($image.Contains("mcr.microsoft.com/windows/servercore") -and ![string]::IsNullOrEmpty($env:WindowsServerCoreImageURL)) -or
                ($image.Contains("mcr.microsoft.com/windows/nanoserver") -and ![string]::IsNullOrEmpty($env:WindowsNanoServerImageURL))) {
                $url=""
                if ($image.Contains("mcr.microsoft.com/windows/servercore")) {
                    $url=$env:WindowsServerCoreImageURL
                } elseif ($image.Contains("mcr.microsoft.com/windows/nanoserver")) {
                    $url=$env:WindowsNanoServerImageURL
                }
                $fileName = [IO.Path]::GetFileName($url.Split("?")[0])
                $tmpDest = [IO.Path]::Combine([System.IO.Path]::GetTempPath(), $fileName)
                Write-Log "Downloading image $image to $tmpDest"
                DownloadFileWithRetry -URL $url -Dest $tmpDest -redactUrl

                Write-Log "Loading image $image from $tmpDest"
                Retry-Command -ScriptBlock {
                    & docker load -i $tmpDest
                } -ErrorMessage "Failed to load image $image from $tmpDest"

                Write-Log "Removing tmp tar file $tmpDest"
                Remove-Item -Path $tmpDest
            } else {
                Write-Log "Pulling image $image"
                Retry-Command -ScriptBlock {
                    docker pull $image
                } -ErrorMessage "Failed to pull image $image"
            }
        }
    }
}

function Get-FilesToCacheOnVHD {
    Write-Log "Caching misc files on VHD, container runtimne: $containerRuntime"

    foreach ($dir in $map.Keys) {
        New-Item -ItemType Directory $dir -Force | Out-Null

        foreach ($URL in $map[$dir]) {
            $fileName = [IO.Path]::GetFileName($URL)
            # Do not cache containerd package on docker VHD
            if ($containerRuntime -ne 'containerd' -And $dir -eq "c:\akse-cache\containerd\") {
                Write-Log "Skip to download $URL for docker VHD"
                continue
            }

            # Windows containerD supports Windows containerD, starting from Kubernetes 1.20
            if ($containerRuntime -eq 'containerd' -And $dir -eq "c:\akse-cache\win-k8s\") {
                $k8sMajorVersion = $fileName.split(".",3)[0]
                $k8sMinorVersion = $fileName.split(".",3)[1]
                if ($k8sMinorVersion -lt "20" -And $k8sMajorVersion -eq "v1") {
                    Write-Log "Skip to download $URL for containerD is supported from Kubernets 1.20"
                    continue
                }
            }

            $dest = [IO.Path]::Combine($dir, $fileName)

            Write-Log "Downloading $URL to $dest"
            DownloadFileWithRetry -URL $URL -Dest $dest
        }
    }
}

function Install-ContainerD {
    # installing containerd during VHD building is to cache container images into the VHD,
    # and the containerd to managed customer containers after provisioning the vm is not necessary
    # the one used here, considering containerd version/package is configurable, and the first one
    # is expected to override the later one
    Write-Log "Getting containerD binaries from $global:defaultContainerdPackageUrl"

    $installDir = "c:\program files\containerd"
    Write-Log "Installing containerd to $installDir"
    New-Item -ItemType Directory $installDir -Force | Out-Null

    $containerdFilename=[IO.Path]::GetFileName($global:defaultContainerdPackageUrl)
    $containerdTmpDest = [IO.Path]::Combine($installDir, $containerdFilename)
    DownloadFileWithRetry -URL $global:defaultContainerdPackageUrl -Dest $containerdTmpDest
    # The released containerd package format is either zip or tar.gz
    if ($containerdFilename.endswith(".zip")) {
        Expand-Archive -path $containerdTmpDest -DestinationPath $installDir -Force
    } else {
        tar -xzf $containerdTmpDest --strip=1 -C $installDir
        mv -Force $installDir\bin\* $installDir
        Remove-Item -Path $installDir\bin -Force -Recurse
    }
    Remove-Item -Path $containerdTmpDest | Out-Null

    $newPaths = [Environment]::GetEnvironmentVariable("Path", [EnvironmentVariableTarget]::Machine) + ";$installDir"
    [Environment]::SetEnvironmentVariable("Path", $newPaths, [EnvironmentVariableTarget]::Machine)
    $env:Path += ";$installDir"

    $containerdConfigPath = [Io.Path]::Combine($installDir, "config.toml")
    # enabling discard_unpacked_layers allows GC to remove layers from the content store after
    # successfully unpacking these layers to the snapshotter to reduce the disk space caching Windows containerd images
    (containerd config default)  | %{$_ -replace "discard_unpacked_layers = false", "discard_unpacked_layers = true"}  | Out-File  -FilePath $containerdConfigPath -Encoding ascii

    Get-Content $containerdConfigPath

    # start containerd to pre-pull the images to disk on VHD
    # CSE will configure and register containerd as a service at deployment time
    Start-Job -Name containerd -ScriptBlock { containerd.exe }
}

function Install-Docker {
    Write-Log "Attempting to install Docker version $defaultDockerVersion"
    Install-PackageProvider -Name DockerMsftProvider -Force -ForceBootstrap | Out-null
    $package = Find-Package -Name Docker -ProviderName DockerMsftProvider -RequiredVersion $defaultDockerVersion
    Write-Log "Installing Docker version $($package.Version)"
    $package | Install-Package -Force | Out-Null

    if ($defaultDockerVersion -eq "20.10.9"){
        # We only do this for docker 20.10.9 so we do not need to add below code in Install-Docker in configfunc.ps1 because
        # 1. the cat file is installed in building WS2019+docker
        # 2. it does not need to run below code if a newer docker version is used in CSE later
        Write-Log "Downloading cat for docker 20.10.9"
        DownloadFileWithRetry -URL "https://dockermsft.azureedge.net/dockercontainer/docker-20-10-9.cat" -Dest "C:\Windows\System32\CatRoot\{F750E6C3-38EE-11D1-85E5-00C04FC295EE}\docker-20-10-9.cat"
    }

    Start-Service docker
}


function Install-OpenSSH {
    Write-Log "Installing OpenSSH Server"
    Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0
}

function Install-WindowsPatches {
    Write-Log "Installing Windows patches"
    Write-Log "The length of patchUrls is $($patchUrls.Length)"
    foreach ($patchUrl in $patchUrls) {
        $pathOnly = $patchUrl.Split("?")[0]
        $fileName = Split-Path $pathOnly -Leaf
        $fileExtension = [IO.Path]::GetExtension($fileName)
        $fullPath = [IO.Path]::Combine($env:TEMP, $fileName)

        switch ($fileExtension) {
            ".msu" {
                Write-Log "Downloading windows patch from $pathOnly to $fullPath"
                DownloadFileWithRetry -URL $patchUrl -Dest $fullPath -redactUrl
                Write-Log "Starting install of $fileName"
                $proc = Start-Process -Passthru -FilePath wusa.exe -ArgumentList "$fullPath /quiet /norestart"
                Wait-Process -InputObject $proc
                switch ($proc.ExitCode) {
                    0 {
                        Write-Log "Finished install of $fileName"
                    }
                    3010 {
                        WRite-Log "Finished install of $fileName. Reboot required"
                    }
                    default {
                        Write-Log "Error during install of $fileName. ExitCode: $($proc.ExitCode)"
                        exit 1
                    }
                }
            }
            default {
                Write-Log "Installing patches with extension $fileExtension is not currently supported."
                exit 1
            }
        }
    }
}

function Set-WinRmServiceAutoStart {
    Write-Log "Setting WinRM service start to auto"
    sc.exe config winrm start=auto
}

function Set-WinRmServiceDelayedStart {
    # Hyper-V messes with networking components on startup after the feature is enabled
    # causing issues with communication over winrm and setting winrm to delayed start
    # gives Hyper-V enough time to finish configuration before having packer continue.
    Write-Log "Setting WinRM service start to delayed-auto"
    sc.exe config winrm start=delayed-auto
}

# Best effort to update defender signatures
# This can fail if there is already a signature
# update running which means we will get them anyways
# Also at the time the VM is provisioned Defender will trigger any required updates
function Update-DefenderSignatures {
    Write-Log "Updating windows defender signatures."
    $service = Get-Service "Windefend"
    $service.WaitForStatus("Running","00:5:00")
    Update-MpSignature
}

function Update-WindowsFeatures {
    $featuresToEnable = @(
        "Containers",
        "Hyper-V",
        "Hyper-V-PowerShell")

    foreach ($feature in $featuresToEnable) {
        Write-Log "Enabling Windows feature: $feature"
        Install-WindowsFeature $feature
    }
}

function Update-Registry {
    # Enables DNS resolution of SMB shares for containerD
    # https://github.com/kubernetes-sigs/windows-gmsa/issues/30#issuecomment-802240945
    if ($containerRuntime -eq 'containerd') {
        Write-Log "Apply SMB Resolution Fix for containerD"
        Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name EnableCompartmentNamespace -Value 1 -Type DWORD
    }

    if ($env:WindowsSKU -Like '2019*') {
        Write-Log "Enable a HNS fix (0x40) in 2022-11B and another HNS fix (0x10)"
        $hnsControlFlag=0x50
        $currentValue=(Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name HNSControlFlag -ErrorAction Ignore)
        if (![string]::IsNullOrEmpty($currentValue)) {
            Write-Log "The current value of HNSControlFlag is $currentValue"
            $hnsControlFlag=([int]$currentValue.HNSControlFlag -bor $hnsControlFlag)
        }
        Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name HNSControlFlag -Value $hnsControlFlag -Type DWORD

        Write-Log "Enable a WCIFS fix in 2022-10B"
        $currentValue=(Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\wcifs" -Name WcifsSOPCountDisabled -ErrorAction Ignore)
        if (![string]::IsNullOrEmpty($currentValue)) {
            Write-Log "The current value of WcifsSOPCountDisabled is $currentValue"
        }
        Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\wcifs" -Name WcifsSOPCountDisabled -Value 0 -Type DWORD
    }

    if ($env:WindowsSKU -Like '2022*') {
        Write-Log "Enable a WCIFS fix in 2022-10B"
        $regPath=(Get-Item -Path "HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft" -ErrorAction Ignore)
        if (!$regPath) {
            Write-Log "Creating HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft"
            New-Item -Path "HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft"
        }

        $regPath=(Get-Item -Path "HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft\FeatureManagement" -ErrorAction Ignore)
        if (!$regPath) {
            Write-Log "Creating HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft\FeatureManagement"
            New-Item -Path "HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft\FeatureManagement"
        }

        $regPath=(Get-Item -Path "HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft\FeatureManagement\Overrides" -ErrorAction Ignore)
        if (!$regPath) {
            Write-Log "Creating HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft\FeatureManagement\Overrides"
            New-Item -Path "HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft\FeatureManagement\Overrides"
        }

        $currentValue=(Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft\FeatureManagement\Overrides" -Name 2629306509 -ErrorAction Ignore)
        if (![string]::IsNullOrEmpty($currentValue)) {
            Write-Log "The current value of 2629306509 is $currentValue"
        }
        Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft\FeatureManagement\Overrides" -Name 2629306509 -Value 1 -Type DWORD
    }
}

function Get-SystemDriveDiskInfo {
    Write-Log "Get Disk info"
    $disksInfo=Get-CimInstance -ClassName Win32_LogicalDisk
    foreach($disk in $disksInfo) {
        if ($disk.DeviceID -eq "C:") {
            Write-Log "Disk C: Free space: $($disk.FreeSpace), Total size: $($disk.Size)"
        }
    }
}

function Get-DefenderPreferenceInfo {
    Write-Log "Get preferences for the Windows Defender scans and updates"
    Write-Log(Get-MpPreference | Format-List | Out-String)
}

function Exclude-ReservedUDPSourcePort()
{
    # https://docs.microsoft.com/en-us/azure/virtual-network/virtual-networks-faq#what-protocols-can-i-use-within-vnets
    # Default UDP Dynamic Port Range in Windows server: Start Port: 49152, Number of Ports : 16384. Range: [49152, 65535]
    # Exclude UDP source port 65330. This only excludes the port in AKS Windows nodes but will not impact Windows containers.
    # Reference: https://github.com/Azure/AKS/issues/2988
    # List command: netsh int ipv4 show excludedportrange udp
    Invoke-Executable -Executable "netsh.exe" -ArgList @("int", "ipv4", "add", "excludedportrange", "udp", "65330", "1", "persistent")
}

# Disable progress writers for this session to greatly speed up operations such as Invoke-WebRequest
$ProgressPreference = 'SilentlyContinue'

try{
    switch ($env:ProvisioningPhase) {
        "1" {
            Write-Log "Performing actions for provisioning phase 1"
            Expand-OS-Partition
            Exclude-ReservedUDPSourcePort
            Disable-WindowsUpdates
            Set-WinRmServiceDelayedStart
            Update-DefenderSignatures
            Install-WindowsPatches
            Install-OpenSSH
            Update-WindowsFeatures
        }
        "2" {
            Write-Log "Performing actions for provisioning phase 2 for container runtime '$containerRuntime'"
            Set-WinRmServiceAutoStart
            if ($containerRuntime -eq 'containerd') {
                Install-ContainerD
            } else {
                Install-Docker
            }
            Update-Registry
            Get-ContainerImages
            Get-FilesToCacheOnVHD
            Remove-Item -Path c:\windows-vhd-configuration.ps1
            (New-Guid).Guid | Out-File -FilePath 'c:\vhd-id.txt'
        }
        default {
            Write-Log "Unable to determine provisiong phase... exiting"
            exit 1
        }
    }
}
finally {
    Get-SystemDriveDiskInfo
    Get-DefenderPreferenceInfo
}
