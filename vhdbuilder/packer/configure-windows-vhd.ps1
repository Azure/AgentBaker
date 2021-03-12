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

$global:containerdPackageUrl = "https://mobyartifacts.azureedge.net/moby/moby-containerd/1.4.3+azure/windows/windows_amd64/moby-containerd-1.4.3+azure-1.amd64.zip"

function Write-Log($Message) {
    $msg = $message | Timestamp
    Write-Output $msg
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
        { '2019', '2019-containerd'} {
            $imagesToPull = @(
                "mcr.microsoft.com/windows/servercore:ltsc2019",
                "mcr.microsoft.com/windows/nanoserver:1809",
                "mcr.microsoft.com/oss/kubernetes/pause:1.4.1",
                "mcr.microsoft.com/oss/kubernetes-csi/livenessprobe:v2.0.1-alpha.1-windows-1809-amd64",
                "mcr.microsoft.com/oss/kubernetes-csi/livenessprobe:v2.2.0",
                "mcr.microsoft.com/oss/kubernetes-csi/csi-node-driver-registrar:v1.2.1-alpha.1-windows-1809-amd64",
                "mcr.microsoft.com/oss/kubernetes-csi/csi-node-driver-registrar:v2.0.1",
                "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v0.5.1",
                "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v0.6.0",
                "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v0.7.0",
                "mcr.microsoft.com/oss/kubernetes-csi/secrets-store/driver:v0.0.19",
                "mcr.microsoft.com/oss/azure/secrets-store/provider-azure:0.0.12",
                "mcr.microsoft.com/oss/kubernetes-csi/azuredisk-csi:v1.0.0",
                "mcr.microsoft.com/oss/kubernetes-csi/azuredisk-csi:v1.1.0",
                "mcr.microsoft.com/oss/kubernetes-csi/azuredisk-csi:v1.1.1",
                "mcr.microsoft.com/oss/kubernetes-csi/azurefile-csi:v1.0.0")
            Write-Log "Pulling images for windows server 2019"
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
        foreach ($image in $imagesToPull) {
            & ctr.exe -n k8s.io images pull $image
        }
    }
    else {
        foreach ($image in $imagesToPull) {
            docker pull $image
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
            "https://acs-mirror.azureedge.net/aks-engine/windows/provisioning/signedscripts-v0.0.3.zip",
            "https://acs-mirror.azureedge.net/aks-engine/windows/provisioning/signedscripts-v0.0.4.zip",
            "https://acs-mirror.azureedge.net/aks-engine/windows/provisioning/signedscripts-v0.0.8.zip",
            "https://acs-mirror.azureedge.net/aks-engine/windows/provisioning/signedscripts-v0.0.10.zip"
        );
        "c:\akse-cache\containerd\"   = @(
            $global:containerdPackageUrl
        );
        "c:\akse-cache\csi-proxy\"    = @(
            "https://acs-mirror.azureedge.net/csi-proxy/v0.2.2/binaries/csi-proxy-v0.2.2.tar.gz"
        );
        "c:\akse-cache\win-k8s\"      = @(
            "https://acs-mirror.azureedge.net/kubernetes/v1.16.10-hotfix.20200817/windowszip/v1.16.10-hotfix.20200817-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.16.13-hotfix.20210118/windowszip/v1.16.13-hotfix.20210118-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.16.15-hotfix.20210118/windowszip/v1.16.15-hotfix.20210118-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.17.7-hotfix.20200817/windowszip/v1.17.7-hotfix.20200817-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.17.9-hotfix.20200824/windowszip/v1.17.9-hotfix.20200824-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.17.11-hotfix.20200901/windowszip/v1.17.11-hotfix.20200901-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.17.13-hotfix.20210118/windowszip/v1.17.13-hotfix.20210118-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.17.16-hotfix.20210118/windowszip/v1.17.16-hotfix.20210118-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.18.4-hotfix.20200626/windowszip/v1.18.4-hotfix.20200626-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.18.6-hotfix.20200723/windowszip/v1.18.6-hotfix.20200723-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.18.8-hotfix.20200924/windowszip/v1.18.8-hotfix.20200924-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.18.10-hotfix.20210118/windowszip/v1.18.10-hotfix.20210118-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.18.14-hotfix.20210118/windowszip/v1.18.14-hotfix.20210118-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.19.0/windowszip/v1.19.0-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.19.1-hotfix.20200923/windowszip/v1.19.1-hotfix.20200923-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.19.3-hotfix.20210118/windowszip/v1.19.3-hotfix.20210118-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.19.6-hotfix.20210118/windowszip/v1.19.6-hotfix.20210118-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.19.7-hotfix.20210122/windowszip/v1.19.7-hotfix.20210122-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.20.2/windowszip/v1.20.2-1int.zip"
        );
        "c:\akse-cache\win-vnet-cni\" = @(
            "https://acs-mirror.azureedge.net/azure-cni/v1.2.0/binaries/azure-vnet-cni-singletenancy-windows-amd64-v1.2.0.zip",
            "https://acs-mirror.azureedge.net/azure-cni/v1.2.0_hotfix/binaries/azure-vnet-cni-singletenancy-windows-amd64-v1.2.0_hotfix.zip",
            "https://acs-mirror.azureedge.net/azure-cni/v1.2.2/binaries/azure-vnet-cni-singletenancy-windows-amd64-v1.2.2.zip",
            "https://acs-mirror.azureedge.net/azure-cni/v1.2.6/binaries/azure-vnet-cni-singletenancy-windows-amd64-v1.2.6.zip"
        );
        "c:\akse-cache\calico\" = @(
            "https://acs-mirror.azureedge.net/calico-node/v3.17.1/binaries/calico-windows-v3.17.1.zip",
            "https://acs-mirror.azureedge.net/calico-node/v3.17.2/binaries/calico-windows-v3.17.2.zip"
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
            curl.exe --retry 5 --retry-delay 0 -L $URL -o $dest
        }
    }
}

