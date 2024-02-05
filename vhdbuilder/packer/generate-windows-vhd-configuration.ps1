# MUST define global variable with "global"
# This script is used to generate shared configuration for configure-windows-vhd.ps1 and windows-vhd-content-test.ps1.
# MUST NOT add any shared functions in this script.
$windowsConfig = @'
$global:windowsSKU = $env:WindowsSKU
$validSKU = @("2019-containerd", "2022-containerd", "2022-containerd-gen2", "23H2", "23H2-gen2")
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

# defenderUpdateUrl refers to the latest windows defender platform update
$global:defenderUpdateUrl = "https://go.microsoft.com/fwlink/?linkid=870379&arch=x64"
# defenderUpdateInfoUrl refers to the info of latest windows defender platform update
$global:defenderUpdateInfoUrl = "https://go.microsoft.com/fwlink/?linkid=870379&arch=x64&action=info"

switch -Regex ($windowsSku) {
    "2019-containerd" {
        $global:patchUrls = @("https://catalog.s.download.windowsupdate.com/c/msdownload/update/software/secu/2024/01/windows10.0-kb5034127-x64_7ac59afbc502ef9dc77f7584b31110ea0b23a566.msu")
        $global:patchIDs = @("KB5034127")

        $global:imagesToPull = @(
            "mcr.microsoft.com/windows/servercore:ltsc2019",
            "mcr.microsoft.com/windows/nanoserver:1809"
        )
    }
    "2022-containerd*" {
        $global:patchUrls = @("https://catalog.s.download.windowsupdate.com/d/msdownload/update/software/secu/2024/01/windows10.0-kb5034129-x64_15b9faad91f6b35ead3fbfb0fdc00f86244adc7f.msu")
        $global:patchIDs = @("KB5034129")

        $global:imagesToPull = @(
            "mcr.microsoft.com/windows/servercore:ltsc2022",
            "mcr.microsoft.com/windows/nanoserver:ltsc2022",

            # NPM (Network Policy Manager) Owner: jaer-tsun (Jaeryn)
            "mcr.microsoft.com/containernetworking/azure-npm:v1.4.34"
        )
    }
    "23H2*" {
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
    "mcr.microsoft.com/oss/kubernetes/pause:3.9-hotfix-20230808",
    # This is for test purpose only to reduce the test duration.
    "mcr.microsoft.com/windows/servercore/iis:latest",
    # CSI. Owner: andyzhangx (Andy Zhang)
    "mcr.microsoft.com/oss/kubernetes-csi/livenessprobe:v2.10.0", # for k8s 1.25.x, 1.26.x, 1.27.x
    "mcr.microsoft.com/oss/kubernetes-csi/livenessprobe:v2.11.0", # for k8s 1.28.x
    "mcr.microsoft.com/oss/kubernetes-csi/livenessprobe:v2.12.0", # for k8s 1.29.x
    "mcr.microsoft.com/oss/kubernetes-csi/csi-node-driver-registrar:v2.8.0", # for k8s 1.25.x, 1.26.x, 1.27.x
    "mcr.microsoft.com/oss/kubernetes-csi/csi-node-driver-registrar:v2.9.0", # for k8s 1.28.x
    "mcr.microsoft.com/oss/kubernetes-csi/csi-node-driver-registrar:v2.10.0", # for k8s 1.29.x
    "mcr.microsoft.com/oss/kubernetes-csi/azuredisk-csi:v1.26.7", # for k8s 1.25.x, 1.26.x
    "mcr.microsoft.com/oss/kubernetes-csi/azuredisk-csi:v1.26.8", # for k8s 1.25.x, 1.26.x
    "mcr.microsoft.com/oss/kubernetes-csi/azuredisk-csi:v1.26.9", # for k8s 1.25.x, 1.26.x
    "mcr.microsoft.com/oss/kubernetes-csi/azuredisk-csi:v1.28.4.1-windows-hp", # for k8s 1.27.x
    "mcr.microsoft.com/oss/kubernetes-csi/azuredisk-csi:v1.28.5-windows-hp", # for k8s 1.27.x
    "mcr.microsoft.com/oss/kubernetes-csi/azuredisk-csi:v1.28.6-windows-hp", # for k8s 1.27.x
    "mcr.microsoft.com/oss/kubernetes-csi/azuredisk-csi:v1.29.1.1-windows-hp", # for k8s 1.28.x
    "mcr.microsoft.com/oss/kubernetes-csi/azuredisk-csi:v1.29.2-windows-hp", # for k8s 1.28.x
    "mcr.microsoft.com/oss/kubernetes-csi/azuredisk-csi:v1.29.3-windows-hp", # for k8s 1.28.x
    "mcr.microsoft.com/oss/kubernetes-csi/azurefile-csi:v1.24.11", # for k8s 1.25.x
    "mcr.microsoft.com/oss/kubernetes-csi/azurefile-csi:v1.26.9", # for k8s 1.26.x
    "mcr.microsoft.com/oss/kubernetes-csi/azurefile-csi:v1.26.10", # for k8s 1.26.x
    "mcr.microsoft.com/oss/kubernetes-csi/azurefile-csi:v1.26.11", # for k8s 1.26.x
    "mcr.microsoft.com/oss/kubernetes-csi/azurefile-csi:v1.28.6-windows-hp", # for k8s 1.27.x
    "mcr.microsoft.com/oss/kubernetes-csi/azurefile-csi:v1.28.7-windows-hp", # for k8s 1.27.x
    "mcr.microsoft.com/oss/kubernetes-csi/azurefile-csi:v1.28.8-windows-hp", # for k8s 1.27.x
    "mcr.microsoft.com/oss/kubernetes-csi/azurefile-csi:v1.29.1-windows-hp", # for k8s 1.28.x
    "mcr.microsoft.com/oss/kubernetes-csi/azurefile-csi:v1.29.2-windows-hp", # for k8s 1.28.x
    "mcr.microsoft.com/oss/kubernetes-csi/azurefile-csi:v1.29.3-windows-hp", # for k8s 1.28.x
    # Addon of Azure secrets store. Owner: jiashun0011 (Jiashun Liu)
    "mcr.microsoft.com/oss/kubernetes-csi/secrets-store/driver:v1.3.4",
    "mcr.microsoft.com/oss/kubernetes-csi/secrets-store/driver:v1.4.0",
    "mcr.microsoft.com/oss/azure/secrets-store/provider-azure:v1.4.1",
    "mcr.microsoft.com/oss/azure/secrets-store/provider-azure:v1.5.0",
    # Azure cloud node manager. Owner: nilo19 (Qi Ni)
    "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v1.25.18", # for k8s 1.25.x
    "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v1.25.24", # for k8s 1.25.x
    "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v1.26.14", # for k8s 1.26.x
    "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v1.26.19", # for k8s 1.26.x
    "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v1.27.8", # for k8s 1.27.x
    "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v1.27.13", # for k8s 1.27.x
    "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v1.28.0", # for k8s 1.28.x
    "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v1.28.5", # for k8s 1.28.x
    "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v1.29.0", # for k8s 1.29.x
    # OMS-Agent (Azure monitor). Owner: ganga1980 (Ganga Mahesh Siddem)
    "mcr.microsoft.com/azuremonitor/containerinsights/ciprod:win-3.1.17",
    # CNS (Container Networking Service) Owner: jaer-tsun (Jaeryn)
    "mcr.microsoft.com/containernetworking/azure-cns:v1.4.52",
    "mcr.microsoft.com/containernetworking/azure-cns:v1.4.50",
    "mcr.microsoft.com/containernetworking/azure-cns:v1.5.17",
    "mcr.microsoft.com/containernetworking/azure-cns:v1.5.11"
    # Dropgz (init container to CNS). Owner: pjohnst5 (Paul Johnston)
    "mcr.microsoft.com/containernetworking/cni-dropgz:v0.0.13"
)

$global:map = @{
    "c:\akse-cache\"              = @(
        "https://acs-mirror.azureedge.net/ccgakvplugin/v1.1.5/binaries/windows-gmsa-ccgakvplugin-v1.1.5.zip",
        "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v0.0.33.zip",
        "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v0.0.36.zip",
        "https://acs-mirror.azureedge.net/aks/windows/cse/aks-windows-cse-scripts-v0.0.37.zip"
    );
    # Different from other packages which are downloaded/cached and used later only during CSE, windows containerd is installed
    # during building the Windows VHD to cache container images.
    # We use the latest containerd package to start containerd then cache images, and the latest one is expected to be
    # specified by AKS PR for most of the cases. BUT as long as there's a new unpacked image version, we should keep the
    # versions synced.
    "c:\akse-cache\containerd\"   = @(
        $defaultContainerdPackageUrl,
        "https://acs-mirror.azureedge.net/containerd/windows/v1.7.1-azure.1/binaries/containerd-v1.7.1-azure.1-windows-amd64.tar.gz",
        "https://acs-mirror.azureedge.net/containerd/windows/v1.7.9-azure.1/binaries/containerd-v1.7.9-azure.1-windows-amd64.tar.gz"
    );
    "c:\akse-cache\csi-proxy\"    = @(
        "https://acs-mirror.azureedge.net/csi-proxy/v1.1.2-hotfix.20230807/binaries/csi-proxy-v1.1.2-hotfix.20230807.tar.gz"
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
        "https://acs-mirror.azureedge.net/kubernetes/v1.25.11-hotfix.20231102/windowszip/v1.25.11-hotfix.20231102-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.25.15-hotfix.20231103/windowszip/v1.25.15-hotfix.20231103-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.26.3-hotfix.20231102/windowszip/v1.26.3-hotfix.20231102-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.26.6-hotfix.20231102/windowszip/v1.26.6-hotfix.20231102-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.26.10-hotfix.20231103/windowszip/v1.26.10-hotfix.20231103-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.26.12/windowszip/v1.26.12-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.27.1-hotfix.20231102/windowszip/v1.27.1-hotfix.20231102-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.27.3-hotfix.20231102/windowszip/v1.27.3-hotfix.20231102-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.27.7-hotfix.20231103/windowszip/v1.27.7-hotfix.20231103-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.27.9/windowszip/v1.27.9-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.28.0-hotfix.20231102/windowszip/v1.28.0-hotfix.20231102-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.28.3-hotfix.20231103/windowszip/v1.28.3-hotfix.20231103-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.28.5/windowszip/v1.28.5-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.29.0/windowszip/v1.29.0-1int.zip"
    );
    "c:\akse-cache\win-vnet-cni\" = @(
        # Azure CNI v1 (legacy)
        "https://acs-mirror.azureedge.net/azure-cni/v1.5.6.1/binaries/azure-vnet-cni-singletenancy-windows-amd64-v1.5.6.1.zip",
        # Azure CNI v2 (pod subnet) upgrading from v1.4.39.1 (unsigned) to v1.4.39.2 (signed)
        "https://acs-mirror.azureedge.net/azure-cni/v1.4.39.2/binaries/azure-vnet-cni-singletenancy-swift-windows-amd64-v1.4.39.2.zip",
        # Azure CNI for Overlay upgrading from v1.4.39.1 (unsigned) to v1.4.39.2 (signed)
        "https://acs-mirror.azureedge.net/azure-cni/v1.4.39.2/binaries/azure-vnet-cni-singletenancy-overlay-windows-amd64-v1.4.39.2.zip"
    );
    "c:\akse-cache\calico\" = @(
        "https://acs-mirror.azureedge.net/calico-node/v3.21.6/binaries/calico-windows-v3.21.6.zip",
        "https://acs-mirror.azureedge.net/calico-node/v3.24.0/binaries/calico-windows-v3.24.0.zip"
    );
    "c:\akse-cache\tools\" = @(
        "https://download.sysinternals.com/files/DU.zip"
    )
}
'@
# Both configure-windows-vhd.ps1 and windows-vhd-content-test.ps1 will import c:\windows-vhd-configuration.ps1
$windowsConfig | Out-File -FilePath c:\windows-vhd-configuration.ps1
