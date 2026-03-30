package e2e

import (
	"testing"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
)

// Test_LocalDNSHostsPlugin tests the localdns hosts plugin across all supported distros
// on the legacy (bash CSE) bootstrap path.
// Hosts plugin validators (AA flag, IP match, Corefile, hosts file) run automatically
// via ValidateCommonLinux when EnableHostsPlugin is set.
//
// Run a single distro with: go test -run "Test_LocalDNSHostsPlugin/AzureLinuxV3" -v
func Test_LocalDNSHostsPlugin(t *testing.T) {
	tests := []struct {
		name            string
		vhd             *config.Image
		vmConfigMutator func(*armcompute.VirtualMachineScaleSet)
	}{
		{name: "Ubuntu2204", vhd: config.VHDUbuntu2204Gen2Containerd},
		{name: "Ubuntu2404", vhd: config.VHDUbuntu2404Gen2Containerd},
		{name: "AzureLinuxV3", vhd: config.VHDAzureLinuxV3Gen2},
		{name: "ACL", vhd: config.VHDACLGen2TL, vmConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
			vmss.Properties = addTrustedLaunchToVMSS(vmss.Properties)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RunScenario(t, &Scenario{
				Description: "Tests that localdns hosts plugin works correctly on " + tt.name,
				Config: Config{
					Cluster: ClusterKubenet,
					VHD:     tt.vhd,
					BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
						nbc.AgentPoolProfile.LocalDNSProfile.EnableHostsPlugin = true
					},
					VMConfigMutator: tt.vmConfigMutator,
				},
			})
		})
	}
}

// Test_LocalDNSHostsPlugin_Scriptless tests the localdns hosts plugin across all supported distros
// on the scriptless (aks-node-controller) bootstrap path.
// The base AKSNodeConfig from nbcToAKSNodeConfigV1 already includes a full LocalDnsProfile with
// DNS overrides, so the mutator only needs to enable the hosts plugin.
//
// Run a single distro with: go test -run "Test_LocalDNSHostsPlugin_Scriptless/Ubuntu2204" -v
func Test_LocalDNSHostsPlugin_Scriptless(t *testing.T) {
	tests := []struct {
		name            string
		vhd             *config.Image
		vmConfigMutator func(*armcompute.VirtualMachineScaleSet)
	}{
		{name: "Ubuntu2204", vhd: config.VHDUbuntu2204Gen2Containerd},
		{name: "Ubuntu2404", vhd: config.VHDUbuntu2404Gen2Containerd},
		{name: "AzureLinuxV3", vhd: config.VHDAzureLinuxV3Gen2},
		{name: "ACL", vhd: config.VHDACLGen2TL, vmConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
			vmss.Properties = addTrustedLaunchToVMSS(vmss.Properties)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RunScenario(t, &Scenario{
				Description: "Tests that localdns hosts plugin works correctly on " + tt.name + " (scriptless)",
				Config: Config{
					Cluster:         ClusterKubenet,
					VHD:             tt.vhd,
					VMConfigMutator: tt.vmConfigMutator,
					AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
						config.LocalDnsProfile.EnableHostsPlugin = true
					},
				},
			})
		})
	}
}
