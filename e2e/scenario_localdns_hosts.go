package e2e

import (
	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
)

func init() {
	tests := []struct {
		name            string
		vhd             *config.Image
		vmConfigMutator func(*armcompute.VirtualMachineScaleSet)
	}{
		{name: "Ubuntu2204", vhd: config.VHDUbuntu2204Gen2Containerd},
		{name: "Ubuntu2404", vhd: config.VHDUbuntu2404Gen2Containerd},
		{name: "Ubuntu2604Minimal", vhd: config.VHDUbuntu2604MinimalGen2Containerd},
		{name: "AzureLinuxV3", vhd: config.VHDAzureLinuxV3Gen2},
		{name: "ACL", vhd: config.VHDACLGen2TL, vmConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
			vmss.Properties = addTrustedLaunchToVMSS(vmss.Properties)
		}},
	}

	for _, tt := range tests {
		cluster := ClusterKubenet
		if tt.name == "Ubuntu2604Minimal" {
			cluster = ClusterLatestKubernetesVersionKubenet
		}
		Register(&Scenario{
			Name:        "LocalDNSHostsPlugin/" + tt.name,
			Description: "Tests that localdns hosts plugin works correctly on " + tt.name,
			Config: Config{
				Cluster: cluster,
				VHD:     tt.vhd,
				BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
					nbc.AgentPoolProfile.LocalDNSProfile.EnableHostsPlugin = true
					nbc.AgentPoolProfile.LocalDNSProfile.EnableLocalDNS = true
				},
				AKSNodeConfigMutator: func(_ *Cluster, config *aksnodeconfigv1.Configuration) {
					config.LocalDnsProfile.EnableHostsPlugin = true
					config.LocalDnsProfile.EnableLocalDns = true
				},
				VMConfigMutator: tt.vmConfigMutator,
			},
		})
	}
}
