package scenario

import (
	"encoding/json"
	"errors"
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
		},
		LogCheck: func() error {
			data, err := os.ReadFile("scenario-logs/ubuntu2204ContainerdURL/cluster-provision.log")
			if err != nil {
				return err
			}

			if !strings.Contains(string(data), "Succeeded to install containerd from user input") {
				return errors.New("downloadContainerdFromURL was not reached")
			}

			if strings.Contains(string(data), "Succeeded to download containerd version") {
				return errors.New("downloadContainerdFromVersion was reached when it should not have been")
			}

			return nil
		},
	}
}

type Manifest struct {
	Containerd struct {
		Edge string `json:"edge"`
	} `json:"containerd"`
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
		},
		LogCheck: func() error {
			data, err := os.ReadFile("scenario-logs/ubuntu2204ContainerdVersion/cluster-provision.log")
			if err != nil {
				return err
			}

			if strings.Contains(string(data), "Succeeded to install containerd from user input") {
				return errors.New("downloadContainerdFromURL was reached when it should not have been")
			}

			// This will never get hit because harded coded is only used when the manifest is not present
			if strings.Contains(string(data), "Succeeded to download containerd version") {
				return errors.New("downloadContainerdFromVersion was reached. manifest.json should be present but does not appear to be")
			}

			manifestData, err := os.ReadFile("../parts/linux/cloud-init/artifacts/manifest.json")
			if err != nil {
				return err
			}
			manifestDataStr := string(manifestData)
			manifestDataStr = strings.TrimRight(manifestDataStr, "#EOF \n\r\t")
			manifestData = []byte(manifestDataStr)

			var manifest Manifest
			err = json.Unmarshal([]byte(manifestData), &manifest)
			if err != nil {
				return err
			}

			manifestContainerdVersion := manifest.Containerd.Edge
			if !strings.Contains(string(data), "containerd_version="+manifestContainerdVersion) {
				return errors.New("containerd version was not set to the manifest version")
			}

			return nil
		},
	}
}
