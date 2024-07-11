package scenario

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbakere2e/cluster"
	"github.com/Azure/agentbakere2e/config"
)

func ubuntu2204ChronyRestarts() *Scenario {
	return &Scenario{
		Name:        "ubuntu2204-chrony-restarts",
		Description: "Tests that the chrony service restarts if it is killed",
		Tags: Tags{
			Name:     "ubuntu2204-chrony-restarts",
			OS:       "ubuntu2204",
			Platform: "x64",
		},
		Config: Config{
			Cluster:     cluster.ClusterKubenet,
			VHDSelector: config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.AgentPoolProfile.Distro = "aks-ubuntu-containerd-22.04-gen2"
			},
			LiveVMValidators: []*LiveVMValidator{
				serviceCanRestartValidator("chronyd", 10),
				FileHasContentsValidator("/etc/systemd/system/chronyd.service.d/10-chrony-restarts.conf", "Restart=always"),
				FileHasContentsValidator("/etc/systemd/system/chronyd.service.d/10-chrony-restarts.conf", "RestartSec=5"),
			},
		},
	}
}
