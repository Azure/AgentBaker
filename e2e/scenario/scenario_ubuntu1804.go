package scenario

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

// Returns config for the 'base' E2E scenario
func (t *Template) ubuntu1804() *Scenario {
	return &Scenario{
		Name:        "ubuntu1804",
		Description: "Tests that a node using an Ubuntu 1804 VHD can be properly bootstrapped",
		Config: Config{
			ClusterSelector: NetworkPluginKubenetSelector,
			ClusterMutator:  NetworkPluginKubenetMutator,
			VHDSelector:     t.Ubuntu1804Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-containerd-18.04"
				nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = "1.28.3"
				nbc.AgentPoolProfile.Distro = "aks-ubuntu-containerd-18.04"
			},
		},
	}
}
