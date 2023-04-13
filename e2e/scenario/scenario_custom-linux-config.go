package scenario

import (
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

func customLinuxConfig() *Scenario {
	return &Scenario{
		Name:        "custom-node-config",
		Description: "tests that an ubuntu 1804 node can be properly bootstrapped with custom linux config",
		Config: Config{
			ClusterSelector: NetworkPluginKubenetSelector,
			ClusterMutator:  NetworkPluginKubenetMutator,
			BootstrapConfigMutator: func(t *testing.T, nbc *datamodel.NodeBootstrappingConfiguration) {
				customLinuxConfig := &datamodel.CustomLinuxOSConfig{}
				nbc.AgentPoolProfile.CustomLinuxOSConfig = customLinuxConfig
				nbc.ContainerService.Properties.AgentPoolProfiles[0].CustomLinuxOSConfig = customLinuxConfig
			},
		},
	}
}
