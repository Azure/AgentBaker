package e2e

import (
	"context"
	"testing"

	"github.com/Azure/agentbaker/e2e/components"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/stretchr/testify/require"
)

func Test_Ubuntu2404_NvidiaDevicePluginRunning(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that NVIDIA device plugin is running properly after CSE execution on Ubuntu 24.04 GPU nodes",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2404Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = "Standard_NC6s_v3"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = true
				nbc.EnableNvidia = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NC6s_v3")
			},
			Validator: func(ctx context.Context, s *Scenario) {

				// Validate that the NVIDIA device plugin binary was installed correctly
				versions := components.GetExpectedPackageVersions("nvidia-device-plugin", "ubuntu", "r2404")
				require.Lenf(s.T, versions, 1, "Expected exactly one nvidia-device-plugin version for ubuntu r2404 but got %d", len(versions))
				ValidateInstalledPackageVersion(ctx, s, "nvidia-device-plugin", versions[0])

				// Validate that the NVIDIA device plugin systemd service is running
				ValidateNvidiaDevicePluginServiceRunning(ctx, s)

				// Validate that GPU resources are advertised by the device plugin
				ValidateNodeAdvertisesGPUResources(ctx, s)

				// Validate that GPU workloads can be scheduled
				ValidateGPUWorkloadSchedulable(ctx, s)
			},
		},
	})
}

func Test_Ubuntu2204_NvidiaDevicePluginRunning(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that NVIDIA device plugin is running properly after CSE execution on Ubuntu 22.04 GPU nodes",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = "Standard_NC6s_v3"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = true
				nbc.EnableNvidia = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NC6s_v3")
			},
			Validator: func(ctx context.Context, s *Scenario) {

				// Validate that the NVIDIA device plugin binary was installed correctly
				versions := components.GetExpectedPackageVersions("nvidia-device-plugin", "ubuntu", "r2204")
				require.Lenf(s.T, versions, 1, "Expected exactly one nvidia-device-plugin version for ubuntu r2204 but got %d", len(versions))
				ValidateInstalledPackageVersion(ctx, s, "nvidia-device-plugin", versions[0])

				// Validate that the NVIDIA device plugin systemd service is running
				ValidateNvidiaDevicePluginServiceRunning(ctx, s)

				// Validate that GPU resources are advertised by the device plugin
				ValidateNodeAdvertisesGPUResources(ctx, s)

				// Validate that GPU workloads can be scheduled
				ValidateGPUWorkloadSchedulable(ctx, s)
			},
		},
	})
}

func Test_AzureLinux3_NvidiaDevicePluginRunning(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that NVIDIA device plugin is running properly after CSE execution on Azure Linux V3 GPU nodes",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV3Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = "Standard_NC6s_v3"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = true
				nbc.EnableNvidia = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NC6s_v3")
			},
			Validator: func(ctx context.Context, s *Scenario) {

				// Validate that the NVIDIA device plugin binary was installed correctly
				versions := components.GetExpectedPackageVersions("nvidia-device-plugin", "azurelinux", "v3.0")
				require.Lenf(s.T, versions, 1, "Expected exactly one nvidia-device-plugin version for azurelinux 3.0 but got %d", len(versions))
				ValidateInstalledPackageVersion(ctx, s, "nvidia-device-plugin", versions[0])

				// Validate that the NVIDIA device plugin systemd service is running
				ValidateNvidiaDevicePluginServiceRunning(ctx, s)

				// Validate that GPU resources are advertised by the device plugin
				ValidateNodeAdvertisesGPUResources(ctx, s)

				// Validate that GPU workloads can be scheduled
				ValidateGPUWorkloadSchedulable(ctx, s)
			},
		},
	})
}
