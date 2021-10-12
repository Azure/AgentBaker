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
$validSKU = @("2019", "2019-containerd")
if (-not ($validSKU -contains $windowsSKU)) {
    throw "Unsupported windows image SKU: $windowsSKU"
}

# Windows Server 2019 update history can be found at https://support.microsoft.com/en-us/help/4464619
# then you can get download links by searching for specific KBs at http://www.catalog.update.microsoft.com/home.aspx
#
# IMPORTANT NOTES: Please check the KB article before getting the KB links. For example, for 2021-4C:
# You must install the April 22, 2021 servicing stack update (SSU) (KB5001407) before installing the latest cumulative update (LCU).
# SSUs improve the reliability of the update process to mitigate potential issues while installing the LCU.
$global:patchUrls = @("http://download.windowsupdate.com/c/msdownload/update/software/updt/2021/09/windows10.0-kb5005625-x64_9a7d6abe389d940e08d759243c981670c33c71f5.msu")
$global:patchIDs = @("KB5005625")

# defaultContainerdPackageUrl refers to the latest containerd package used to pull and cache container images
$global:defaultContainerdPackageUrl = "https://acs-mirror.azureedge.net/containerd/windows/v0.0.42/binaries/containerd-v0.0.42-windows-amd64.tar.gz"

$global:defaultDockerVersion = "20.10.6"

