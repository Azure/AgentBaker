$global:windowsSKU = $env:WindowsSKU
$validSKU = @("2019-containerd", "2022-containerd", "2022-containerd-gen2", "23H2", "23H2-gen2")
if (-not ($validSKU -contains $windowsSKU)) {
    throw "Unsupported windows image SKU: $windowsSKU"
}

# We use the same temp dir for all temp tools that will be used for vhd build
$global:aksTempDir = "c:\akstemp"

# We use the same dir for all tools that will be used in AKS Windows nodes
$global:aksToolsDir = "c:\aks-tools"

# We need to guarantee that the node provisioning will not fail because the vhd is full before resize-osdisk is called in AKS Windows CSE script.
$global:lowestFreeSpace = 1*1024*1024*1024 # 1GB

$global:excludeHashComparisionListInAzureChinaCloud = @(
    "calico-windows",
    "azure-vnet-cni-singletenancy-windows-amd64",
    "azure-vnet-cni-singletenancy-swift-windows-amd64",
    "azure-vnet-cni-singletenancy-overlay-windows-amd64",
    # We need upstream's help to republish this package. Before that, it does not impact functionality and 1.26 is only in public preview
    # so we can ignore the different hash values.
    "v1.26.0-1int.zip",
    "azure-acr-credential-provider-windows-amd64-v1.29.2.tar.gz"
)

# defaultContainerdPackageUrl refers to the stable containerd package used to pull and cache container images
# Add cache for another containerd version which is not installed by default
$global:defaultContainerdPackageUrl = "https://acs-mirror.azureedge.net/containerd/windows/v1.6.35-azure.1/binaries/containerd-v1.6.35-azure.1-windows-amd64.tar.gz"
if ($windowsSKU -Like "23H2*") {
    $global:defaultContainerdPackageUrl = "https://acs-mirror.azureedge.net/containerd/windows/v1.7.20-azure.1/binaries/containerd-v1.7.20-azure.1-windows-amd64.tar.gz"
}

# Windows Server 2019 update history can be found at https://support.microsoft.com/en-us/help/4464619
# Windows Server 2022 update history can be found at https://support.microsoft.com/en-us/topic/windows-server-2022-update-history-e1caa597-00c5-4ab9-9f3e-8212fe80b2ee
# Windows Server 23H2 update history can be found at https://support.microsoft.com/en-us/topic/windows-server-version-23h2-update-history-68c851ff-825a-4dbc-857b-51c5aa0ab248
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
        $global:patchUrls = @()
        $global:patchIDs = @()
    }
    "2022-containerd*" {
        $global:patchUrls = @()
        $global:patchIDs = @()
    }
    "23H2*" {
        $global:patchUrls = @()
        $global:patchIDs = @()
    }
}


$HelpersFile = "c:/build/components_json_helpers.ps1"
$ComponentsJsonFile = "c:/build/components.json"

# fallback in case we're running in a test pipeline or locally
if (!(Test-Path $HelpersFile)) {
    $HelpersFile = "vhdbuilder/packer/windows/components_json_helpers.ps1"
}

if (!(Test-Path $ComponentsJsonFile)) {
    $ComponentsJsonFile = "parts/linux/cloud-init/artifacts/components.json"
}

Write-Output "Components JSON: $ComponentsJsonFile"
Write-Output "Helpers Ps1: $HelpersFile"

. "$HelpersFile"

$componentsJson = Get-Content $ComponentsJsonFile | Out-String | ConvertFrom-Json
$global:imagesToPull = GetComponentsFromComponentsJson $componentsJson

$global:map = @{
    # Different from other packages which are downloaded/cached and used later only during CSE, windows containerd is installed
    # during building the Windows VHD to cache container images.
    # We use the latest containerd package to start containerd then cache images, and the latest one is expected to be
    # specified by AKS PR for most of the cases. BUT as long as there's a new unpacked image version, we should keep the
    # versions synced.
    "c:\akse-cache\containerd\"   = @(
        $defaultContainerdPackageUrl,
        "https://acs-mirror.azureedge.net/containerd/windows/v1.7.17-azure.1/binaries/containerd-v1.7.17-azure.1-windows-amd64.tar.gz",
        "https://acs-mirror.azureedge.net/containerd/windows/v1.7.20-azure.1/binaries/containerd-v1.7.20-azure.1-windows-amd64.tar.gz"
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
        "https://acs-mirror.azureedge.net/kubernetes/v1.27.101-akslts/windowszip/v1.27.101-akslts-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.27.102-akslts/windowszip/v1.27.102-akslts-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.28.15/windowszip/v1.28.15-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.28.100-akslts/windowszip/v1.28.100-akslts-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.29.11/windowszip/v1.29.11-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.29.12/windowszip/v1.29.12-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.29.13/windowszip/v1.29.13-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.30.7/windowszip/v1.30.7-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.30.8/windowszip/v1.30.8-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.30.9/windowszip/v1.30.9-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.31.3/windowszip/v1.31.3-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.31.4/windowszip/v1.31.4-1int.zip",
        "https://acs-mirror.azureedge.net/kubernetes/v1.31.5/windowszip/v1.31.5-1int.zip"
    );
    "c:\akse-cache\win-vnet-cni\" = @(
        # Azure CNI v1 (legacy)
        "https://acs-mirror.azureedge.net/azure-cni/v1.5.38/binaries/azure-vnet-cni-windows-amd64-v1.5.38.zip",
        "https://acs-mirror.azureedge.net/azure-cni/v1.6.18/binaries/azure-vnet-cni-windows-amd64-v1.6.18.zip",
        # Azure CNI v2 (pod subnet)
        "https://acs-mirror.azureedge.net/azure-cni/v1.4.58/binaries/azure-vnet-cni-swift-windows-amd64-v1.4.58.zip",
        "https://acs-mirror.azureedge.net/azure-cni/v1.4.59/binaries/azure-vnet-cni-swift-windows-amd64-v1.4.59.zip",
        # Azure CNI for Overlay
        "https://acs-mirror.azureedge.net/azure-cni/v1.4.58/binaries/azure-vnet-cni-overlay-windows-amd64-v1.4.58.zip",
        "https://acs-mirror.azureedge.net/azure-cni/v1.4.59/binaries/azure-vnet-cni-overlay-windows-amd64-v1.4.59.zip"
    );
    "c:\akse-cache\calico\" = @(
        "https://acs-mirror.azureedge.net/calico-node/v3.24.0/binaries/calico-windows-v3.24.0.zip"
    )
}
