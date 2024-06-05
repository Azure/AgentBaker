package scenario

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

func (t *Template) marinerv2SystemdChronyDropin() *Scenario {
	return &Scenario{
		Name:        "marinerv2-systemd-dropin-for-chrony",
		Description: "Tests that the systemd drop-in file for chrony is placed in the right place",
		Config: Config{
			ClusterSelector: NetworkPluginKubenetSelector,
			ClusterMutator:  NetworkPluginKubenetMutator,
			VHDSelector:     t.CBLMarinerV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-cblmariner-v2-gen2"
				nbc.AgentPoolProfile.Distro = "aks-cblmariner-v2-gen2"
			},
			LiveVMValidators: []*LiveVMValidator{
				FileHasContentsValidator("/etc/systemd/system/chronyd.service.d/10-chrony-restarts.conf", "Restart=always"),
				FileHasContentsValidator("/etc/systemd/system/chronyd.service.d/10-chrony-restarts.conf", "RestartSec=5"),
			},
		},
	}
}

func (t *Template) marinerv2ChronyRestarts() *Scenario {
	return &Scenario{
		Name:        "marinerv2-chrony-restarts",
		Description: "Tests that the chrony service restarts if it is killed",
		Config: Config{
			ClusterSelector: NetworkPluginKubenetSelector,
			ClusterMutator:  NetworkPluginKubenetMutator,
			VHDSelector:     t.CBLMarinerV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-cblmariner-v2-gen2"
				nbc.AgentPoolProfile.Distro = "aks-cblmariner-v2-gen2"
			},
			LiveVMValidators: []*LiveVMValidator{
				serviceCanRestartValidator("chronyd", 10),
			},
		},
	}
}
