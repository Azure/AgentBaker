package scenario

import (
	"fmt"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
)

// Returns config for the 'gpu' E2E scenario
func (t *Template) ubuntu2204gpu(vmSeries string) *Scenario {
	vmSize := DefaultGPUSeriesVMSizes[vmSeries]
	return &Scenario{
		Name:        fmt.Sprintf("ubuntu2204-gpu-%s", vmSeries),
		Description: fmt.Sprintf("Tests that a GPU-enabled node with VM size %s using an Ubuntu 2204 VHD can be properly bootstrapped", vmSize),
		Config: Config{
			ClusterSelector: NetworkPluginKubenetSelector,
			ClusterMutator:  NetworkPluginKubenetMutator,
			VHDSelector:     t.Ubuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize = vmSize
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.AgentPoolProfile.VMSize = vmSize
				nbc.AgentPoolProfile.Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = false
				nbc.EnableNvidia = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr(vmSize)
			},
		},
	}
}
