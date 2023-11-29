package scenario

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
)

func (t *Template) azurelinuxv2_azurecni() *Scenario {
	return &Scenario{
		Name:        "azurelinuxv2-azurecni",
		Description: "azurelinuxv2 scenario on a cluster configured with Azure CNI",
		Config: Config{
			ClusterSelector: NetworkPluginAzureSelector,
			ClusterMutator:  NetworkPluginAzureMutator,
			VHDSelector:     t.AzureLinuxV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = string(armcontainerservice.NetworkPluginAzure)
				nbc.AgentPoolProfile.KubernetesConfig.NetworkPlugin = string(armcontainerservice.NetworkPluginAzure)
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-azurelinux-v2-gen2"
				nbc.AgentPoolProfile.Distro = "aks-azurelinux-v2-gen2"
			},
		},
	}
}
