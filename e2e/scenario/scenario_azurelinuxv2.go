package scenario

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
)

func azurelinuxv2() *Scenario {
	return &Scenario{
		Name:        "azurelinuxv2",
		Description: "Tests that a node using a AzureLinuxV2 (CgroupV2) VHD can be properly bootstrapped",
		Config: Config{
			ClusterSelector: NetworkPluginKubenetSelector,
			ClusterMutator:  NetworkPluginKubenetMutator,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-azurelinux-v2-gen2"
				nbc.AgentPoolProfile.Distro = "aks-azurelinux-v2-gen2"
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.Properties.VirtualMachineProfile.StorageProfile.ImageReference = &armcompute.ImageReference{
					ID: to.Ptr(DefaultImageVersionIDs["azurelinuxv2"]),
				}
			},
		},
	}
}
