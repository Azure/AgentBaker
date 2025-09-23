package e2e

import (
	"context"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
)

func Test_Ubuntu2404_NvidiaDevicePluginRunning(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that NVIDIA device plugin is running properly after CSE execution on Ubuntu 24.04 GPU nodes",
		Tags: Tags{
			GPU: true, // This test requires actual GPU hardware
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
				// Validate that the NVIDIA device plugin daemonset is running
				ValidateNvidiaDevicePluginDaemonSetRunning(ctx, s)
				
				// Validate that GPU resources are advertised by the device plugin
				ValidateNodeAdvertisesGPUResources(ctx, s)
				
				// Validate that the NVIDIA device plugin binary was installed correctly
				ValidateNvidiaDevicePluginBinaryInstalled(ctx, s)
				
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
			GPU: true, // This test requires actual GPU hardware
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
				// Validate that the NVIDIA device plugin daemonset is running
				ValidateNvidiaDevicePluginDaemonSetRunning(ctx, s)
				
				// Validate that GPU resources are advertised by the device plugin
				ValidateNodeAdvertisesGPUResources(ctx, s)
				
				// Validate that the NVIDIA device plugin binary was installed correctly
				ValidateNvidiaDevicePluginBinaryInstalled(ctx, s)
				
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
			GPU: true, // This test requires actual GPU hardware
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
				// Validate that the NVIDIA device plugin daemonset is running
				ValidateNvidiaDevicePluginDaemonSetRunning(ctx, s)
				
				// Validate that GPU resources are advertised by the device plugin
				ValidateNodeAdvertisesGPUResources(ctx, s)
				
				// Validate that the NVIDIA device plugin binary was installed correctly
				ValidateNvidiaDevicePluginBinaryInstalled(ctx, s)
				
				// Validate that GPU workloads can be scheduled
				ValidateGPUWorkloadSchedulable(ctx, s)
			},
		},
	})
}