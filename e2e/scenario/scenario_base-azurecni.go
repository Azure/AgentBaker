package scenario

import (
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

const (
	networkPluginAzure = "azure"
)

func base_azurecni() *Scenario {
	return &Scenario{
		Name:        "base-azurecni",
		Description: "base scenario on cluster configured with NetworkPlugin 'Azure'",
		Config: Config{
			ClusterSelector: NetworkPluginAzureSelector,
			ClusterMutator:  NetworkPluginAzureMutator,
			BootstrapConfigMutator: func(t *testing.T, nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = networkPluginAzure
				nbc.AgentPoolProfile.KubernetesConfig.NetworkPlugin = networkPluginAzure
			},
		},
	}
}
