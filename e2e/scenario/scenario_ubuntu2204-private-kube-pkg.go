package scenario

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

func (t *Template) ubuntu2204privatekubepkg() *Scenario {
	return &Scenario{
		Name:        "ubuntu2204privatekubepkg",
		Description: "Tests that a node using the Ubuntu 2204 VHD that was built with private kube packages can be properly bootstrapped with the specified kube version",
		Config: Config{
			ClusterSelector: NetworkPluginKubenetSelector,
			ClusterMutator:  NetworkPluginKubenetMutator,
			VHDSelector:     t.Ubuntu2204Gen2ContainerdPrivateKubePkg,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = "1.25.6"
				nbc.AgentPoolProfile.Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.K8sComponents.LinuxPrivatePackageURL = "https://privatekube.blob.core.windows.net/kubernetes/v1.25.6-hotfix.20230612/binaries/v1.25.6-hotfix.20230612.tar.gz"
			},
		},
	}
}
