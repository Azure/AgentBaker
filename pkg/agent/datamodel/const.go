// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package datamodel

// the orchestrators supported by vlabs.
const (
	// Kubernetes is the string constant for the Kubernetes orchestrator type.
	Kubernetes string = "Kubernetes"
)

const (
	// KubernetesWindowsDockerVersion is the default version for docker on Windows nodes in kubernetes.
	KubernetesWindowsDockerVersion = "20.10.9"
	// KubernetesDefaultWindowsSku is the default SKU for Windows VMs in kubernetes.
	KubernetesDefaultWindowsSku = "Datacenter-Core-1809-with-Containers-smalldisk"
	// KubernetesDefaultContainerdWindowsSandboxIsolation is the default containerd handler for windows pods.
	KubernetesDefaultContainerdWindowsSandboxIsolation = "process"
)

// Availability profiles.
const (
	// AvailabilitySet means that the vms are in an availability set.
	AvailabilitySet = "AvailabilitySet"
	/* DefaultOrchestratorName specifies the 3 character orchestrator code of the cluster template and affects.
	resource naming. */
	DefaultOrchestratorName = "k8s"
	/* DefaultHostedProfileMasterName specifies the 3 character orchestrator code of the clusters with hosted.
	master profiles. */
	DefaultHostedProfileMasterName = "aks"
	// DefaultSubnetNameResourceSegmentIndex specifies the default subnet name resource segment index.
	DefaultSubnetNameResourceSegmentIndex = 10
	// DefaultVnetResourceGroupSegmentIndex specifies the default virtual network resource segment index.
	DefaultVnetResourceGroupSegmentIndex = 4
	// DefaultVnetNameResourceSegmentIndex specifies the default virtual network name segment index.
	DefaultVnetNameResourceSegmentIndex = 8
	// VirtualMachineScaleSets means that the vms are in a virtual machine scaleset.
	VirtualMachineScaleSets = "VirtualMachineScaleSets"
	// ScaleSetPrioritySpot means the ScaleSet will use Spot VMs.
	ScaleSetPrioritySpot = "Spot"
)

// Supported container runtimes.
const (
	Docker         = "docker"
	KataContainers = "kata-containers"
	Containerd     = "containerd"
)

// storage profiles.
const (
	// ManagedDisks means that the nodes use managed disks for their os and attached volumes.
	ManagedDisks = "ManagedDisks"
)

const (
	// NetworkPluginAzure is the string expression for Azure CNI plugin.
	NetworkPluginAzure = "azure"
	// NetworkPluginNone is the string expression for no CNI plugin.
	NetworkPluginNone = "none"
	// VMSSVMType is the string const for the vmss VM Type.
	VMSSVMType = "vmss"
	// StandardVMType is the string const for the standard VM Type.
	StandardVMType = "standard"
)

const (
	// DefaultEnableCSIProxyWindows determines if CSI proxy should be enabled by default for Windows nodes.
	DefaultEnableCSIProxyWindows = false
	// DefaultWindowsSSHEnabled is the default windowsProfile.sshEnabled value.
	DefaultWindowsSSHEnabled = true
	// DefaultWindowsSecureTLSEnabled is the default windowsProfile.WindowsSecureTlsEnabled value.
	DefaultWindowsSecureTLSEnabled = false
)

const (
	// AzurePublicCloud is a const string reference identifier for public cloud.
	AzurePublicCloud = "AzurePublicCloud"
	// AzureChinaCloud is a const string reference identifier for china cloud.
	AzureChinaCloud = "AzureChinaCloud"
	// AzureGermanCloud is a const string reference identifier for german cloud.
	AzureGermanCloud = "AzureGermanCloud"
	// AzureUSGovernmentCloud is a const string reference identifier for us government cloud.
	AzureUSGovernmentCloud = "AzureUSGovernmentCloud"
	// AzureStackCloud is a const string reference identifier for Azure Stack cloud.
	AzureStackCloud = "AzureStackCloud"
)

const (
	// AzureADIdentitySystem is a const string reference identifier for Azure AD identity System.
	AzureADIdentitySystem = "azure_ad"
)

// Known container runtime configuration keys.
const (
	ContainerDataDirKey = "dataDir"
)

const (
	// KubernetesDefaultRelease is the default Kubernetes release.
	KubernetesDefaultRelease string = "1.13"
	// KubernetesDefaultReleaseWindows is the default Kubernetes release.
	KubernetesDefaultReleaseWindows string = "1.14"
)

