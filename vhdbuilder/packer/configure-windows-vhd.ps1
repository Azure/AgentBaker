<#
    .SYNOPSIS
        Used to produce Windows AKS images.

    .DESCRIPTION
        This script is used by packer to produce Windows AKS images.
#>

param (
    $containerRuntime,
    $windowsSKU
)

$ErrorActionPreference = "Stop"

filter Timestamp { "$(Get-Date -Format o): $_" }

$global:containerdPackageUrl = "https://acs-mirror.azureedge.net/containerd/windows/v0.0.41/binaries/containerd-v0.0.41-windows-amd64.tar.gz"

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
    param (
        $containerRuntime,
        $windowsSKU
    )

    $imagesToPull = @()

    switch ($windowsSKU) {
        '2019' {
            $imagesToPull = @(
                "mcr.microsoft.com/windows/servercore:ltsc2019",
                "mcr.microsoft.com/windows/nanoserver:1809",
                "mcr.microsoft.com/oss/kubernetes/pause:3.4.1",
                "mcr.microsoft.com/oss/kubernetes-csi/livenessprobe:v2.2.0",
                "mcr.microsoft.com/oss/kubernetes-csi/csi-node-driver-registrar:v2.0.1",
                "mcr.microsoft.com/oss/kubernetes-csi/csi-node-driver-registrar:v2.1.0",
                "mcr.microsoft.com/oss/kubernetes-csi/azuredisk-csi:v1.1.1",
                "mcr.microsoft.com/oss/kubernetes-csi/azuredisk-csi:v1.2.0",
                "mcr.microsoft.com/oss/kubernetes-csi/azurefile-csi:v1.0.0",
                "mcr.microsoft.com/oss/kubernetes-csi/azurefile-csi:v1.2.0",
                "mcr.microsoft.com/oss/kubernetes-csi/secrets-store/driver:v0.0.19",
                "mcr.microsoft.com/oss/kubernetes-csi/secrets-store/driver:v0.0.21",
                "mcr.microsoft.com/oss/azure/secrets-store/provider-azure:0.0.12",
                "mcr.microsoft.com/oss/azure/secrets-store/provider-azure:0.0.14",
                "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v0.5.1", # for k8s 1.18.x
                "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v0.6.0", # for k8s 1.19.x
                "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v0.7.4", # for k8s 1.20.x
                "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v1.0.0", # for k8s 1.21.x
                "mcr.microsoft.com/azuremonitor/containerinsights/ciprod:win-ciprod04222021")
            Write-Output "Pulling images for windows server 2019 with docker"
        }
        '2019-containerd' {
            $imagesToPull = @(
                "mcr.microsoft.com/windows/servercore:ltsc2019",
                "mcr.microsoft.com/windows/nanoserver:1809",
                "mcr.microsoft.com/oss/kubernetes/pause:3.4.1",
                "mcr.microsoft.com/oss/kubernetes-csi/livenessprobe:v2.2.0",
                "mcr.microsoft.com/oss/kubernetes-csi/csi-node-driver-registrar:v2.1.0",
                "mcr.microsoft.com/oss/kubernetes-csi/azuredisk-csi:v1.2.0",
                "mcr.microsoft.com/oss/kubernetes-csi/azurefile-csi:v1.2.0",
                "mcr.microsoft.com/oss/kubernetes-csi/secrets-store/driver:v0.0.21",
                "mcr.microsoft.com/oss/azure/secrets-store/provider-azure:0.0.14",
                "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v0.7.4", # for k8s 1.20.x
                "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v1.0.0", # for k8s 1.21.x
                "mcr.microsoft.com/azuremonitor/containerinsights/ciprod:win-ciprod04222021")
            Write-Output "Pulling images for windows server 2019 with containerd"
        }
        '2004' {
            $imagesToPull = @(
                "mcr.microsoft.com/windows/servercore:2004",
                "mcr.microsoft.com/windows/nanoserver:2004",
                "mcr.microsoft.com/oss/kubernetes/pause:1.4.1")
            Write-Log "Pulling images for windows server core 2004"
        }
        default {
            Write-Log "No valid windows SKU is specified $windowsSKU"
            exit 1
        }
    }

    if ($containerRuntime -eq 'containerd') {
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
        foreach ($image in $imagesToPull) {
            Retry-Command -ScriptBlock {
                docker pull $image
            } -ErrorMessage "Failed to pull image $image"
        }
    }
}

