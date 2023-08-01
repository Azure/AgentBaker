package scenario

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
)

func calico() *Scenario {
	return &Scenario{
		Name:        "calico",
		Description: "Test an Ubuntu 22.04 node configured for Calico NPM",
		// This is the same as ubuntu2204, except use a larger VM SKU and set netpol Calico.
		Config: Config{
			ClusterSelector: NetworkPluginKubenetSelector,
			ClusterMutator:  NetworkPluginKubenetMutator,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.AgentPoolProfile.Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.AgentPoolProfile.KubernetesConfig.NetworkPolicy = "calico"
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.Properties.VirtualMachineProfile.StorageProfile.ImageReference = &armcompute.ImageReference{
					ID: to.Ptr(DefaultImageVersionIDs["ubuntu2204"]),
				}
				// TODO: when I change this I get an error
				//     "message": "The selected VM size 'Standard_D4_v2' cannot boot Hypervisor Generation '2'. If this was a Create operation please check that the Hypervisor Generation of the Image matches the Hypervisor Generation of the selected VM Size. If this was an Update operation please select a Hypervisor Generation '2' VM Size. For more information, see https://aka.ms/azuregen2vm"
				//vmss.SKU.Name = to.Ptr("Standard_D4_v3")
			},
		},
	}
}
