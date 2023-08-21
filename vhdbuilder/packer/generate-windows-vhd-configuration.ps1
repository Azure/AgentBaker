# MUST define global variable with "global"
# This script is used to generate shared configuration for configure-windows-vhd.ps1 and windows-vhd-content-test.ps1.
# MUST NOT add any shared functions in this script.
$windowsConfig = @'
$global:windowsSKU = $env:WindowsSKU
$validSKU = @("2019-containerd", "2022-containerd", "2022-containerd-gen2")
if (-not ($validSKU -contains $windowsSKU)) {
    throw "Unsupported windows image SKU: $windowsSKU"
}

# defaultContainerdPackageUrl refers to the stable containerd package used to pull and cache container images
# Add cache for another containerd version which is not installed by default
$global:defaultContainerdPackageUrl = "https://acs-mirror.azureedge.net/containerd/windows/v1.6.21-azure.1/binaries/containerd-v1.6.21-azure.1-windows-amd64.tar.gz"

# Windows Server 2019 update history can be found at https://support.microsoft.com/en-us/help/4464619
# Windows Server 2022 update history can be found at https://support.microsoft.com/en-us/topic/windows-server-2022-update-history-e1caa597-00c5-4ab9-9f3e-8212fe80b2ee
# then you can get download links by searching for specific KBs at http://www.catalog.update.microsoft.com/home.aspx
#
# IMPORTANT NOTES: Please check the KB article before getting the KB links. For example, for 2021-4C:
# You must install the April 22, 2021 servicing stack update (SSU) (KB5001407) before installing the latest cumulative update (LCU).
# SSUs improve the reliability of the update process to mitigate potential issues while installing the LCU.

switch -Regex ($windowsSku) {
    "2019-containerd" {
        $global:patchUrls = @()
        $global:patchIDs = @()

        $global:imagesToPull = @(
            "mcr.microsoft.com/windows/servercore:ltsc2019",
            "mcr.microsoft.com/windows/nanoserver:1809"
        )
    }
    "2022-containerd*" {
        $global:patchUrls = @()
        $global:patchIDs = @()

        $global:imagesToPull = @(
            "mcr.microsoft.com/windows/servercore:ltsc2022",
            "mcr.microsoft.com/windows/nanoserver:ltsc2022",

            # NPM (Network Policy Manager) Owner: jaer-tsun (Jaeryn)
            "mcr.microsoft.com/containernetworking/azure-npm:v1.4.34"
        )
    }
}

$global:imagesToPull += @(
    "mcr.microsoft.com/oss/kubernetes/pause:3.6-hotfix.20220114",
    "mcr.microsoft.com/oss/kubernetes/pause:3.9",
    "mcr.microsoft.com/oss/kubernetes/pause:3.9-hotfix-20230808",
    # This is for test purpose only to reduce the test duration.
    "mcr.microsoft.com/windows/servercore/iis:latest",
    # CSI. Owner: andyzhangx (Andy Zhang)
    "mcr.microsoft.com/oss/kubernetes-csi/livenessprobe:v2.10.0",
    "mcr.microsoft.com/oss/kubernetes-csi/csi-node-driver-registrar:v2.8.0",
    # azuredisk-csi:v1.28 is only for AKS 1.27+, v1.26 is for other AKS versions
    "mcr.microsoft.com/oss/kubernetes-csi/azuredisk-csi:v1.26.5",
    "mcr.microsoft.com/oss/kubernetes-csi/azuredisk-csi:v1.26.6",
    "mcr.microsoft.com/oss/kubernetes-csi/azuredisk-csi:v1.28.1-windows-hp",
    "mcr.microsoft.com/oss/kubernetes-csi/azuredisk-csi:v1.28.2-windows-hp",
    # azurefile-csi:v1.28 is only for AKS 1.27+, v1.24, v1.26 is for other AKS versions
    "mcr.microsoft.com/oss/kubernetes-csi/azurefile-csi:v1.24.5",
    "mcr.microsoft.com/oss/kubernetes-csi/azurefile-csi:v1.24.6",
    "mcr.microsoft.com/oss/kubernetes-csi/azurefile-csi:v1.26.4",
    "mcr.microsoft.com/oss/kubernetes-csi/azurefile-csi:v1.26.5",
    "mcr.microsoft.com/oss/kubernetes-csi/azurefile-csi:v1.28.1-windows-hp",
    "mcr.microsoft.com/oss/kubernetes-csi/azurefile-csi:v1.28.2-windows-hp",
    # Addon of Azure secrets store. Owner: ZeroMagic (Ji'an Liu)
    "mcr.microsoft.com/oss/kubernetes-csi/secrets-store/driver:v1.3.4",
    "mcr.microsoft.com/oss/azure/secrets-store/provider-azure:v1.4.1",
    # Azure cloud node manager. Owner: nilo19 (Qi Ni)
    "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v1.24.18", # for k8s 1.24.x
    "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v1.24.20", # for k8s 1.24.x
    "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v1.24.21", # for k8s 1.24.x
    "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v1.25.12", # for k8s 1.25.x
    "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v1.25.14", # for k8s 1.25.x
    "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v1.25.15", # for k8s 1.25.x
    "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v1.26.8", # for k8s 1.26.x
    "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v1.26.10", # for k8s 1.26.x
    "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v1.26.11", # for k8s 1.26.x
    "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v1.27.4", # for k8s 1.27.x
    "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v1.27.5", # for k8s 1.27.x
    # OMS-Agent (Azure monitor). Owner: ganga1980 (Ganga Mahesh Siddem)
    "mcr.microsoft.com/azuremonitor/containerinsights/ciprod:win-3.1.12",
    # CNS (Container Networking Service) Owner: jaer-tsun (Jaeryn)
    "mcr.microsoft.com/containernetworking/azure-cns:v1.4.44.3",
    "mcr.microsoft.com/containernetworking/azure-cns:v1.4.44.4",
    "mcr.microsoft.com/containernetworking/azure-cns:v1.5.5"
)

