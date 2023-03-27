package e2e_test

import (
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
)

// These SIG image versions are stored in the ACS test subscription, guarded by resource deletion locks
var defaultImageVersionIDs = map[string]string{
	"ubuntu1804":       "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/1804Gen2/versions/1.1677169694.31375",
	"ubuntu2204":       "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/2204Gen2/versions/1.1679939578.12283",
	"marinerv1":        "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/CBLMarinerV1/versions/1.1679939595.17588",
	"marinerv2":        "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/CBLMarinerV2Gen2/versions/1.1679939582.10768",
	"ubuntu2204-arm64": "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/2204Gen2Arm64/versions/1.1679939579.29526",
	"marinerv2-arm64":  "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/CBLMarinerV2Gen2Arm64/versions/1.1679939588.23459",
}

var cases = map[string]scenarioConfig{
	"base": {},
	"ubuntu2204": {
		bootstrapConfigMutator: func(t *testing.T, nbc *datamodel.NodeBootstrappingConfiguration) {
			nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-containerd-22.04-gen2"
			nbc.AgentPoolProfile.Distro = "aks-ubuntu-containerd-22.04-gen2"
		},
		vmConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
			vmss.Properties.VirtualMachineProfile.StorageProfile.ImageReference = &armcompute.ImageReference{
				ID: to.Ptr(defaultImageVersionIDs["ubuntu2204"]),
			}
		},
	},
	"marinerv1": {
		bootstrapConfigMutator: func(t *testing.T, nbc *datamodel.NodeBootstrappingConfiguration) {
			nbc.ContainerService.Properties.AgentPoolProfiles[0].OSType = "CBLMariner"
			nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-cblmariner-v1"
			nbc.AgentPoolProfile.OSType = "CBLMariner"
			nbc.AgentPoolProfile.Distro = "aks-cblmariner-v1"
		},
		vmConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
			vmss.Properties.VirtualMachineProfile.StorageProfile.ImageReference = &armcompute.ImageReference{
				ID: to.Ptr(defaultImageVersionIDs["marinerv1"]),
			}
		},
	},
	"marinerv2": {
		bootstrapConfigMutator: func(t *testing.T, nbc *datamodel.NodeBootstrappingConfiguration) {
			nbc.ContainerService.Properties.AgentPoolProfiles[0].OSType = "Mariner"
			nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-cblmariner-v2-gen2"
			nbc.AgentPoolProfile.OSType = "Mariner"
			nbc.AgentPoolProfile.Distro = "aks-cblmariner-v2-gen2"
		},
		vmConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
			vmss.Properties.VirtualMachineProfile.StorageProfile.ImageReference = &armcompute.ImageReference{
				ID: to.Ptr(defaultImageVersionIDs["marinerv2"]),
			}
		},
	},
	"ubuntu2204-arm64": {
		bootstrapConfigMutator: func(t *testing.T, nbc *datamodel.NodeBootstrappingConfiguration) {
			nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_D2pds_V5"
			nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-arm64-containerd-22.04-gen2"
			nbc.AgentPoolProfile.VMSize = "Standard_D2pds_V5"
			nbc.AgentPoolProfile.Distro = "aks-ubuntu-arm64-containerd-22.04-gen2"
		},
		vmConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
			vmss.Properties.VirtualMachineProfile.StorageProfile.ImageReference = &armcompute.ImageReference{
				ID: to.Ptr(defaultImageVersionIDs["ubuntu2204-arm64"]),
			}
			vmss.SKU.Name = to.Ptr("Standard_D2pds_V5")
		},
	},
	"marinerv2-arm64": {
		bootstrapConfigMutator: func(t *testing.T, nbc *datamodel.NodeBootstrappingConfiguration) {
			nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_D4pds_V5"
			nbc.ContainerService.Properties.AgentPoolProfiles[0].OSType = "Mariner"
			nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-cblmariner-v2-arm64-gen2"
			nbc.AgentPoolProfile.VMSize = "Standard_D4pds_V5"
			nbc.AgentPoolProfile.OSType = "Mariner"
			nbc.AgentPoolProfile.Distro = "aks-cblmariner-v2-arm64-gen2"
		},
		vmConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
			vmss.Properties.VirtualMachineProfile.StorageProfile.ImageReference = &armcompute.ImageReference{
				ID: to.Ptr(defaultImageVersionIDs["marinerv2-arm64"]),
			}
			vmss.SKU.Name = to.Ptr("Standard_D4pds_V5")
		},
	},
	"gpu": {
		bootstrapConfigMutator: func(t *testing.T, nbc *datamodel.NodeBootstrappingConfiguration) {
			nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_NC6"
			nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-containerd-18.04-gen2"
			nbc.AgentPoolProfile.VMSize = "Standard_NC6"
			nbc.AgentPoolProfile.Distro = "aks-ubuntu-containerd-18.04-gen2"
			nbc.ConfigGPUDriverIfNeeded = true
			nbc.EnableGPUDevicePluginIfNeeded = false
			nbc.EnableNvidia = true
		},
		vmConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
			vmss.SKU.Name = to.Ptr("Standard_NC6s_v3")
		},
	},
}
