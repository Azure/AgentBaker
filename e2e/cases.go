package e2e_test

import (
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
)

// These SIG image versions are stored in the ACS test subscription, guarded by resource deletion locks
var defaultUbuntuImageVersionIDs = map[string]string{
	"1804gen2":      "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/1804Gen2/versions/1.1677169694.31375",
	"2004gen2fips":  "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/2004Gen2/versions/1.1679939587.10044",
	"2204gen2":      "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/2204Gen2/versions/1.1679939578.12283",
	"2204gen2arm64": "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/2204Gen2Arm64/versions/1.1679939579.29526",
}

var defaultMarinerImageVersionIDs = map[string]string{
	"v1":          "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/CBLMarinerV1/versions/1.1679939595.17588",
	"v2gen2":      "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/CBLMarinerV2Gen2/versions/1.1679939582.10768",
	"v2gen2arm64": "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/CBLMarinerV2Gen2Arm64/versions/1.1679939588.23459",
	"v2gen2kata":  "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/CBLMarinerV2kataGen2/versions/1.1679939567.7755",
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
				ID: to.Ptr(defaultUbuntuImageVersionIDs["2204gen2"]),
			}
		},
	},
	"marinerv1": {
		bootstrapConfigMutator: func(t *testing.T, nbc *datamodel.NodeBootstrappingConfiguration) {
			nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-cblmariner-v1"
			nbc.AgentPoolProfile.Distro = "aks-cblmariner-v1"
		},
		vmConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
			vmss.Properties.VirtualMachineProfile.StorageProfile.ImageReference = &armcompute.ImageReference{
				ID: to.Ptr(defaultMarinerImageVersionIDs["v1"]),
			}
		},
	},
	"marinerv2": {
		bootstrapConfigMutator: func(t *testing.T, nbc *datamodel.NodeBootstrappingConfiguration) {
			nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-cblmariner-v2-gen2"
			nbc.AgentPoolProfile.Distro = "aks-cblmariner-v2-gen2"
		},
		vmConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
			vmss.Properties.VirtualMachineProfile.StorageProfile.ImageReference = &armcompute.ImageReference{
				ID: to.Ptr(defaultMarinerImageVersionIDs["v2gen2"]),
			}
		},
	},
	"ubuntu2204-arm64": {
		bootstrapConfigMutator: func(t *testing.T, nbc *datamodel.NodeBootstrappingConfiguration) {
			nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_D2pds_V5"
			nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-arm64-containerd-22.04-gen2"
			// This needs to be set based on current CSE implementation...
			nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.CustomKubeBinaryURL = "https://acs-mirror.azureedge.net/kubernetes/v1.24.9/binaries/kubernetes-node-linux-arm64.tar.gz"
			nbc.AgentPoolProfile.VMSize = "Standard_D2pds_V5"
			nbc.AgentPoolProfile.Distro = "aks-ubuntu-arm64-containerd-22.04-gen2"
			nbc.IsARM64 = true

		},
		vmConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
			vmss.Properties.VirtualMachineProfile.StorageProfile.ImageReference = &armcompute.ImageReference{
				ID: to.Ptr(defaultUbuntuImageVersionIDs["2204gen2arm64"]),
			}
			vmss.SKU.Name = to.Ptr("Standard_D2pds_V5")
		},
	},
	"marinerv2-arm64": {
		bootstrapConfigMutator: func(t *testing.T, nbc *datamodel.NodeBootstrappingConfiguration) {
			nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_D2pds_V5"
			nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-cblmariner-v2-arm64-gen2"
			nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.CustomKubeBinaryURL = "https://acs-mirror.azureedge.net/kubernetes/v1.24.9/binaries/kubernetes-node-linux-arm64.tar.gz"
			nbc.AgentPoolProfile.VMSize = "Standard_D2pds_V5"
			nbc.AgentPoolProfile.Distro = "aks-cblmariner-v2-arm64-gen2"
			nbc.IsARM64 = true
		},
		vmConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
			vmss.Properties.VirtualMachineProfile.StorageProfile.ImageReference = &armcompute.ImageReference{
				ID: to.Ptr(defaultMarinerImageVersionIDs["v2gen2arm64"]),
			}
			vmss.SKU.Name = to.Ptr("Standard_D2pds_V5")
		},
	},
	"marinerv2-kata": {
		bootstrapConfigMutator: func(t *testing.T, nbc *datamodel.NodeBootstrappingConfiguration) {
			nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_D4ads_v5"
			nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-cblmariner-v2-gen2-kata"
			nbc.AgentPoolProfile.VMSize = "Standard_D4ads_v5"
			nbc.AgentPoolProfile.Distro = "aks-cblmariner-v2-gen2-kata"
		},
		vmConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
			vmss.Properties.VirtualMachineProfile.StorageProfile.ImageReference = &armcompute.ImageReference{
				ID: to.Ptr(defaultMarinerImageVersionIDs["v2gen2kata"]),
			}
		},
	},
	"ubuntu2004-fips": {
		bootstrapConfigMutator: func(t *testing.T, nbc *datamodel.NodeBootstrappingConfiguration) {
			nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_DS2_v2"
			nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-fips-containerd-20.04-gen2"
			nbc.AgentPoolProfile.VMSize = "Standard_DS2_v2"
			nbc.AgentPoolProfile.Distro = "aks-ubuntu-fips-containerd-20.04-gen2"
		},
		vmConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
			vmss.Properties.VirtualMachineProfile.StorageProfile.ImageReference = &armcompute.ImageReference{
				ID: to.Ptr(defaultUbuntuImageVersionIDs["2004gen2fips"]),
			}
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