switch ($windowsSKU) {
    "2019" {
        $global:imagesToPull = @(
            "mcr.microsoft.com/windows/servercore:ltsc2019",
            "mcr.microsoft.com/windows/nanoserver:1809",
            "mcr.microsoft.com/oss/kubernetes/pause:3.4.1",
            "mcr.microsoft.com/oss/kubernetes-csi/livenessprobe:v2.3.0",
            "mcr.microsoft.com/oss/kubernetes-csi/livenessprobe:v2.4.0",
            "mcr.microsoft.com/oss/kubernetes-csi/csi-node-driver-registrar:v2.2.0",
            "mcr.microsoft.com/oss/kubernetes-csi/csi-node-driver-registrar:v2.3.0",
            "mcr.microsoft.com/oss/kubernetes-csi/azuredisk-csi:v1.5.0",
            "mcr.microsoft.com/oss/kubernetes-csi/azuredisk-csi:v1.7.0",
            "mcr.microsoft.com/oss/kubernetes-csi/azurefile-csi:v1.5.0",
            "mcr.microsoft.com/oss/kubernetes-csi/azurefile-csi:v1.6.0",
            "mcr.microsoft.com/oss/kubernetes-csi/secrets-store/driver:v0.0.21",
            "mcr.microsoft.com/oss/azure/secrets-store/provider-azure:0.0.14",
            "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v0.6.0", # for k8s 1.19.x
            "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v0.7.5", # for k8s 1.20.x
            "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v1.0.2", # for k8s 1.21.x
            "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v1.1.1", # for k8s 1.22.x
            "mcr.microsoft.com/azuremonitor/containerinsights/ciprod:win-ciprod06112021-2",
            "mcr.microsoft.com/azuremonitor/containerinsights/ciprod:win-ciprod10112021")
    }
    "2019-containerd" {
        $global:imagesToPull = @(
            "mcr.microsoft.com/windows/servercore:ltsc2019",
            "mcr.microsoft.com/windows/nanoserver:1809",
            "mcr.microsoft.com/oss/kubernetes/pause:3.4.1",
            "mcr.microsoft.com/oss/kubernetes-csi/livenessprobe:v2.3.0",
            "mcr.microsoft.com/oss/kubernetes-csi/livenessprobe:v2.4.0",
            "mcr.microsoft.com/oss/kubernetes-csi/csi-node-driver-registrar:v2.2.0",
            "mcr.microsoft.com/oss/kubernetes-csi/csi-node-driver-registrar:v2.3.0",
            "mcr.microsoft.com/oss/kubernetes-csi/azuredisk-csi:v1.5.0",
            "mcr.microsoft.com/oss/kubernetes-csi/azuredisk-csi:v1.7.0",
            "mcr.microsoft.com/oss/kubernetes-csi/azurefile-csi:v1.5.0",
            "mcr.microsoft.com/oss/kubernetes-csi/azurefile-csi:v1.6.0",
            "mcr.microsoft.com/oss/kubernetes-csi/secrets-store/driver:v0.0.21",
            "mcr.microsoft.com/oss/azure/secrets-store/provider-azure:0.0.14",
            "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v0.7.5", # for k8s 1.20.x
            "mcr.microsoft.com/oss/kubernetes/azure-cloud-node-manager:v1.0.2", # for k8s 1.21.x
            "mcr.microsoft.com/azuremonitor/containerinsights/ciprod:win-ciprod06112021-2",
            "mcr.microsoft.com/azuremonitor/containerinsights/ciprod:win-ciprod10112021")
    }
    default {
        throw "No valid windows SKU is specified $windowsSKU"
    }
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
        "https://github.com/Microsoft/SDN/raw/master/Kubernetes/windows/debug/VFP.psm1",
        "https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/helper.psm1",
        "https://github.com/Microsoft/SDN/raw/master/Kubernetes/windows/hns.psm1",
        "https://globalcdn.nuget.org/packages/microsoft.applicationinsights.2.11.0.nupkg",
        "https://acs-mirror.azureedge.net/aks-engine/windows/provisioning/signedscripts-v0.0.15.zip",
        "https://acs-mirror.azureedge.net/ccgakvplugin/v1.1.3/binaries/windows-gmsa-ccgakvplugin-v1.1.3.zip"
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
        "https://acs-mirror.azureedge.net/csi-proxy/v1.0.0/binaries/csi-proxy-v1.0.0.tar.gz"
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
        "https://acs-mirror.azureedge.net/kubernetes/v1.18.17-hotfix.20210519/windowszip/v1.18.17-hotfix.20210519-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.18.19-hotfix.20210519/windowszip/v1.18.19-hotfix.20210519-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.19.9-hotfix.20210526/windowszip/v1.19.9-hotfix.20210526-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.19.11-hotfix.20210823/windowszip/v1.19.11-hotfix.20210823-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.19.13/windowszip/v1.19.13-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.19.13-hotfix.20210830/windowszip/v1.19.13-hotfix.20210830-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.20.5-hotfix.20210603/windowszip/v1.20.5-hotfix.20210603-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.20.7-hotfix.20210603/windowszip/v1.20.7-hotfix.20210603-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.20.7-hotfix.20210816/windowszip/v1.20.7-hotfix.20210816-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.20.9/windowszip/v1.20.9-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.20.9-hotfix.20210830/windowszip/v1.20.9-hotfix.20210830-1int.zip"
        "https://acs-mirror.azureedge.net/kubernetes/v1.21.1-hotfix.20210827/windowszip/v1.21.1-hotfix.20210827-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.21.2-hotfix.20210830/windowszip/v1.21.2-hotfix.20210830-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.22.1/windowszip/v1.22.1-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.22.2/windowszip/v1.22.2-1int.zip"
    );
    "c:\akse-cache\win-vnet-cni\" = @(
        "https://acs-mirror.azureedge.net/azure-cni/v1.4.9/binaries/azure-vnet-cni-singletenancy-windows-amd64-v1.4.9.zip",
        "https://acs-mirror.azureedge.net/azure-cni/v1.4.12/binaries/azure-vnet-cni-singletenancy-windows-amd64-v1.4.12.zip",
        "https://acs-mirror.azureedge.net/azure-cni/v1.4.13/binaries/azure-vnet-cni-singletenancy-windows-amd64-v1.4.13.zip"
    );
    "c:\akse-cache\calico\" = @(
        "https://acs-mirror.azureedge.net/calico-node/v3.19.0/binaries/calico-windows-v3.19.0.zip",
        "https://acs-mirror.azureedge.net/calico-node/v3.20.0/binaries/calico-windows-v3.20.0.zip"
    )
}
'@
# Both configure-windows-vhd.ps1 and windows-vhd-content-test.ps1 will import c:\windows-vhd-configuration.ps1
$windowsConfig | Out-File -FilePath c:\windows-vhd-configuration.ps1
