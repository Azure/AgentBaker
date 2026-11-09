package scenario

// TEST-ONLY: lives on the sakwa/test/localdns-lps-rejected-* branch, never merged.

import (
	"context"
	"fmt"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

// localDNSNoFallbackDropInPath moves the rejection marker somewhere it cannot be written (procfs
// does not allow creating files), so every start fetches the rejected patch again, as before the fix.
const localDNSNoFallbackDropInPath = "/etc/systemd/system/localdns.service.d/01-e2e-no-fallback.conf"

func init() {
	// LocalDNSLPSRejectedCorefileNoFallbackControl is the negative control for
	// LocalDNSLPSRejectedCorefileFallback: the same rejected payload with the fallback disabled.
	// It is EXPECTED TO FAIL provisioning (CSE exit 216, ERR_LOCALDNS_FAIL).
	Register(&Scenario{
		Name:        "LocalDNSLPSRejectedCorefileNoFallbackControl",
		Description: "Negative control: a rejected LPS corefile with the fallback disabled is expected to fail provisioning",
		SkipIf:      skipUnlessBranchVHD,
		Config: Config{
			Cluster:                    ClusterKubenet,
			VHD:                        config.VHDUbuntu2404Gen2Containerd,
			ForceScriptlessCompilation: true,
			BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.LocalDNSProfile.EnableLocalDNS = true
			},
			CustomDataWriteFiles: []CustomDataWriteFile{
				{
					Path:        localDNSFetcherDropInPath,
					Permissions: "0644",
					Owner:       "root",
					Content:     localDNSFetcherDropIn(),
				},
				{
					Path:        localDNSNoFallbackDropInPath,
					Permissions: "0644",
					Owner:       "root",
					Content:     "[Service]\nEnvironment=LOCALDNS_LPS_REJECTED_MARKER=/proc/e2e-localdns-lps-corefile-rejected\n",
				},
				{
					Path:        localDNSPayloadPath,
					Permissions: "0644",
					Owner:       "root",
					Content:     localDNSLPSRejectedPayload(),
				},
				{
					Path:        localDNSFetcherPath,
					Permissions: "0755",
					Owner:       "root",
					Content:     localDNSLPSFetcherWrapper(),
				},
			},
			AKSNodeConfigMutator: func(_ *Cluster, config *aksnodeconfigv1.Configuration) {
				config.LocalDnsProfile.EnableLocalDns = true
			},
			Validator: func(context.Context, *Scenario) error {
				return fmt.Errorf("negative control provisioned: expected CSE to fail with the fallback disabled")
			},
		},
	})
}
