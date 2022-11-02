# MUST define global variable with "global"
# This script is used to generate shared configuration for configure-windows-vhd.ps1 and windows-vhd-content-test.ps1.
# MUST NOT add any shared functions in this script.
$windowsConfig = @'
$global:containerRuntime = $env:ContainerRuntime
$validContainerRuntimes = @("containerd", "docker")
if (-not ($validContainerRuntimes -contains $containerRuntime)) {
    throw "Unsupported container runtime: $containerRuntime"
}

$global:windowsSKU = $env:WindowsSKU
$validSKU = @("2019", "2019-containerd", "2022-containerd", "2022-containerd-gen2")
if (-not ($validSKU -contains $windowsSKU)) {
    throw "Unsupported windows image SKU: $windowsSKU"
}

# Windows Server 2019 update history can be found at https://support.microsoft.com/en-us/help/4464619
# Windows Server 2022 update history can be found at https://support.microsoft.com/en-us/topic/windows-server-2022-update-history-e1caa597-00c5-4ab9-9f3e-8212fe80b2ee
# then you can get download links by searching for specific KBs at http://www.catalog.update.microsoft.com/home.aspx
#
# IMPORTANT NOTES: Please check the KB article before getting the KB links. For example, for 2021-4C:
# You must install the April 22, 2021 servicing stack update (SSU) (KB5001407) before installing the latest cumulative update (LCU).
# SSUs improve the reliability of the update process to mitigate potential issues while installing the LCU.
switch -Regex ($windowsSKU) {
    "2019*" {
        $global:patchUrls = @()
        $global:patchIDs = @()
    }
    "2022*" {
        $global:patchUrls = @("https://catalog.s.download.windowsupdate.com/d/msdownload/update/software/updt/2022/10/windows10.0-kb5018485-x64_13183a9eaf45a33f681d43fa80c1b283f213991c.msu")
        $global:patchIDs = @("KB5018485")
    }
}

# defaultContainerdPackageUrl refers to the latest containerd package used to pull and cache container images
$global:defaultContainerdPackageUrl = "https://acs-mirror.azureedge.net/containerd/windows/v0.0.47/binaries/containerd-v0.0.47-windows-amd64.tar.gz"

$global:defaultDockerVersion = "20.10.9"