$global:map = @{
    "c:\akse-cache\"              = @(
        "https://github.com/Azure/AgentBaker/raw/master/staging/cse/windows/debug/collect-windows-logs.ps1",
        # Please also update staging/cse/windows/debug/update-debug-scripts.ps1 before we remove below scripts from SDN repo
        "https://github.com/microsoft/SDN/raw/d9eaf8f330b9c8119c792ba3768bcf4c2da86123/Kubernetes/windows/debug/collectlogs.ps1",
        "https://github.com/microsoft/SDN/raw/d9eaf8f330b9c8119c792ba3768bcf4c2da86123/Kubernetes/windows/debug/dumpVfpPolicies.ps1",
        "https://github.com/microsoft/SDN/raw/d9eaf8f330b9c8119c792ba3768bcf4c2da86123/Kubernetes/windows/debug/portReservationTest.ps1",
        "https://github.com/microsoft/SDN/raw/d9eaf8f330b9c8119c792ba3768bcf4c2da86123/Kubernetes/windows/debug/starthnstrace.cmd",
        "https://github.com/microsoft/SDN/raw/d9eaf8f330b9c8119c792ba3768bcf4c2da86123/Kubernetes/windows/debug/startpacketcapture.cmd",
        "https://github.com/microsoft/SDN/raw/d9eaf8f330b9c8119c792ba3768bcf4c2da86123/Kubernetes/windows/debug/stoppacketcapture.cmd",
        "https://github.com/microsoft/SDN/raw/d9eaf8f330b9c8119c792ba3768bcf4c2da86123/Kubernetes/windows/debug/starthnstrace.ps1",
        "https://github.com/microsoft/SDN/raw/d9eaf8f330b9c8119c792ba3768bcf4c2da86123/Kubernetes/windows/debug/startpacketcapture.ps1",
        "https://github.com/microsoft/SDN/raw/d9eaf8f330b9c8119c792ba3768bcf4c2da86123/Kubernetes/windows/debug/VFP.psm1",
        "https://github.com/microsoft/SDN/raw/d9eaf8f330b9c8119c792ba3768bcf4c2da86123/Kubernetes/windows/debug/networkmonitor/networkhealth.ps1",
        "https://github.com/microsoft/SDN/raw/d9eaf8f330b9c8119c792ba3768bcf4c2da86123/Kubernetes/windows/helper.psm1",
        "https://github.com/microsoft/SDN/raw/d9eaf8f330b9c8119c792ba3768bcf4c2da86123/Kubernetes/windows/hns.psm1",
        "https://github.com/microsoft/SDN/raw/d9eaf8f330b9c8119c792ba3768bcf4c2da86123/Kubernetes/windows/hns.v2.psm1",
        "https://globalcdn.nuget.org/packages/microsoft.applicationinsights.2.11.0.nupkg",
        "https://acs-mirror.azureedge.net/ccgakvplugin/v1.1.5/binaries/windows-gmsa-ccgakvplugin-v1.1.5.zip",
        "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v0.0.25.zip",
        "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v0.0.26.zip",
        "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v0.0.27.zip",
        "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v0.0.29.zip"
    );
    # Different from other packages which are downloaded/cached and used later only during CSE, windows containerd is installed
    # during building the Windows VHD to cache container images.
    # We use the latest containerd package to start containerd then cache images, and the latest one is expected to be
    # specified by AKS PR for most of the cases. BUT as long as there's a new unpacked image version, we should keep the
    # versions synced.
    "c:\akse-cache\containerd\"   = @(
        $defaultContainerdPackageUrl,
        "https://acs-mirror.azureedge.net/containerd/windows/v1.7.1-azure.1/binaries/containerd-v1.7.1-azure.1-windows-amd64.tar.gz"
    );
    "c:\akse-cache\csi-proxy\"    = @(
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
        "https://acs-mirror.azureedge.net/kubernetes/v1.23.12-hotfix.20220922/windowszip/v1.23.12-hotfix.20220922-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.23.15-hotfix.20230114/windowszip/v1.23.15-hotfix.20230114-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.24.6-hotfix.20221006/windowszip/v1.24.6-hotfix.20221006-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.24.9-hotfix.20230612/windowszip/v1.24.9-hotfix.20230612-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.24.10-hotfix.20230612/windowszip/v1.24.10-hotfix.20230612-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.24.15/windowszip/v1.24.15-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.25.4/windowszip/v1.25.4-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.25.5-hotfix.20230612/windowszip/v1.25.5-hotfix.20230612-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.25.6-hotfix.20230612/windowszip/v1.25.6-hotfix.20230612-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.25.11/windowszip/v1.25.11-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.26.0-hotfix.20230612/windowszip/v1.26.0-hotfix.20230612-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.26.3-hotfix.20230612/windowszip/v1.26.3-hotfix.20230612-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.26.6/windowszip/v1.26.6-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.27.1/windowszip/v1.27.1-1int.zip"
        "https://acs-mirror.azureedge.net/kubernetes/v1.27.1-hotfix.20230612/windowszip/v1.27.1-hotfix.20230612-1int.zip"
        "https://acs-mirror.azureedge.net/kubernetes/v1.27.3/windowszip/v1.27.3-1int.zip"
        "https://acs-mirror.azureedge.net/kubernetes/v1.28.0/windowszip/v1.28.0-1int.zip"       
    );
    "c:\akse-cache\win-vnet-cni\" = @(
        # Azure CNI v1 (legacy) upgrading from v1.4.35 to v1.5.6
        "https://acs-mirror.azureedge.net/azure-cni/v1.4.35/binaries/azure-vnet-cni-singletenancy-windows-amd64-v1.4.35.zip",
        "https://acs-mirror.azureedge.net/azure-cni/v1.5.6/binaries/azure-vnet-cni-singletenancy-windows-amd64-v1.5.6.zip",
        # Azure CNI v2 (pod subnet) upgrading from v1.4.35 to v1.5.5
        "https://acs-mirror.azureedge.net/azure-cni/v1.4.35/binaries/azure-vnet-cni-singletenancy-swift-windows-amd64-v1.4.35.zip",
        "https://acs-mirror.azureedge.net/azure-cni/v1.5.5/binaries/azure-vnet-cni-singletenancy-swift-windows-amd64-v1.5.5.zip",
        # Azure CNI for Overlay upgrading from v1.4.35_Win2019OverlayFix to v1.5.5
        "https://acs-mirror.azureedge.net/azure-cni/v1.4.35_Win2019OverlayFix/binaries/azure-vnet-cni-singletenancy-overlay-windows-amd64-v1.4.35_Win2019OverlayFix.zip",
        "https://acs-mirror.azureedge.net/azure-cni/v1.5.5/binaries/azure-vnet-cni-singletenancy-overlay-windows-amd64-v1.5.5.zip"
    );
    "c:\akse-cache\calico\" = @(
        "https://acs-mirror.azureedge.net/calico-node/v3.21.6/binaries/calico-windows-v3.21.6.zip",
        "https://acs-mirror.azureedge.net/calico-node/v3.24.0/binaries/calico-windows-v3.24.0.zip"
    )
}
'@
# Both configure-windows-vhd.ps1 and windows-vhd-content-test.ps1 will import c:\windows-vhd-configuration.ps1
$windowsConfig | Out-File -FilePath c:\windows-vhd-configuration.ps1