function Install-ContainerD {
    Write-Log "Getting containerD binaries from $global:containerdPackageUrl"

    $installDir = "c:\program files\containerd"
    $zipPath = [IO.Path]::Combine($installDir, "containerd.zip")

    Write-Log "Installing containerd to $installDir"
    New-Item -ItemType Directory $installDir -Force | Out-Null
    curl.exe --retry 5 --retry-delay 0 -L $global:containerdPackageUrl -o $zipPath
    Expand-Archive -Path $zipPath -DestinationPath $installDir
    Remove-Item -Path $zipPath | Out-null

    $newPath = [Environment]::GetEnvironmentVariable("Path", [EnvironmentVariableTarget]::Machine) + ";$installDir"
    [Environment]::SetEnvironmentVariable("Path", $newPath, [EnvironmentVariableTarget]::Machine)
    $env:Path += ";$installDir"

    Write-Log "Registering containerd as a service"
    & containerd.exe --register-service
    $svc = Get-Service -Name "containerd" -ErrorAction SilentlyContinue
    if ($null -eq $svc) {
        throw "containerd.exe did not get installed as a service correctly."
    }

    Write-Log "Starting containerd service"
    $svc | Start-Service
    if ($svc.Status -ne "Running") {
        throw "containerd service is not running"
    }
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

    $patchUrls = @("http://download.windowsupdate.com/c/msdownload/update/software/secu/2021/03/windows10.0-kb5000822-x64_567b66c719842beeaaf49e6332aa6f3477c225a7.msu")

    foreach ($patchUrl in $patchUrls) {
        $pathOnly = $patchUrl.Split("?")[0]
        $fileName = Split-Path $pathOnly -Leaf
        $fileExtension = [IO.Path]::GetExtension($fileName)
        $fullPath = [IO.Path]::Combine($env:TEMP, $fileName)

        switch ($fileExtension) {
            ".msu" {
                Write-Log "Downloading windows patch from $pathOnly to $fullPath"
                curl.exe --retry 5 --retry-delay 0 -L $patchUrl -o $fullPath
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

function Update-DefenderSignatures {
    Write-Log "Updating windows defender signatures."
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
    Write-Host "Enable a HNS fix in 2021-2C"
    reg add HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\hns\State /v HNSControlFlag /t REG_DWORD /d 1
}

# Disable progress writers for this session to greatly speed up operations such as Invoke-WebRequest
$ProgressPreference = 'SilentlyContinue'

$containerRuntime = $env:ContainerRuntime
$validContainerRuntimes = @('containerd', 'docker')
if (-not ($validContainerRuntimes -contains $containerRuntime)) {
    Write-Host "Unsupported container runtime: $containerRuntime"
    exit 1
}

$windowsSKU = $env:WindowsSKU
$validSKU = @('2019', '2019-containerd', '2004')
if (-not ($validSKU -contains $windowsSKU)) {
    Write-Host "Unsupported windows image SKU: $windowsSKU"
    exit 1
}

switch ($env:ProvisioningPhase) {
    "1" {
        Write-Log "Performing actions for provisioning phase 1"
        Set-WinRmServiceDelayedStart
        Set-AllowedSecurityProtocols
        Disable-WindowsUpdates
        Install-WindowsPatches
        Update-DefenderSignatures
        Install-OpenSSH
        Update-WindowsFeatures
    }
    "2" {
        Write-Log "Performing actions for provisioning phase 2 for container runtime '$containerRuntime'"
        Set-WinRmServiceAutoStart
        # TODO: make decision on if we want to install docker along with containerd (will need to update CSE too,)
        Install-Docker
        if ($containerRuntime -eq 'containerd') {
            Install-ContainerD
        }
        Update-Registry
        Get-ContainerImages -containerRuntime $containerRuntime -windowsSKU $windowsSKU
        Get-FilesToCacheOnVHD -containerRuntime $containerRuntime
        (New-Guid).Guid | Out-File -FilePath 'c:\vhd-id.txt'
    }
    default {
        Write-Log "Unable to determine provisiong phase... exiting"
        exit 1
    }
}
