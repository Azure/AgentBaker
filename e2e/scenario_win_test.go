package e2e

import (
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbakere2e/config"
)

func Test_WindowsServer2019Containerd(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2019 with Containerd",
		Config: Config{
			Cluster: ClusterAzureNetwork,
			VHD:     config.VHDWindows2019Containerd,
			BootstrapConfigMutator: func(configuration *datamodel.NodeBootstrappingConfiguration) {

			},
		},
	})
}

func Test_WindowsServer2022Containerd(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2022 with Containerd",
		Config: Config{
			Cluster: ClusterAzureNetwork,
			VHD:     config.VHDWindows2022Containerd,
			BootstrapConfigMutator: func(configuration *datamodel.NodeBootstrappingConfiguration) {

			},
		},
	})
}

func Test_Windows2022ContainerdGen2(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2022 with Containerd",
		Config: Config{
			Cluster: ClusterAzureNetwork,
			VHD:     config.VHDWindows2022ContainerdGen2,
			BootstrapConfigMutator: func(configuration *datamodel.NodeBootstrappingConfiguration) {

			},
		},
	})
}

func Test_Windows23H2(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2022 with Containerd",
		Config: Config{
			Cluster: ClusterAzureNetwork,
			VHD:     config.VHDWindows23H2,
			BootstrapConfigMutator: func(configuration *datamodel.NodeBootstrappingConfiguration) {

			},
		},
	})
}

func Test_Windows23H2Gen2(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2022 with Containerd",
		Config: Config{
			Cluster: ClusterAzureNetwork,
			VHD:     config.VHDWindows23H2Gen2,
			BootstrapConfigMutator: func(configuration *datamodel.NodeBootstrappingConfiguration) {

			},
		},
	})
}
