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

function Retag-ImageForAzureChinaCloud {
    Param(
        [string]
        $imageUrl
    )
    Write-Log "Retagging image $imageUrl for AzureChinaCloud"
    $retagImageUrl=$image.replace('mcr.microsoft.com', 'mcr.azk8s.cn')
    ctr.exe -n k8s.io image tag $imageUrl $retagImageUrl
}

function Get-ContainerImages {
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

        Retag-ImageForAzureChinaCloud -imageUrl $image
    }
    Stop-Job  -Name containerd
    Remove-Job -Name containerd
}

function Get-FilesToCacheOnVHD {
    Write-Log "Caching misc files on VHD"

    foreach ($dir in $map.Keys) {
        New-Item -ItemType Directory $dir -Force | Out-Null

        foreach ($URL in $map[$dir]) {
            $fileName = [IO.Path]::GetFileName($URL)
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
    Write-Log "Apply SMB Resolution Fix for containerD"
    Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name EnableCompartmentNamespace -Value 1 -Type DWORD

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

        Write-Log "Enable 3 fixes in 2023-04B"
        $currentValue=(Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name HnsPolicyUpdateChange -ErrorAction Ignore)
        if (![string]::IsNullOrEmpty($currentValue)) {
            Write-Log "The current value of HnsPolicyUpdateChange is $currentValue"
        }
        Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name HnsPolicyUpdateChange -Value 1 -Type DWORD

        $currentValue=(Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name HnsNatAllowRuleUpdateChange -ErrorAction Ignore)
        if (![string]::IsNullOrEmpty($currentValue)) {
            Write-Log "The current value of HnsNatAllowRuleUpdateChange is $currentValue"
        }
        Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name HnsNatAllowRuleUpdateChange -Value 1 -Type DWORD

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

        $currentValue=(Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft\FeatureManagement\Overrides" -Name 3105872524 -ErrorAction Ignore)
        if (![string]::IsNullOrEmpty($currentValue)) {
            Write-Log "The current value of 3105872524 is $currentValue"
        }
        Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft\FeatureManagement\Overrides" -Name 3105872524 -Value 1 -Type DWORD

        Write-Log "Enable 1 fix in 2023-05B"
        $regPath=(Get-Item -Path "HKLM:\SYSTEM\CurrentControlSet\Services\VfpExt" -ErrorAction Ignore)
        if (!$regPath) {
            Write-Log "Creating HKLM:\SYSTEM\CurrentControlSet\Services\VfpExt"
            New-Item -Path "HKLM:\SYSTEM\CurrentControlSet\Services\VfpExt"
        }

        $regPath=(Get-Item -Path "HKLM:\SYSTEM\CurrentControlSet\Services\VfpExt\Parameters" -ErrorAction Ignore)
        if (!$regPath) {
            Write-Log "Creating HKLM:\SYSTEM\CurrentControlSet\Services\VfpExt\Parameters"
            New-Item -Path "HKLM:\SYSTEM\CurrentControlSet\Services\VfpExt\Parameters"
        }

        $currentValue=(Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\VfpExt\Parameters" -Name VfpEvenPodDistributionIsEnabled -ErrorAction Ignore)
        if (![string]::IsNullOrEmpty($currentValue)) {
            Write-Log "The current value of VfpEvenPodDistributionIsEnabled is $currentValue"
        }
        Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\VfpExt\Parameters" -Name VfpEvenPodDistributionIsEnabled -Value 1 -Type DWORD
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

         Write-Log "Enable 4 fixes in 2023-04B"
        $currentValue=(Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name HnsPolicyUpdateChange -ErrorAction Ignore)
        if (![string]::IsNullOrEmpty($currentValue)) {
            Write-Log "The current value of HnsPolicyUpdateChange is $currentValue"
        }
        Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name HnsPolicyUpdateChange -Value 1 -Type DWORD

        $currentValue=(Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name HnsNatAllowRuleUpdateChange -ErrorAction Ignore)
        if (![string]::IsNullOrEmpty($currentValue)) {
            Write-Log "The current value of HnsNatAllowRuleUpdateChange is $currentValue"
        }
        Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name HnsNatAllowRuleUpdateChange -Value 1 -Type DWORD

        $currentValue=(Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft\FeatureManagement\Overrides" -Name 3508525708 -ErrorAction Ignore)
        if (![string]::IsNullOrEmpty($currentValue)) {
            Write-Log "The current value of 3508525708 is $currentValue"
        }
        Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft\FeatureManagement\Overrides" -Name 3508525708 -Value 1 -Type DWORD
        
        $currentValue=(Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name HnsAclUpdateChange -ErrorAction Ignore)
        if (![string]::IsNullOrEmpty($currentValue)) {
            Write-Log "The current value of HnsAclUpdateChange is $currentValue"
        }
        Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name HnsAclUpdateChange -Value 1 -Type DWORD

        Write-Log "Enable 4 fixes in 2023-05B"
        $regPath=(Get-Item -Path "HKLM:\SYSTEM\CurrentControlSet\Services\VfpExt" -ErrorAction Ignore)
        if (!$regPath) {
            Write-Log "Creating HKLM:\SYSTEM\CurrentControlSet\Services\VfpExt"
            New-Item -Path "HKLM:\SYSTEM\CurrentControlSet\Services\VfpExt"
        }

        $regPath=(Get-Item -Path "HKLM:\SYSTEM\CurrentControlSet\Services\VfpExt\Parameters" -ErrorAction Ignore)
        if (!$regPath) {
            Write-Log "Creating HKLM:\SYSTEM\CurrentControlSet\Services\VfpExt\Parameters"
            New-Item -Path "HKLM:\SYSTEM\CurrentControlSet\Services\VfpExt\Parameters"
        }

        $currentValue=(Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\VfpExt\Parameters" -Name VfpEvenPodDistributionIsEnabled -ErrorAction Ignore)
        if (![string]::IsNullOrEmpty($currentValue)) {
            Write-Log "The current value of VfpEvenPodDistributionIsEnabled is $currentValue"
        }
        Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\VfpExt\Parameters" -Name VfpEvenPodDistributionIsEnabled -Value 1 -Type DWORD
        
        $currentValue=(Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name HnsNpmRefresh -ErrorAction Ignore)
        if (![string]::IsNullOrEmpty($currentValue)) {
            Write-Log "The current value of HnsNpmRefresh is $currentValue"
        }
        Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name HnsNpmRefresh -Value 1 -Type DWORD

        $currentValue=(Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft\FeatureManagement\Overrides" -Name 1995963020 -ErrorAction Ignore)
        if (![string]::IsNullOrEmpty($currentValue)) {
            Write-Log "The current value of 1995963020 is $currentValue"
        }
        Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft\FeatureManagement\Overrides" -Name 1995963020 -Value 1 -Type DWORD

        $currentValue=(Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft\FeatureManagement\Overrides" -Name 189519500 -ErrorAction Ignore)
        if (![string]::IsNullOrEmpty($currentValue)) {
            Write-Log "The current value of 189519500 is $currentValue"
        }
        Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft\FeatureManagement\Overrides" -Name 189519500 -Value 1 -Type DWORD
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
            Write-Log "Performing actions for provisioning phase 2"
            Set-WinRmServiceAutoStart
            Install-ContainerD
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
