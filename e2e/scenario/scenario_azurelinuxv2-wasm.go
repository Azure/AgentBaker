package scenario

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbakere2e/config"
)

func azurelinuxv2Wasm() *Scenario {
	return &Scenario{
		Name:        "azurelinuxv2-wasm",
		Description: "tests that a new AzureLinuxV2 (CgroupV2) node using krustlet can be properly bootstrapped",
		Tags: Tags{
			Name:     "azurelinuxv2-wasm",
			OS:       "azurelinuxv2",
			Platform: "x64",
			WASM:     true,
		},
		Config: Config{
			ClusterSelector: NetworkPluginKubenetSelector,
			ClusterMutator:  NetworkPluginKubenetMutator,
			VHDSelector:     config.VHDAzureLinuxV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].WorkloadRuntime = datamodel.WasmWasi
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-azurelinux-v2-gen2"
				nbc.AgentPoolProfile.WorkloadRuntime = datamodel.WasmWasi
				nbc.AgentPoolProfile.Distro = "aks-azurelinux-v2-gen2"
			},
		},
	}
}
