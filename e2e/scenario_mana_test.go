package e2e

import (
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
	"github.com/stretchr/testify/require"
)

func TestMANAScenariosUseExplicitSKU(t *testing.T) {
	scenarios := make(map[string]*Scenario)
	for _, s := range registeredScenarios() {
		scenarios[s.Name] = s
	}

	for _, defaultSKU := range []string{"Standard_NM16ads_MA35D", "Standard_D2ds_v5", "Standard_E4s_v7"} {
		t.Run(defaultSKU, func(t *testing.T) {
			originalSKU := config.Config.DefaultVMSKU
			t.Cleanup(func() { config.Config.DefaultVMSKU = originalSKU })
			config.Config.DefaultVMSKU = defaultSKU

			for _, name := range []string{"Ubuntu2204_MANA", "Ubuntu2404_MANA", "Ubuntu2604Minimal_MANA", "AzureLinuxV3_MANA"} {
				t.Run(name, func(t *testing.T) {
					s, ok := scenarios[name]
					require.True(t, ok, "MANA scenario must remain registered")
					nbc := &datamodel.NodeBootstrappingConfiguration{
						ContainerService: &datamodel.ContainerService{
							Properties: &datamodel.Properties{
								AgentPoolProfiles: []*datamodel.AgentPoolProfile{{VMSize: defaultSKU}},
							},
						},
						AgentPoolProfile: &datamodel.AgentPoolProfile{VMSize: defaultSKU},
					}
					s.BootstrapConfigMutator(nil, nbc)
					require.Equal(t, "Standard_D2ds_v6", nbc.AgentPoolProfile.VMSize)
					require.Equal(t, "Standard_D2ds_v6", nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize)

					nic := &armcompute.VirtualMachineScaleSetNetworkConfiguration{}
					vmss := &armcompute.VirtualMachineScaleSet{
						SKU: &armcompute.SKU{Name: to.Ptr(defaultSKU)},
						Properties: &armcompute.VirtualMachineScaleSetProperties{
							VirtualMachineProfile: &armcompute.VirtualMachineScaleSetVMProfile{
								NetworkProfile: &armcompute.VirtualMachineScaleSetNetworkProfile{
									NetworkInterfaceConfigurations: []*armcompute.VirtualMachineScaleSetNetworkConfiguration{nic},
								},
							},
						},
					}
					s.VMConfigMutator(vmss)
					require.Equal(t, "Standard_D2ds_v6", *vmss.SKU.Name)
					require.NotNil(t, nic.Properties)
					require.Equal(t, to.Ptr(true), nic.Properties.EnableAcceleratedNetworking)
				})
			}
		})
	}
}
