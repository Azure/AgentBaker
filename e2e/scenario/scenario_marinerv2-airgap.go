package scenario

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbakere2e/config"
)

func marinerv2AirGap() *Scenario {
	return &Scenario{
		Name:        "marinerv2-airgap",
		Description: "Tests that a node using a MarinerV2 VHD can be properly bootstrapped",
		Config: Config{
			ClusterSelector: NetworkPluginKubenetSelector,
			ClusterMutator:  NetworkPluginKubenetMutator,
			VHDSelector:     config.VHDCBLMarinerV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-cblmariner-v2-gen2"
				nbc.AgentPoolProfile.Distro = "aks-cblmariner-v2-gen2"
			},
			Airgap: true,
		},
	}
}
