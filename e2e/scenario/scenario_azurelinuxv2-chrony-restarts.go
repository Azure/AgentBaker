package scenario

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbakere2e/cluster"
	"github.com/Azure/agentbakere2e/config"
)

func azurelinuxv2ChronyRestarts() *Scenario {
	return &Scenario{
		Name:        "azurelinuxv2-chrony-restarts",
		Description: "Tests that the chrony service restarts if it is killed",
		Tags: Tags{
			Name:     "azurelinuxv2-chrony-restarts",
			OS:       "azurelinuxv2",
			Platform: "x64",
		},
		Config: Config{
			Cluster:     cluster.ClusterKubenet,
			VHDSelector: config.VHDAzureLinuxV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-azurelinux-v2-gen2"
				nbc.AgentPoolProfile.Distro = "aks-azurelinux-v2-gen2"
			},
			LiveVMValidators: []*LiveVMValidator{
				serviceCanRestartValidator("chronyd", 10),
				FileHasContentsValidator("/etc/systemd/system/chronyd.service.d/10-chrony-restarts.conf", "Restart=always"),
				FileHasContentsValidator("/etc/systemd/system/chronyd.service.d/10-chrony-restarts.conf", "RestartSec=5"),
			},
		},
	}
}
