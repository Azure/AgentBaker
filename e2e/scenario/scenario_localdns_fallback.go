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
					// Enabling LocalDNS is only half of what the RP does for a LocalDNS
					// pool; it also points kubelet at the cluster listener, which is what
					// bakes 169.254.10.11 into every pod's resolv.conf. Without this the
					// node runs LocalDNS but no pod on it ever uses .11, so every
					// pod-level assertion below skips and the whole premise of the
					// fallback goes untested.
					nbc.KubeletConfig["--cluster-dns"] = localDNSClusterListenerIP
				},
				AKSNodeConfigMutator: func(_ *Cluster, config *aksnodeconfigv1.Configuration) {
					config.LocalDnsProfile.EnableLocalDns = true
					config.KubeletConfig.KubeletFlags["--cluster-dns"] = localDNSClusterListenerIP
					// The flag alone is not parity with the CSE path. '--cluster-dns' is in
					// pkg/agent.TranslatedKubeletConfigFlags, so on the CSE path setting the
					// flag also rewrites KubeletConfiguration.clusterDNS in the kubelet config
					// file. On the ANC path the flag and the config-file field are independent
					// proto fields, so the flag alone leaves clusterDNS at the cluster default
					// and the two paths emit different KUBELET_CONFIG_FILE_CONTENT -- which the
					// provision-config/nbc-cmd env parity validator fails on.
					config.KubeletConfig.KubeletConfigFileConfig.ClusterDns = []string{localDNSClusterListenerIP}
				},
				Validator: func(ctx context.Context, s *Scenario) error {
					return ValidateLocalDNSFallbackRecovery(ctx, s)
				},
			},
		})
	}
}
