package e2e

import (
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbakere2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
)

func Test_Windows2019Containerd(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2019 with Containerd",
		Config: Config{
			Cluster: ClusterAzureNetwork,
			VHD:     config.VHDWindows2019Containerd,
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_D2_v2")
			},
			BootstrapConfigMutator: func(configuration *datamodel.NodeBootstrappingConfiguration) {

			},
		},
	})
}

func Test_Windows2022Containerd(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2022 with Containerd",
		Config: Config{
			Cluster: ClusterAzureNetwork,
			VHD:     config.VHDWindows2022Containerd,
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_D2_v2")
			},
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
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_D2ds_v5")
			},
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
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_D2_v2")
			},
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
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_D2ds_v5")
			},
			BootstrapConfigMutator: func(configuration *datamodel.NodeBootstrappingConfiguration) {

			},
		},
	})
}
