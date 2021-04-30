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
        $retryDelay = 0
    )
    curl.exe -f --retry $retryCount --retry-delay $retryDelay -L $URL -o $Dest
    if ($LASTEXITCODE) {
        throw "Curl exited with '$LASTEXITCODE' while attemping to download '$URL'"
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
                Start-Sleep $Delay
            }
        } while ($cnt -lt $Maximum)

        # Throw an error after $Maximum unsuccessful invocations. Doesn't need
        # a condition, since the function returns upon successful invocation.
        throw 'All retries failed. $ErrorMessage'
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
        Write-Log "Pulling images for windows server 2019 with containerd"
        # start containerd to pre-pull the images to disk on VHD
        # CSE will configure and register containerd as a service at deployment time
        Start-Job -Name containerd -ScriptBlock { containerd.exe }
        foreach ($image in $imagesToPull) {
            Retry-Command -ScriptBlock {
                & ctr.exe -n k8s.io images pull $image
            } -ErrorMessage "Failed to pull image $image"
        }
        Stop-Job  -Name containerd
        Remove-Job -Name containerd
    }
    else {
        Write-Log "Pulling images for windows server 2019 with docker"
        foreach ($image in $imagesToPull) {
            Retry-Command -ScriptBlock {
                docker pull $image
            } -ErrorMessage "Failed to pull image $image"
        }
    }
}

function Get-FilesToCacheOnVHD {
    Write-Log "Caching misc files on VHD, container runtimne: $containerRuntime"

    foreach ($dir in $map.Keys) {
        New-Item -ItemType Directory $dir -Force | Out-Null

        foreach ($URL in $map[$dir]) {
            $fileName = [IO.Path]::GetFileName($URL)

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
    Write-Log "Getting containerD binaries from $global:containerdPackageUrl"

    $installDir = "c:\program files\containerd"
    Write-Log "Installing containerd to $installDir"
    New-Item -ItemType Directory $installDir -Force | Out-Null

    $containerdFilename=[IO.Path]::GetFileName($global:containerdPackageUrl)
    $containerdTmpDest = [IO.Path]::Combine($installDir, $containerdFilename)
    DownloadFileWithRetry -URL $global:containerdPackageUrl -Dest $containerdTmpDest
    # The released containerd package format is either zip or tar.gz
    if ($containerdFilename.endswith(".zip")) {
        Expand-Archive -path $containerdTmpDest -DestinationPath $installDir -Force
    } else {
        tar -xzf $containerdTmpDest --strip=1 -C $installDir
        Get-ChildItem -Path $installDir\bin -Recurse -File | Move-Item -Destination $installDir
        Remove-Item -Path  $global:ContainerdInstallLocation\bin -Force
    }
    Remove-Item -Path $containerdTmpDest | Out-Null

    $newPaths = [Environment]::GetEnvironmentVariable("Path", [EnvironmentVariableTarget]::Machine) + ";$installDir"
    [Environment]::SetEnvironmentVariable("Path", $newPaths, [EnvironmentVariableTarget]::Machine)
    $env:Path += ";$installDir"
}

function Install-Docker {
    Write-Log "Attempting to install Docker version $defaultDockerVersion"
    Install-PackageProvider -Name DockerMsftProvider -Force -ForceBootstrap | Out-null
    $package = Find-Package -Name Docker -ProviderName DockerMsftProvider -RequiredVersion $defaultDockerVersion
    Write-Log "Installing Docker version $($package.Version)"
    $package | Install-Package -Force | Out-Null
    Start-Service docker
}


function Install-OpenSSH {
    Write-Log "Installing OpenSSH Server"
    Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0
}

function Install-WindowsPatches {
    foreach ($patchUrl in $patchUrls) {
        $pathOnly = $patchUrl.Split("?")[0]
        $fileName = Split-Path $pathOnly -Leaf
        $fileExtension = [IO.Path]::GetExtension($fileName)
        $fullPath = [IO.Path]::Combine($env:TEMP, $fileName)

        switch ($fileExtension) {
            ".msu" {
                Write-Log "Downloading windows patch from $pathOnly to $fullPath"
                DownloadFileWithRetry -URL $patchUrl -Dest $fullPath
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

function Set-AllowedSecurityProtocols {
    $allowedProtocols = @()
    $insecureProtocols = @([System.Net.SecurityProtocolType]::SystemDefault, [System.Net.SecurityProtocolType]::Ssl3)

    foreach ($protocol in [System.Enum]::GetValues([System.Net.SecurityProtocolType])) {
        if ($insecureProtocols -notcontains $protocol) {
            $allowedProtocols += $protocol
        }
    }

    Write-Log "Settings allowed security protocols to: $allowedProtocols"
    [System.Net.ServicePointManager]::SecurityProtocol = $allowedProtocols
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
    # if multple LB policies are included for same endpoint then HNS hangs.
    # this fix forces an error
    Write-Log "Enable a HNS fix in 2021-2C"
    Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name HNSControlFlag -Value 1 -Type DWORD

    # Enables DNS resolution of SMB shares for containerD
    # https://github.com/kubernetes-sigs/windows-gmsa/issues/30#issuecomment-802240945
    if ($containerRuntime -eq 'containerd') {
        Write-Log "Apply SMB Resolution Fix for containerD"
        Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name EnableCompartmentNamespace -Value 1 -Type DWORD
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

# Disable progress writers for this session to greatly speed up operations such as Invoke-WebRequest
$ProgressPreference = 'SilentlyContinue'

switch ($env:ProvisioningPhase) {
    "1" {
        Write-Log "Performing actions for provisioning phase 1"
        Disable-WindowsUpdates
        Set-WinRmServiceDelayedStart
        Update-DefenderSignatures
        Set-AllowedSecurityProtocols
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
        Get-SystemDriveDiskInfo
        (New-Guid).Guid | Out-File -FilePath 'c:\vhd-id.txt'
    }
    default {
        Write-Log "Unable to determine provisiong phase... exiting"
        exit 1
    }
}
