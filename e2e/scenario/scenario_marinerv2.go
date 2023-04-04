package scenario

import (
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
)

func marinerv2() *Scenario {
	return &Scenario{
		Name:                "marinerv2",
		Description:         "Tests that a node using a MarinerV2 VHD can be properly bootstrapped",
		ClusterConfigurator: NewNetworkPluginKubenetConfigurator(),
		ScenarioConfig: ScenarioConfig{
			BootstrapConfigMutator: func(t *testing.T, nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-cblmariner-v2-gen2"
				nbc.AgentPoolProfile.Distro = "aks-cblmariner-v2-gen2"
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.Properties.VirtualMachineProfile.StorageProfile.ImageReference = &armcompute.ImageReference{
					ID: to.Ptr(DefaultImageVersionIDs["marinerv2"]),
				}
			},
		},
	}
}
