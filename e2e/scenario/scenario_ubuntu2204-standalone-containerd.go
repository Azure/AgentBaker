package scenario

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

// These tests were created to verify that the apt-get call in downloadContainerdFromVersion is not executed.
// The code path is not hit in either of these tests. In the future, testing with some kind of firewall to ensure no egress
// calls are made would be beneficial for airgap testing.

func (t *Template) ubuntu2204ContainerdURL() *Scenario {
	return &Scenario{
		Name:        "ubuntu2204ContainerdURL",
		Description: "tests that a node using the Ubuntu 2204 VHD with the ContainerdPackageURL override bootstraps with the provided URL and not the maifest contianerd version",
		Config: Config{
			ClusterSelector: NetworkPluginKubenetSelector,
			ClusterMutator:  NetworkPluginKubenetMutator,
			VHDSelector:     t.Ubuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.AgentPoolProfile.Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.ContainerdPackageURL = "https://packages.microsoft.com/ubuntu/22.04/prod/pool/main/m/moby-containerd/moby-containerd_1.6.9+azure-ubuntu22.04u1_amd64.deb"
			},
			LiveVMValidators: []*LiveVMValidator{
				containerdVersionValidator("1.6.9"),
			},
		},
	}
}

func (t *Template) ubuntu2204ContainerdVersion() *Scenario {
	return &Scenario{
		Name:        "ubuntu2204ContainerdVersion",
		Description: "tests that a node using an Ubuntu2204 VHD and the ContainerdVersion override bootstraps with the correct manifest containerd version and ignores the override",
		Config: Config{
			ClusterSelector: NetworkPluginKubenetSelector,
			ClusterMutator:  NetworkPluginKubenetMutator,
			VHDSelector:     t.Ubuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.AgentPoolProfile.Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.ContainerdVersion = "1.6.9"
			},
			LiveVMValidators: []*LiveVMValidator{
				containerdVersionValidator(getContainerdManifestVersion()),
			},
		},
	}
}

func getContainerdManifestVersion() string {
	manifest, err := getVHDManifest()
	if err != nil {
		panic(err)
	}

	return manifest.Containerd.Edge
}
