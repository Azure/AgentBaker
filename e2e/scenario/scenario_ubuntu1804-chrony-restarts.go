package scenario

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

func (t *Template) ubuntu1804SystemdChronyDropin() *Scenario {
	return &Scenario{
		Name:        "ubuntu1804-systemd-dropin-for-chrony",
		Description: "Tests that the systemd drop-in file for chrony is placed in the right place",
		Config: Config{
			ClusterSelector: NetworkPluginKubenetSelector,
			ClusterMutator:  NetworkPluginKubenetMutator,
			VHDSelector:     t.Ubuntu1804Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-containerd-18.04-gen2"
				nbc.AgentPoolProfile.Distro = "aks-ubuntu-containerd-18.04-gen2"
			},
			LiveVMValidators: []*LiveVMValidator{
				FileHasContentsValidator("/etc/systemd/system/chrony.service.d/10-chrony-restarts.conf", "Restart=always"),
				FileHasContentsValidator("/etc/systemd/system/chrony.service.d/10-chrony-restarts.conf", "RestartSec=5"),
			},
		},
	}
}

func (t *Template) ubuntu1804ChronyRestarts() *Scenario {
	return &Scenario{
		Name:        "ubuntu1804-chrony-restarts",
		Description: "Tests that the chrony service restarts if it is killed",
		Config: Config{
			ClusterSelector: NetworkPluginKubenetSelector,
			ClusterMutator:  NetworkPluginKubenetMutator,
			VHDSelector:     t.Ubuntu1804Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-containerd-18.04-gen2"
				nbc.AgentPoolProfile.Distro = "aks-ubuntu-containerd-18.04-gen2"
			},
			LiveVMValidators: []*LiveVMValidator{
				serviceCanRestartValidator("chronyd", 10),
			},
		},
	}
}
