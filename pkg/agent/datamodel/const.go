// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package datamodel

// the orchestrators supported by vlabs
const (
	// Kubernetes is the string constant for the Kubernetes orchestrator type
	Kubernetes string = "Kubernetes"
)

const (
	// KubernetesWindowsDockerVersion is the default version for docker on Windows nodes in kubernetes
	KubernetesWindowsDockerVersion = "19.03.11"
	// KubernetesDefaultWindowsSku is the default SKU for Windows VMs in kubernetes
	KubernetesDefaultWindowsSku = "Datacenter-Core-1809-with-Containers-smalldisk"
	// KubernetesDefaultWindowsRuntimeHandler is the default containerd handler for windows pods
	KubernetesDefaultWindowsRuntimeHandler = "process"
)

// Availability profiles
const (
	// AvailabilitySet means that the vms are in an availability set
	AvailabilitySet = "AvailabilitySet"
	// DefaultOrchestratorName specifies the 3 character orchestrator code of the cluster template and affects resource naming.
	DefaultOrchestratorName = "k8s"
	// DefaultHostedProfileMasterName specifies the 3 character orchestrator code of the clusters with hosted master profiles.
	DefaultHostedProfileMasterName = "aks"
	// DefaultSubnetNameResourceSegmentIndex specifies the default subnet name resource segment index.
	DefaultSubnetNameResourceSegmentIndex = 10
	// DefaultVnetResourceGroupSegmentIndex specifies the default virtual network resource segment index.
	DefaultVnetResourceGroupSegmentIndex = 4
	// DefaultVnetNameResourceSegmentIndex specifies the default virtual network name segment index.
	DefaultVnetNameResourceSegmentIndex = 8
	// VirtualMachineScaleSets means that the vms are in a virtual machine scaleset
	VirtualMachineScaleSets = "VirtualMachineScaleSets"
	// ScaleSetPrioritySpot means the ScaleSet will use Spot VMs
	ScaleSetPrioritySpot = "Spot"
)

// Supported container runtimes
const (
	Docker         = "docker"
	KataContainers = "kata-containers"
	Containerd     = "containerd"
)

// storage profiles
const (
	// ManagedDisks means that the nodes use managed disks for their os and attached volumes
	ManagedDisks = "ManagedDisks"
)

const (
	// NetworkPluginAzure is the string expression for Azure CNI plugin.
	NetworkPluginAzure = "azure"
	// VMSSVMType is the string const for the vmss VM Type
	VMSSVMType = "vmss"
	// StandardVMType is the string const for the standard VM Type
	StandardVMType = "standard"
)

// Azure API Versions
const (
	APIVersionNetwork = "2018-08-01"
)

const (
	// DefaultEnableCSIProxyWindows determines if CSI proxy should be enabled by default for Windows nodes
	DefaultEnableCSIProxyWindows = false
	// DefaultWindowsSSHEnabled is the default windowsProfile.sshEnabled value
	DefaultWindowsSSHEnabled = true
)

const (
	// AzureChinaCloud is a const string reference identifier for china cloud
	AzureChinaCloud = "AzureChinaCloud"
	// AzureStackCloud is a const string reference identifier for Azure Stack cloud
	AzureStackCloud = "AzureStackCloud"
)

const (
	// AzureADIdentitySystem is a const string reference identifier for Azure AD identity System
	AzureADIdentitySystem = "azure_ad"
)

// Known container runtime configuration keys
const (
	ContainerDataDirKey = "dataDir"
)

const (
	// KubernetesDefaultRelease is the default Kubernetes release
	KubernetesDefaultRelease string = "1.13"
	// KubernetesDefaultReleaseWindows is the default Kubernetes release
	KubernetesDefaultReleaseWindows string = "1.14"
)

// Addon name consts
const (
	// IPMASQAgentAddonName is the name of the ip masq agent addon
	IPMASQAgentAddonName = "ip-masq-agent"
	// AADPodIdentityAddonName is the name of the aad-pod-identity addon deployment
	AADPodIdentityAddonName = "aad-pod-identity"
)
