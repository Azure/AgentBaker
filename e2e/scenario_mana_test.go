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

	for _, tc := range []struct{ defaultSKU, manaSKU string }{
		{"Standard_NM16ads_MA35D", "Standard_D2ds_v6"},
		{"Standard_D2ds_v5", "Standard_D4ds_v6"},
		{"Standard_E4s_v7", "Standard_D2ds_v6"},
	} {
		t.Run(tc.defaultSKU+"/"+tc.manaSKU, func(t *testing.T) {
			defaultSKU := tc.defaultSKU
			originalSKU := config.Config.DefaultVMSKU
			originalMANASKU := config.Config.MANAVMSKU
			t.Cleanup(func() {
				config.Config.DefaultVMSKU = originalSKU
				config.Config.MANAVMSKU = originalMANASKU
			})
			config.Config.DefaultVMSKU = defaultSKU
			config.Config.MANAVMSKU = tc.manaSKU

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
					require.Equal(t, tc.manaSKU, nbc.AgentPoolProfile.VMSize)
					require.Equal(t, tc.manaSKU, nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize)

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
					require.Equal(t, tc.manaSKU, *vmss.SKU.Name)
					require.NotNil(t, nic.Properties)
					require.Equal(t, to.Ptr(true), nic.Properties.EnableAcceleratedNetworking)
				})
			}
		})
	}
}
