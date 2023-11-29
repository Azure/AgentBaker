package scenario

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

func (t *Template) ubuntu2204Wasm() *Scenario {
	return &Scenario{
		Name:        "ubuntu2204-wasm",
		Description: "tests that a new ubuntu 2204 node using krustlet can be properly bootstrapepd",
		Config: Config{
			ClusterSelector: NetworkPluginKubenetSelector,
			ClusterMutator:  NetworkPluginKubenetMutator,
			VHDSelector:     t.Ubuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].WorkloadRuntime = datamodel.WasmWasi
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.AgentPoolProfile.WorkloadRuntime = datamodel.WasmWasi
				nbc.AgentPoolProfile.Distro = "aks-ubuntu-containerd-22.04-gen2"
			},
		},
	}
}
