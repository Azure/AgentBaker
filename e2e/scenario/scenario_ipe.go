package scenario

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v8"
	"k8s.io/apimachinery/pkg/util/wait"
)

func init() {
	// These need a different boot-time IMDS tag from the existing default-mode ACL nodes.
	for _, image := range []struct {
		name  string
		vhd   *config.Image
		arm64 bool
	}{
		{"ACL_IPEAudit", config.VHDACLGen2TL, false},
		{"ACL_ARM64_IPEAudit", config.VHDACLArm64Gen2TL, true},
	} {
		Register(&Scenario{
			Name:        image.name,
			Description: "Verify IMDS-requested IPE audit, active policy, allowed unsigned execution with a denial audit record, and persistence across reboot",
			SkipIf: func(context.Context) string {
				if !strings.EqualFold(os.Getenv("ACL_IPE_AUDIT_E2E"), "true") {
					return "requires explicit ACL_IPE_AUDIT_E2E=true and IPE-capable VHDs"
				}
				return ""
			},
			Config: Config{
				Cluster: ClusterKubenet,
				VHD:     image.vhd,
				UseNVMe: image.arm64,
				BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
					if image.arm64 {
						nbc.AgentPoolProfile.VMSize = "Standard_D2pds_v6"
						nbc.IsARM64 = true
					}
				},
				VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
					vmss.Properties = addTrustedLaunchToVMSS(vmss.Properties)
					if image.arm64 {
						vmss.SKU.Name = to.Ptr("Standard_D2pds_v6")
					}
					if vmss.Tags == nil {
						vmss.Tags = map[string]*string{}
					}
					vmss.Tags["acl-node-security-profile"] = to.Ptr("ipe=audit")
				},
				Validator: func(ctx context.Context, s *Scenario) error {
					if err := validateIPEAudit(ctx, s); err != nil {
						return err
					}
					if err := RebootVMAndWaitForSSH(ctx, s); err != nil {
						return err
					}
					if err := waitForKubeletAfterReboot(ctx, s); err != nil {
						return err
					}
					if err := validateIPEAudit(ctx, s); err != nil {
						return err
					}
					if _, err := s.Runtime.Kube.WaitUntilNodeReady(ctx, s.Runtime.VMSSName); err != nil {
						return err
					}
					return ValidateNodeCanRunAPod(ctx, s)
				},
			},
		})
	}
}

var ipeProbePathPattern = regexp.MustCompile(`(?m)^IPE_PROBE_PATH=(/opt/ab-ipe-audit\.[A-Za-z0-9]+/probe)$`)

func validateIPEAudit(ctx context.Context, s *Scenario) error {
	result, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, ipeAuditProbeScript, 0, "IPE audit state or unsigned execution failed")
	if err != nil {
		return err
	}
	logging.Logf(ctx, "IPE audit probe:\n%s", result.stdout)
	match := ipeProbePathPattern.FindStringSubmatch(result.stdout)
	if match == nil {
		return fmt.Errorf("IPE audit probe did not report its unique executable path")
	}
	probe := match[1]
	// Match this probe's event, not an unrelated or historical denial.
	query := fmt.Sprintf(`set -euo pipefail
sudo journalctl -k -b --no-pager -o cat | awk -v p=%q 'index($0, p)'
if sudo test -f /var/log/audit/audit.log; then
    sudo cat /var/log/audit/audit.log | awk -v p=%q 'index($0, p)'
fi
`, probe, probe)
	err = wait.PollUntilContextTimeout(ctx, time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		records, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, query, 0, "cannot read IPE audit records")
		if err != nil {
			return false, err
		}
		if record := findIPEAuditDenial(records.stdout, probe); record != "" {
			logging.Logf(ctx, "Unsigned executable succeeded while IPE audited a denial:\n%s", record)
			return true, nil
		}
		logging.Logf(ctx, "Waiting for IPE EXECUTE denial with enforcing=0 for %s", probe)
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("could not verify an IPE audit denial for %s: %w", probe, err)
	}
	return nil
}

func findIPEAuditDenial(records, probe string) string {
	for _, line := range strings.Split(records, "\n") {
		fields := strings.Fields(line)
		has := func(want string) bool {
			for _, field := range fields {
				if field == want {
					return true
				}
			}
			return false
		}
		if has("ipe_op=EXECUTE") && has("enforcing=0") &&
			has(`path="`+probe+`"`) && has(`action=DENY"`) {
			return line
		}
	}
	return ""
}

const ipeAuditProbeScript = `
set -euo pipefail
sudo bash <<'IPE_AUDIT'
set -euo pipefail
fail() { echo "FAIL: $*" >&2; exit 1; }
profile=$(curl --fail --silent --show-error --noproxy '*' --connect-timeout 5 --max-time 15 \
    -H Metadata:true 'http://169.254.169.254/metadata/instance/compute/tagsList?api-version=2021-02-01')
printf '%s' "$profile" | jq -e '[.[] | select(.name == "acl-node-security-profile") | .value] == ["ipe=audit"]' >/dev/null ||
    fail "IMDS did not expose the requested ipe=audit profile"
ipe=/sys/kernel/security/ipe
[ "$(cat "$ipe/policies/acl_ipe_boot_policy/active")" = 1 ] || fail "IPE policy is not active"
[ "$(cat "$ipe/enforce")" = 0 ] || fail "IPE is not in audit mode"
cat "$ipe/policies/acl_ipe_boot_policy/policy"
systemctl is-active containerd kubelet
/usr/bin/true
dir=$(mktemp -d /opt/ab-ipe-audit.XXXXXXXXXX)
trap 'rm -f -- "$dir/probe"; rmdir -- "$dir"' EXIT
cp --no-preserve=all /usr/bin/true "$dir/probe"
chmod 0755 "$dir/probe"
"$dir/probe" || fail "unsigned executable was blocked in audit mode"
printf 'IPE_PROBE_PATH=%s/probe\n' "$dir"
IPE_AUDIT
`

func validateACLIPEDisabledControl(ctx context.Context, s *Scenario) error {
	if !strings.EqualFold(os.Getenv("ACL_IPE_AUDIT_E2E"), "true") {
		return nil
	}
	_, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, `
set -euo pipefail
curl --fail --silent --show-error --noproxy '*' --connect-timeout 5 --max-time 15 \
    -H Metadata:true 'http://169.254.169.254/metadata/instance/compute/tagsList?api-version=2021-02-01' |
    jq -e '[.[] | select(.name == "acl-node-security-profile")] | length == 0' >/dev/null
sudo test "$(sudo cat /sys/kernel/security/ipe/policies/acl_ipe_boot_policy/active)" = 0
`, 0, "IPE unexpectedly active on an ACL node without audit opt-in")
	return err
}
