package scenario

import (
	"github.com/Azure/agentbakere2e/cluster"
	"github.com/Azure/agentbakere2e/config"
)

// Returns config for the 'base' E2E scenario
func ubuntu1804() *Scenario {
	return &Scenario{
		Name:        "ubuntu1804",
		Description: "Tests that a node using an Ubuntu 1804 VHD can be properly bootstrapped",
		Tags: Tags{
			Name:     "ubuntu1804",
			OS:       "ubuntu1804",
			Platform: "x64",
		},
		Config: Config{
			Cluster:     cluster.ClusterKubenet,
			VHDSelector: config.VHDUbuntu1804Gen2Containerd,
		},
	}
}