// Addon name consts.
const (
	// IPMASQAgentAddonName is the name of the ip masq agent addon.
	IPMASQAgentAddonName = "ip-masq-agent"
	// AADPodIdentityAddonName is the name of the aad-pod-identity addon deployment.
	AADPodIdentityAddonName = "aad-pod-identity"
)

const (
	/* TempDiskContainerDataDir is the path used to mount docker images, emptyDir volumes, and kubelet data
	when KubeletDiskType == TempDisk. */
	TempDiskContainerDataDir = "/mnt/aks/containers"
)

const (
	Nvidia470CudaDriverVersion = "cuda-470.82.01"
	Nvidia510CudaDriverVersion = "cuda-510.47.03"
	Nvidia525CudaDriverVersion = "cuda-525.85.12"
	Nvidia510GridDriverVersion = "grid-510.73.08"
	Nvidia535GridDriverVersion = "grid-535.54.03"
)

// These SHAs will change once we update aks-gpu images in aks-gpu repository. We do that fairly rarely at this time
// So for now these will be kept here like this.
const (
	AKSGPUGridSHA = "sha-20ffa2"
	AKSGPUCudaSHA = "sha-16fd35"
)

/* ConvergedGPUDriverSizes : these sizes use a "converged" driver to support both cuda/grid workloads.
how do you figure this out? ask HPC or find out by trial and error.
installing vanilla cuda drivers will fail to install with opaque errors.
nvidia-bug-report.sh may be helpful, but usually it tells you the pci card id is incompatible.
That sends me to HPC folks.
see https://github.com/Azure/azhpc-extensions/blob/daaefd78df6f27012caf30f3b54c3bd6dc437652/NvidiaGPU/resources.json
*/
//nolint:gochecknoglobals
var ConvergedGPUDriverSizes = map[string]bool{
	"standard_nv6ads_a10_v5":   true,
	"standard_nv12ads_a10_v5":  true,
	"standard_nv18ads_a10_v5":  true,
	"standard_nv36ads_a10_v5":  true,
	"standard_nv72ads_a10_v5":  true,
	"standard_nv36adms_a10_v5": true,
	"standard_nc8ads_a10_v4":   true,
	"standard_nc16ads_a10_v4":  true,
	"standard_nc32ads_a10_v4":  true,
}

/*
FabricManagerGPUSizes list should be updated as needed if AKS supports
new MIG-capable skus which require fabricmanager for nvlink training.
Specifically, the 8-board VM sizes (ND96 and larger).
Check with HPC or SKU API folks if we can improve this...
*/
//nolint:gochecknoglobals
var FabricManagerGPUSizes = map[string]bool{
	// A100
	"standard_nd96asr_v4":        true,
	"standard_nd112asr_a100_v4":  true,
	"standard_nd120asr_a100_v4":  true,
	"standard_nd96amsr_a100_v4":  true,
	"standard_nd112amsr_a100_v4": true,
	"standard_nd120amsr_a100_v4": true,
	// TODO(ace): one of these is probably dupe...
	// confirm with HPC/SKU owners.
	"standard_nd96ams_a100_v4": true,
	"standard_nd96ams_v4":      true,
	// H100.
	"standard_nd46s_h100_v5":    true,
	"standard_nd48s_h100_v5":    true,
	"standard_nd50s_h100_v5":    true,
	"standard_nd92is_h100_v5":   true,
	"standard_nd96is_h100_v5":   true,
	"standard_nd100is_h100_v5":  true,
	"standard_nd92isr_h100_v5":  true,
	"standard_nd96isr_h100_v5":  true,
	"standard_nd100isr_h100_v5": true,
	// A100 oddballs.
	"standard_nc24ads_a100_v4": false, // NCads_v4 will fail to start fabricmanager.
	"standard_nc48ads_a100_v4": false,
	"standard_nc96ads_a100_v4": false,
}

const (
	OSSKUCBLMariner = "CBLMariner"
	OSSKUMariner    = "Mariner"
	OSSKUAzureLinux = "AzureLinux"
)

// Feature Flags.
const (
	BlockOutboundInternet = "BlockOutboundInternet"
	CSERunInBackground    = "CSERunInBackground"
	EnableIPv6DualStack   = "EnableIPv6DualStack"
	EnableIPv6Only        = "EnableIPv6Only"
	EnableWinDSR          = "EnableWinDSR"
)
