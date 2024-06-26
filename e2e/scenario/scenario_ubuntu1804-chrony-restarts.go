package scenario

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

func (t *Template) ubuntu1804ChronyRestarts() *Scenario {
	return &Scenario{
		Name:        "ubuntu1804-chrony-restarts",
		Description: "Tests that the chrony service restarts if it is killed",
		Config: Config{
			ClusterSelector: NetworkPluginKubenetSelector,
			ClusterMutator:  NetworkPluginKubenetMutator,
			VHD:             t.Ubuntu1804Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-containerd-18.04-gen2"
				nbc.AgentPoolProfile.Distro = "aks-ubuntu-containerd-18.04-gen2"
			},
			LiveVMValidators: []*LiveVMValidator{
				serviceCanRestartValidator("chronyd", 10),
				FileHasContentsValidator("/etc/systemd/system/chrony.service.d/10-chrony-restarts.conf", "Restart=always"),
				FileHasContentsValidator("/etc/systemd/system/chrony.service.d/10-chrony-restarts.conf", "RestartSec=5"),
			},
		},
	}
}
