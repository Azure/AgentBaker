package e2e_test

import (
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
)

var cases = map[string]scenarioConfig{
	"base": {},
	"gpu": {
		bootstrapConfigMutator: func(t *testing.T, nbc *datamodel.NodeBootstrappingConfiguration) {
			nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_NC6"
			nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-containerd-18.04-gen2"
			nbc.AgentPoolProfile.VMSize = "Standard_ND96asr_v4"
			nbc.AgentPoolProfile.Distro = "aks-ubuntu-containerd-18.04-gen2"
			nbc.ConfigGPUDriverIfNeeded = true
			nbc.EnableGPUDevicePluginIfNeeded = false
			nbc.EnableNvidia = true
		},
		vmConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
			vmss.SKU.Name = to.Ptr("Standard_NC6s_v3")
		},
	},
}
