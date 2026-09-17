package scenario

import (
	"context"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

// Registers a dedicated LocalDNS pod-DNS fallback scenario per distro. It is
// kept separate from the LocalDNSHostsPlugin scenarios so its crash-storm /
// handoff timing (inherently flakier than steady-state checks) can be
// quarantined or gated independently without destabilizing them.
//
// Each entry runs under both the CSE (BootstrapConfigMutator) and ANC
// (AKSNodeConfigMutator) provisioning paths, on a default service CIDR. A
// custom-CIDR variant is a follow-up gated on the aks-rp CoreDnsServiceIp fix.
func init() {
	tests := []struct {
		name string
		vhd  *config.Image
	}{
		{name: "Ubuntu2204", vhd: config.VHDUbuntu2204Gen2Containerd},
		{name: "Ubuntu2404", vhd: config.VHDUbuntu2404Gen2Containerd},
		{name: "AzureLinuxV3", vhd: config.VHDAzureLinuxV3Gen2},
	}

	for _, tt := range tests {
		Register(&Scenario{
			Name:        "LocalDNSFallback/" + tt.name,
			Description: "localdns crash-storm and clean-stop drive the pod-DNS fallback (.11) on " + tt.name,
			Config: Config{
				Cluster: ClusterKubenet,
				VHD:     tt.vhd,
				BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
					nbc.AgentPoolProfile.LocalDNSProfile.EnableLocalDNS = true
				},
				AKSNodeConfigMutator: func(_ *Cluster, config *aksnodeconfigv1.Configuration) {
					config.LocalDnsProfile.EnableLocalDns = true
				},
				Validator: func(ctx context.Context, s *Scenario) error {
					return ValidateLocalDNSFallbackRecovery(ctx, s)
				},
			},
		})
	}
}
