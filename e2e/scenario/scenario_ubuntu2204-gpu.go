package scenario

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
)

// Returns config for the 'gpu' E2E scenario
func ubuntu2204gpu(vmSeries string) *Scenario {
	return &Scenario{
		Name:        "ubuntu2204-gpu-" + vmSeries,
		Description: "Tests that a GPU-enabled node using an Ubuntu 2204 VHD can be properly bootstrapped",
		Config: Config{
			ClusterSelector: NetworkPluginKubenetSelector,
			ClusterMutator:  NetworkPluginKubenetMutator,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize = DefaultGPUSeriesSKU[vmSeries]
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.AgentPoolProfile.VMSize = DefaultGPUSeriesSKU[vmSeries]
				nbc.AgentPoolProfile.Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = false
				nbc.EnableNvidia = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr(DefaultGPUSeriesSKU[vmSeries])
				vmss.Properties.VirtualMachineProfile.StorageProfile.ImageReference = &armcompute.ImageReference{
					ID: to.Ptr(DefaultImageVersionIDs["ubuntu2204"]),
				}
			},
		},
	}
}
