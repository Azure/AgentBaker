package scenario

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

func (t *Template) azurelinuxv2AirGap() *Scenario {
	return &Scenario{
		Name:        "azurelinuxv2-airgap",
		Description: "Tests that a node using a AzureLinuxV2 (CgroupV2) VHD can be properly bootstrapped",
		Config: Config{
			ClusterSelector: NetworkPluginKubenetSelector,
			ClusterMutator:  NetworkPluginKubenetMutator,
			VHDSelector:     t.AzureLinuxV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-azurelinux-v2-gen2"
				nbc.AgentPoolProfile.Distro = "aks-azurelinux-v2-gen2"
			},
			Airgap: true,
		},
	}
}
