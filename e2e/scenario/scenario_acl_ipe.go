package scenario

import (
	"context"
	_ "embed"
	"strings"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v8"
	"github.com/samber/lo"
)

//go:embed validate_acl_ipe.sh
var aclIPEAuditScript string

//go:embed validate_acl_overlayfs.sh
var aclDefaultSnapshotterScript string

// IPE (Integrity Policy Enforcement) applies a kernel policy to executable code.
// In audit mode, denied execution is logged but still allowed. These scenarios
// compare the same program on signed /usr and on writable storage.
var (
	_ = Register(newACLIPEAuditScenario(false))
	_ = Register(newACLIPEAuditScenario(true))
)

// The ACL image reads the audit tag during boot, so these checks need separate
// nodes from the ordinary ACL scenarios. Image selection and reporting stay
// with the standard E2E runner.
func newACLIPEAuditScenario(arm64 bool) *Scenario {
	name, image := "ACL_IPEAudit", config.VHDACLGen2TL
	if arm64 {
		name, image = "ACL_ARM64_IPEAudit", config.VHDACLArm64Gen2TL
	}
	return &Scenario{
		Name:        name,
		Description: "Boot an ACL node with IPE audit enabled; verify policy, signed /usr, EROFS and audit behavior for base and writable executables",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     image,
			UseNVMe: arm64,
			// Match the existing ACL ARM64 setup: a Trusted Launch-capable v6 VM,
			// with the same SKU in both bootstrap formats and the VMSS model.
			BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
				if arm64 {
					nbc.AgentPoolProfile.VMSize = "Standard_D2pds_v6"
					nbc.IsARM64 = true
				}
			},
			AKSNodeConfigMutator: func(_ *Cluster, cfg *aksnodeconfigv1.Configuration) {
				if arm64 {
					cfg.VmSize = "Standard_D2pds_v6"
				}
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.Properties = aclVMSSSecurityProfile(vmss.Properties, config.Config.ACLBaseImageSigned)
				if arm64 {
					vmss.SKU.Name = to.Ptr("Standard_D2pds_v6")
				}
				if vmss.Tags == nil {
					vmss.Tags = map[string]*string{}
				}
				vmss.Tags["acl-node-security-profile"] = to.Ptr(ipeAuditProfile(lo.FromPtr(vmss.Tags["acl-node-security-profile"])))
			},
			Validator: validateACLIPEAudit,
		},
	}
}

// The tag is shared with settings such as SELinux. Replace only the IPE entry.
func ipeAuditProfile(profile string) string {
	var fields []string
	for _, field := range strings.Split(profile, ",") {
		if field != "" && strings.SplitN(field, "=", 2)[0] != "ipe" {
			fields = append(fields, field)
		}
	}
	return strings.Join(append(fields, "ipe=audit"), ",")
}

func validateACLIPEAudit(ctx context.Context, s *Scenario) error {
	// Inspect the host kernel and its journal over SSH, not from a container.
	// The requested VM tag alone does not prove that the policy became active.
	result, err := execScriptOnVMForScenarioValidateExitCode(ctx, s,
		"sudo bash -s <<'ACL_IPE_VALIDATION'\n"+aclIPEAuditScript+"\nACL_IPE_VALIDATION\n",
		0, "ACL IPE audit validation failed")
	if result != nil {
		logging.Logf(ctx, "IPE guest evidence:\n%s", result.String())
	}
	return err
}

// Baseline ACL nodes must keep overlayfs when the IPE profile is not requested.
// Read the running daemon's config, including imports and plugin defaults.
func validateACLDefaultSnapshotter(ctx context.Context, s *Scenario) error {
	result, err := execScriptOnVMForScenarioValidateExitCode(ctx, s,
		"sudo bash -s <<'ACL_OVERLAYFS_VALIDATION'\n"+aclDefaultSnapshotterScript+"\nACL_OVERLAYFS_VALIDATION\n",
		0, "ACL default snapshotter validation failed")
	if result != nil {
		logging.Logf(ctx, "ACL default snapshotter:\n%s", result.String())
	}
	return err
}