if ($windowsSku -eq "2019") {
    $global:imagesToPull = @(
            "mcr.microsoft.com/windows/servercore:ltsc2019",
            "mcr.microsoft.com/windows/nanoserver:1809",
            "mcr.microsoft.com/oss/kubernetes/pause:3.6-hotfix.20220114",
            # CSI. Owner: andyzhangx (Andy Zhang)
            "mcr.microsoft.com/oss/kubernetes-csi/livenessprobe:v2.5.0",
            "mcr.microsoft.com/oss/kubernetes-csi/livenessprobe:v2.6.0",
            "mcr.microsoft.com/oss/kubernetes-csi/csi-node-driver-registrar:v2.4.0",
            "mcr.microsoft.com/oss/kubernetes-csi/csi-node-driver-registrar:v2.5.0",
            "mcr.microsoft.com/oss/kubernetes-csi/azuredisk-csi:v1.23.0",
            "mcr.microsoft.com/oss/kubernetes-csi/azuredisk-csi:v1.24.0",
            "mcr.microsoft.com/oss/kubernetes-csi/azurefile-csi:v1.22.0",
            "mcr.microsoft.com/oss/kubernetes-csi/azurefile-csi:v1.23.0",
            # Addon of Azure secrets store. Owner: ZeroMagic (Ji'an Liu)
            "mcr.microsoft.com/oss/kubernetes-csi/secrets-store/driver:v1.2.2",
            "mcr.microsoft.com/oss/azure/secrets-store/provider-azure:v1.2.0",
            # Azure cloud node manager. Owner: nilo19 (Qi Ni)
            "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v0.7.21", # for k8s 1.20.x
            "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v1.0.18", # for k8s 1.21.x
            "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v1.1.14", # for k8s 1.22.x
            "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v1.23.11", # for k8s 1.23.x
            # OMS-Agent (Azure monitor). Owner: ganga1980 (Ganga Mahesh Siddem)
            "mcr.microsoft.com/azuremonitor/containerinsights/ciprod:win-ciprod10042022-3c05dd1b")
} elseif ($windowsSku -eq "2019-containerd") {
    $global:imagesToPull = @(
            "mcr.microsoft.com/windows/servercore:ltsc2019",
            "mcr.microsoft.com/windows/nanoserver:1809",
            "mcr.microsoft.com/oss/kubernetes/pause:3.6-hotfix.20220114",
            # CSI. Owner: andyzhangx (Andy Zhang)
            "mcr.microsoft.com/oss/kubernetes-csi/livenessprobe:v2.5.0",
            "mcr.microsoft.com/oss/kubernetes-csi/livenessprobe:v2.6.0",
            "mcr.microsoft.com/oss/kubernetes-csi/csi-node-driver-registrar:v2.4.0",
            "mcr.microsoft.com/oss/kubernetes-csi/csi-node-driver-registrar:v2.5.0",
            "mcr.microsoft.com/oss/kubernetes-csi/azuredisk-csi:v1.23.0",
            "mcr.microsoft.com/oss/kubernetes-csi/azuredisk-csi:v1.24.0",
            "mcr.microsoft.com/oss/kubernetes-csi/azurefile-csi:v1.22.0",
            "mcr.microsoft.com/oss/kubernetes-csi/azurefile-csi:v1.23.0",
            # Addon of Azure secrets store. Owner: ZeroMagic (Ji'an Liu)
            "mcr.microsoft.com/oss/kubernetes-csi/secrets-store/driver:v1.2.2",
            "mcr.microsoft.com/oss/azure/secrets-store/provider-azure:v1.2.0",
            # Azure cloud node manager. Owner: nilo19 (Qi Ni)
            "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v0.7.21", # for k8s 1.20.x
            "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v1.0.18", # for k8s 1.21.x
            "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v1.1.14", # for k8s 1.22.x
            "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v1.23.11", # for k8s 1.23.x
            "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v1.24.3", # for k8s 1.24.x
            # OMS-Agent (Azure monitor). Owner: ganga1980 (Ganga Mahesh Siddem)
            "mcr.microsoft.com/azuremonitor/containerinsights/ciprod:win-ciprod10042022-3c05dd1b")
} elseif ($windowsSku -eq "2022-containerd" -or $windowsSku -eq "2022-containerd-gen2") {
    $global:imagesToPull = @(
            "mcr.microsoft.com/windows/servercore:ltsc2022",
            "mcr.microsoft.com/windows/nanoserver:ltsc2022",
            "mcr.microsoft.com/oss/kubernetes/pause:3.6-hotfix.20220114",
            # CSI. Owner: andyzhangx (Andy Zhang)
            "mcr.microsoft.com/oss/kubernetes-csi/livenessprobe:v2.5.0",
            "mcr.microsoft.com/oss/kubernetes-csi/livenessprobe:v2.6.0",
            "mcr.microsoft.com/oss/kubernetes-csi/csi-node-driver-registrar:v2.4.0",
            "mcr.microsoft.com/oss/kubernetes-csi/csi-node-driver-registrar:v2.5.0",
            "mcr.microsoft.com/oss/kubernetes-csi/azuredisk-csi:v1.23.0",
            "mcr.microsoft.com/oss/kubernetes-csi/azuredisk-csi:v1.24.0",
            "mcr.microsoft.com/oss/kubernetes-csi/azurefile-csi:v1.22.0",
            "mcr.microsoft.com/oss/kubernetes-csi/azurefile-csi:v1.23.0",
            # Addon of Azure secrets store. Owner: ZeroMagic (Ji'an Liu)
            "mcr.microsoft.com/oss/kubernetes-csi/secrets-store/driver:v1.2.2",
            "mcr.microsoft.com/oss/azure/secrets-store/provider-azure:v1.2.0",
            # Azure cloud node manager. Owner: nilo19 (Qi Ni)
            "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v1.23.11", # for k8s 1.23.x
            "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v1.24.3", # for k8s 1.24.x
            # OMS-Agent (Azure monitor). Owner: ganga1980 (Ganga Mahesh Siddem)
            "mcr.microsoft.com/azuremonitor/containerinsights/ciprod:win-ciprod10042022-3c05dd1b",
            # NPM (Network Policy Manager). Owner: jaer-tsun (Jaeryn)
            "mcr.microsoft.com/containernetworking/azure-npm:v1.4.29",
            "mcr.microsoft.com/containernetworking/azure-cns:v1.4.29")
} else {
    throw "No valid windows SKU is specified $windowsSKU"
}

