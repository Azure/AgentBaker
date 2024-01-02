package scenario

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
)

func (t *Template) ubuntu1804_azurecni() *Scenario {
	return &Scenario{
		Name:        "ubuntu1804-azurecni",
		Description: "ubuntu1804 scenario on cluster configured with Azure CNI",
		Config: Config{
			ClusterSelector: NetworkPluginAzureSelector,
			ClusterMutator:  NetworkPluginAzureMutator,
			VHDSelector:     t.Ubuntu1804Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = string(armcontainerservice.NetworkPluginAzure)
				nbc.AgentPoolProfile.KubernetesConfig.NetworkPlugin = string(armcontainerservice.NetworkPluginAzure)
			},
		},
	}
}
