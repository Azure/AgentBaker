package scenario

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
)

func marinerv1_azurecni() *Scenario {
	return &Scenario{
		Name:        "marinerv1-azurecni",
		Description: "marinerv1 scenario on a cluster configured with Azure CNI",
		Config: Config{
			ClusterSelector: NetworkPluginAzureSelector,
			ClusterMutator:  NetworkPluginAzureMutator,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = string(armcontainerservice.NetworkPluginAzure)
				nbc.AgentPoolProfile.KubernetesConfig.NetworkPlugin = string(armcontainerservice.NetworkPluginAzure)
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-cblmariner-v1"
				nbc.AgentPoolProfile.Distro = "aks-cblmariner-v1"
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.Properties.VirtualMachineProfile.StorageProfile.ImageReference = &armcompute.ImageReference{
					ID: to.Ptr(DefaultImageVersionIDs["marinerv1"]),
				}
			},
		},
	}
}
