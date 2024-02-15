package scenario

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

func (t *Template) ubuntu2204ContainerdURL() *Scenario {
	return &Scenario{
		Name:        "ubuntu2204ContainerdURL",
		Description: "Tests that a node using the Ubuntu 2204 VHD with a containerd URL can be properly bootstrapped",
		Config: Config{
			ClusterSelector: NetworkPluginKubenetSelector,
			ClusterMutator:  NetworkPluginKubenetMutator,
			VHDSelector:     t.Ubuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.AgentPoolProfile.Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.ContainerdPackageURL = "https://packages.microsoft.com/ubuntu/22.04/prod/pool/main/m/moby-containerd/moby-containerd_1.7.13-ubuntu22.04u1_amd64.deb "
			},
			LiveVMValidators: []*LiveVMValidator{
				containerdVersionValidator("1.7.13"),
			},
		},
	}
}

func (t *Template) ubuntu2204ContainerdVersion() *Scenario {
	return &Scenario{
		Name:        "ubuntu2204ContainerdVersion",
		Description: "Tests that a node using the Ubuntu 2204 VHD with a containerd version can be properly bootstrapped",
		Config: Config{
			ClusterSelector: NetworkPluginKubenetSelector,
			ClusterMutator:  NetworkPluginKubenetMutator,
			VHDSelector:     t.Ubuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.AgentPoolProfile.Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.ContainerdVersion = "1.7.13"
			},
			LiveVMValidators: []*LiveVMValidator{
				containerdVersionValidator(getCurrentManifestContainerdVersion()),
			},
		},
	}
}

type Manifest struct {
	Containerd struct {
		Edge string `json:"edge"`
	} `json:"containerd"`
}

func getCurrentManifestContainerdVersion() string {
	manifestData, err := os.ReadFile("../parts/linux/cloud-init/artifacts/manifest.json")
	if err != nil {
		return ""
	}
	manifestDataStr := string(manifestData)
	manifestDataStr = strings.TrimRight(manifestDataStr, "#EOF \n\r\t")
	manifestData = []byte(manifestDataStr)

	var manifest Manifest
	err = json.Unmarshal([]byte(manifestData), &manifest)
	if err != nil {
		return ""
	}
	return manifest.Containerd.Edge
}