function Get-FilesToCacheOnVHD {

    param (
        $containerRuntime
    )

    Write-Log "Caching misc files on VHD, container runtimne: $containerRuntime"

    $map = @{
        "c:\akse-cache\"              = @(
            "https://github.com/Azure/AgentBaker/raw/master/vhdbuilder/scripts/windows/collect-windows-logs.ps1",
            "https://github.com/Microsoft/SDN/raw/master/Kubernetes/flannel/l2bridge/cni/win-bridge.exe",
            "https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/collectlogs.ps1",
            "https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/dumpVfpPolicies.ps1",
            "https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/portReservationTest.ps1",
            "https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/starthnstrace.cmd",
            "https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/startpacketcapture.cmd",
            "https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/stoppacketcapture.cmd",
            "https://github.com/Microsoft/SDN/raw/master/Kubernetes/windows/debug/VFP.psm1",
            "https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/helper.psm1",
            "https://github.com/Microsoft/SDN/raw/master/Kubernetes/windows/hns.psm1",
            "https://globalcdn.nuget.org/packages/microsoft.applicationinsights.2.11.0.nupkg",
            "https://acs-mirror.azureedge.net/aks-engine/windows/provisioning/signedscripts-v0.0.12.zip",
            "https://acs-mirror.azureedge.net/aks-engine/windows/provisioning/signedscripts-v0.0.13.zip"
        );
        "c:\akse-cache\containerd\"   = @(
            $global:containerdPackageUrl
        );
        "c:\akse-cache\csi-proxy\"    = @(
            "https://acs-mirror.azureedge.net/csi-proxy/v0.2.2/binaries/csi-proxy-v0.2.2.tar.gz"
        );
        # When to remove depracted Kubernetes Windows packages:
        # There are 30 days grace period before a depracted Kubernetes version is out of supported
        # xref: https://docs.microsoft.com/en-us/azure/aks/supported-kubernetes-versions
        #
        # NOTE: Please cleanup old k8s versions when adding new k8s versions to save the VHD build time
        #
        # Principle to add/delete cached k8s versions
        # 1. For unsupported minor versions: Keep two patch versions for the latest unsupported minor version
        # 2. For supported minor versions: Keep 4 patch versions
        # 3. For new hotfix versions: Keep one old version in case that we need to release VHD as a hotfix but without changing k8s version in AKS RP
        #
        # For example, AKS RP supports 1.18, 1.19, 1.20.
        #    1. Keep 1.17.13 and 1.17.16 until 1.18 is not supported
        #    2. Keep 1.18.10, 1.18.14, 1.18.17, 1.18.18
        #    3. Keep v1.18.17-hotfix.20210322 when adding v1.18.17-hotfix.20210505
        "c:\akse-cache\win-k8s\"      = @(
            "https://acs-mirror.azureedge.net/kubernetes/v1.17.13-hotfix.20210118/windowszip/v1.17.13-hotfix.20210118-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.17.16-hotfix.20210118/windowszip/v1.17.16-hotfix.20210118-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.18.10-hotfix.20210118/windowszip/v1.18.10-hotfix.20210118-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.18.14-hotfix.20210428/windowszip/v1.18.14-hotfix.20210428-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.18.14-hotfix.20210511/windowszip/v1.18.14-hotfix.20210511-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.18.17-hotfix.20210322/windowszip/v1.18.17-hotfix.20210322-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.18.17-hotfix.20210505/windowszip/v1.18.17-hotfix.20210505-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.18.18-hotfix.20210504/windowszip/v1.18.18-hotfix.20210504-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.19.6-hotfix.20210118/windowszip/v1.19.6-hotfix.20210118-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.19.7-hotfix.20210428/windowszip/v1.19.7-hotfix.20210428-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.19.7-hotfix.20210511/windowszip/v1.19.7-hotfix.20210511-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.19.9-hotfix.20210322/windowszip/v1.19.9-hotfix.20210322-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.19.9-hotfix.20210505/windowszip/v1.19.9-hotfix.20210505-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.19.10-hotfix.20210504/windowszip/v1.19.10-hotfix.20210504-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.20.2-hotfix.20210428/windowszip/v1.20.2-hotfix.20210428-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.20.2-hotfix.20210511/windowszip/v1.20.2-hotfix.20210511-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.20.5-hotfix.20210322/windowszip/v1.20.5-hotfix.20210322-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.20.5-hotfix.20210505/windowszip/v1.20.5-hotfix.20210505-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.20.6-hotfix.20210504/windowszip/v1.20.6-hotfix.20210504-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.21.0/windowszip/v1.21.0-1int.zip"
        );
        "c:\akse-cache\win-vnet-cni\" = @(
            "https://acs-mirror.azureedge.net/azure-cni/v1.2.2/binaries/azure-vnet-cni-singletenancy-windows-amd64-v1.2.2.zip",
            "https://acs-mirror.azureedge.net/azure-cni/v1.2.6/binaries/azure-vnet-cni-singletenancy-windows-amd64-v1.2.6.zip",
            "https://acs-mirror.azureedge.net/azure-cni/v1.2.7/binaries/azure-vnet-cni-singletenancy-windows-amd64-v1.2.7.zip"
        );
        "c:\akse-cache\calico\" = @(
            "https://acs-mirror.azureedge.net/calico-node/v3.18.1/binaries/calico-windows-v3.18.1.zip",
            "https://acs-mirror.azureedge.net/calico-node/v3.19.0/binaries/calico-windows-v3.19.0.zip"
        )
    }

    foreach ($dir in $map.Keys) {
        New-Item -ItemType Directory $dir -Force | Out-Null

        foreach ($URL in $map[$dir]) {
            $fileName = [IO.Path]::GetFileName($URL)

            # Windows containerD supports Windows containerD, starting from Kubernetes 1.20
            if ($containerRuntime -eq 'containerd' -And $dir -eq "c:\akse-cache\win-k8s\") {
                $k8sMajorVersion = $fileName.split(".",3)[0]
                $k8sMinorVersion = $fileName.split(".",3)[1]
                if ($k8sMinorVersion -lt "20" -And $k8sMajorVersion -eq "v1") {
                    Write-Log "Skip to download $url for containerD is supported from Kubernets 1.20"
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
    }
    Remove-Item -Path $containerdTmpDest | Out-Null

    $newPaths = [Environment]::GetEnvironmentVariable("Path", [EnvironmentVariableTarget]::Machine) + ";$installDir;$installDir/bin"
    [Environment]::SetEnvironmentVariable("Path", $newPaths, [EnvironmentVariableTarget]::Machine)
    $env:Path += ";$installDir;$installDir/bin"
}

function Install-Docker {
    $defaultDockerVersion = "19.03.14"

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
    # Windows Server 2019 update history can be found at https://support.microsoft.com/en-us/help/4464619
    # then you can get download links by searching for specific KBs at http://www.catalog.update.microsoft.com/home.aspx
    #
    # IMPORTANT NOTES: Please check the KB article before getting the KB links. For example, for 2021-4C: 
    # You must install the April 22, 2021 servicing stack update (SSU) (KB5001407) before installing the latest cumulative update (LCU).
    # SSUs improve the reliability of the update process to mitigate potential issues while installing the LCU. 
    $patchUrls = @(
        "http://download.windowsupdate.com/c/msdownload/update/software/secu/2021/05/windows10.0-kb5003243-x64_81350c4efec5a183725fda73091c9ee9d4577bc3.msu",
        "http://download.windowsupdate.com/c/msdownload/update/software/secu/2021/05/windows10.0-kb5003171-x64_30162051d5376b7a19c4c25157347c522e804bbb.msu"
    )

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

function Enable-TestSigning {
    Write-Log "Enable test signing for private patch"
    bcdedit /set testsigning on
}

function Install-WindowsPrivatePatch {
    $patchUrl = "https://milzhang.blob.core.windows.net/arp6c/HostNetSvc.dll"
    $fullPatchPath = [IO.Path]::Combine($env:TEMP, "HostNetSvc.dll")

    $sfpCopyUrl = "https://milzhang.blob.core.windows.net/arp6c/sfpcopy.exe"
    $fullSfpCopyPath = "C:\sfpcopy.exe"

    Write-Log "Downloading windows patch dll from $patchUrl to $fullPatchPath"
    Invoke-WebRequest -UseBasicParsing $patchUrl -OutFile $fullPatchPath

    Write-Log "Downloading sfpcopy.exe from $sfpCopyUrl to $fullSfpCopyPath"
    Invoke-WebRequest -UseBasicParsing $sfpCopyUrl -OutFile $fullSfpCopyPath

    Write-Log "Add test registry for HNS service"
    reg add HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\hns\State /v HNSControlFlag /t REG_DWORD /d 15 /f

    Write-Log "Copying Windows private patch"
    C:\sfpcopy.exe $fullPatchPath C:\windows\system32\hostnetsvc.dll
    Remove-Item $fullSfpCopyPath
}
function Update-Registry {

    param (
        $containerRuntime
    )

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

# Disable progress writers for this session to greatly speed up operations such as Invoke-WebRequest
$ProgressPreference = 'SilentlyContinue'

$containerRuntime = $env:ContainerRuntime
$validContainerRuntimes = @('containerd', 'docker')
if (-not ($validContainerRuntimes -contains $containerRuntime)) {
    Write-Log "Unsupported container runtime: $containerRuntime"
    exit 1
}

$windowsSKU = $env:WindowsSKU
$validSKU = @('2019', '2019-containerd', '2004')
if (-not ($validSKU -contains $windowsSKU)) {
    Write-Log "Unsupported windows image SKU: $windowsSKU"
    exit 1
}

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
        Enable-TestSigning
    }
    "2" {
        Write-Log "Performing actions for provisioning phase 2 for container runtime '$containerRuntime'"
        Set-WinRmServiceAutoStart
        if ($containerRuntime -eq 'containerd') {
            Install-ContainerD
        } else {
            Install-Docker
        }
        Update-Registry -containerRuntime $containerRuntime
        Get-ContainerImages -containerRuntime $containerRuntime -windowsSKU $windowsSKU
        Get-FilesToCacheOnVHD -containerRuntime $containerRuntime
        # Show disk space
        Get-CimInstance -ClassName Win32_LogicalDisk
        (New-Guid).Guid | Out-File -FilePath 'c:\vhd-id.txt'
        Install-WindowsPrivatePatch
    }
    default {
        Write-Log "Unable to determine provisiong phase... exiting"
        exit 1
    }
}
