package scenario

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbakere2e/config"
)

func marinerv2Wasm() *Scenario {
	return &Scenario{
		Name:        "marinerv2-wasm",
		Description: "tests that a new marinerv2 node using krustlet can be properly bootstrapped",
		Tags: Tags{
			Name:     "marinerv2-wasm",
			OS:       "marinerv2",
			Platform: "x64",
			WASM:     true,
		},
		Config: Config{
			ClusterSelector: NetworkPluginKubenetSelector,
			ClusterMutator:  NetworkPluginKubenetMutator,
			VHDSelector:     config.VHDCBLMarinerV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].WorkloadRuntime = datamodel.WasmWasi
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-cblmariner-v2-gen2"
				nbc.AgentPoolProfile.WorkloadRuntime = datamodel.WasmWasi
				nbc.AgentPoolProfile.Distro = "aks-cblmariner-v2-gen2"
			},
		},
	}
}