$global:map = @{
    "c:\akse-cache\"              = @(
        "https://github.com/Azure/AgentBaker/raw/master/vhdbuilder/scripts/windows/collect-windows-logs.ps1",
        "https://github.com/Microsoft/SDN/raw/master/Kubernetes/flannel/l2bridge/cni/win-bridge.exe",
        "https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/collectlogs.ps1",
        "https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/dumpVfpPolicies.ps1",
        "https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/portReservationTest.ps1",
        "https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/starthnstrace.cmd",
        "https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/startpacketcapture.cmd",
        "https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/stoppacketcapture.cmd",
        "https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/starthnstrace.ps1",
        "https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/startpacketcapture.ps1",
        "https://github.com/Microsoft/SDN/raw/master/Kubernetes/windows/debug/VFP.psm1",
        "https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/networkmonitor/networkhealth.ps1",
        "https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/helper.psm1",
        "https://github.com/Microsoft/SDN/raw/master/Kubernetes/windows/hns.psm1",
        "https://github.com/Microsoft/SDN/raw/master/Kubernetes/windows/hns.v2.psm1",
        "https://globalcdn.nuget.org/packages/microsoft.applicationinsights.2.11.0.nupkg",
        "https://acs-mirror.azureedge.net/ccgakvplugin/v1.1.5/binaries/windows-gmsa-ccgakvplugin-v1.1.5.zip",
        "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v0.0.12.zip",
        "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v0.0.13.zip",
        "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v0.0.14.zip",
        "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v0.0.15.zip",
        "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v0.0.16.zip"
    );
    # Different from other packages which are downloaded/cached and used later only during CSE, windows containerd is installed
    # during building the Windows VHD to cache container images.
    # We use the latest containerd package to start containerd then cache images, and the latest one is expected to be
    # specified by AKS PR for most of the cases. BUT as long as there's a new unpacked image version, we should keep the
    # versions synced.
    "c:\akse-cache\containerd\"   = @(
        $defaultContainerdPackageUrl
    );
    "c:\akse-cache\csi-proxy\"    = @(
        "https://acs-mirror.azureedge.net/csi-proxy/v0.2.2/binaries/csi-proxy-v0.2.2.tar.gz",
        "https://acs-mirror.azureedge.net/csi-proxy/v1.0.2/binaries/csi-proxy-v1.0.2.tar.gz"
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
        "https://acs-mirror.azureedge.net/kubernetes/v1.21.1-hotfix.20211115/windowszip/v1.21.1-hotfix.20211115-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.21.2-hotfix.20211115/windowszip/v1.21.2-hotfix.20211115-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.21.7-hotfix.20220204/windowszip/v1.21.7-hotfix.20220204-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.21.9-hotfix.20220204/windowszip/v1.21.9-hotfix.20220204-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.21.13/windowszip/v1.21.13-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.21.14/windowszip/v1.21.14-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.22.1-hotfix.20211115/windowszip/v1.22.1-hotfix.20211115-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.22.4-hotfix.20220201/windowszip/v1.22.4-hotfix.20220201-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.22.6-hotfix.20220728/windowszip/v1.22.6-hotfix.20220728-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.22.10/windowszip/v1.22.10-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.22.11-hotfix.20220728/windowszip/v1.22.11-hotfix.20220728-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.22.15/windowszip/v1.22.15-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.23.3-hotfix.20220130/windowszip/v1.23.3-hotfix.20220130-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.23.4/windowszip/v1.23.4-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.23.5-hotfix.20220728/windowszip/v1.23.5-hotfix.20220728-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.23.7/windowszip/v1.23.7-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.23.8-hotfix.20220728/windowszip/v1.23.8-hotfix.20220728-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.23.12-hotfix.20220922/windowszip/v1.23.12-hotfix.20220922-1int.zip", 
        "https://acs-mirror.azureedge.net/kubernetes/v1.24.3-hotfix.20221006/windowszip/v1.24.3-hotfix.20221006-1int.zip", 
        "https://acs-mirror.azureedge.net/kubernetes/v1.24.6-hotfix.20221006/windowszip/v1.24.6-hotfix.20221006-1int.zip", 
        "https://acs-mirror.azureedge.net/kubernetes/v1.25.2-hotfix.20221006/windowszip/v1.25.2-hotfix.20221006-1int.zip"
    );
    "c:\akse-cache\win-vnet-cni\" = @(
        "https://acs-mirror.azureedge.net/azure-cni/v1.4.22/binaries/azure-vnet-cni-singletenancy-windows-amd64-v1.4.22.zip",
        "https://acs-mirror.azureedge.net/azure-cni/v1.4.35/binaries/azure-vnet-cni-singletenancy-windows-amd64-v1.4.35.zip",
        "https://acs-mirror.azureedge.net/azure-cni/v1.4.35/binaries/azure-vnet-cni-singletenancy-swift-windows-amd64-v1.4.35.zip",
        "https://acs-mirror.azureedge.net/azure-cni/v1.4.35/binaries/azure-vnet-cni-singletenancy-overlay-windows-amd64-v1.4.35.zip"
    );
    "c:\akse-cache\calico\" = @(
        "https://acs-mirror.azureedge.net/calico-node/v3.21.6/binaries/calico-windows-v3.21.6.zip"
    )
}
'@
# Both configure-windows-vhd.ps1 and windows-vhd-content-test.ps1 will import c:\windows-vhd-configuration.ps1
$windowsConfig | Out-File -FilePath c:\windows-vhd-configuration.ps1
