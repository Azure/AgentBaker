package scenario

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

func (t *Template) azurelinuxv2Wasm() *Scenario {
	return &Scenario{
		Name:        "azurelinuxv2-wasm",
		Description: "tests that a new AzureLinuxV2 (CgroupV2) node using krustlet can be properly bootstrapped",
		Config: Config{
			ClusterSelector: NetworkPluginKubenetSelector,
			ClusterMutator:  NetworkPluginKubenetMutator,
			VHDSelector:     t.AzureLinuxV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].WorkloadRuntime = datamodel.WasmWasi
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-azurelinux-v2-gen2"
				nbc.AgentPoolProfile.WorkloadRuntime = datamodel.WasmWasi
				nbc.AgentPoolProfile.Distro = "aks-azurelinux-v2-gen2"
			},
		},
	}
}
