package scenario

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/google/uuid"
)

const aclIPEModeEnv = "ACL_IPE_EXPECTED_MODE"

const aclIPEPolicyName = "acl_ipe_boot_policy"
const aclIPESecurityProfileTag = "acl-node-security-profile"

func validateACLIPEVMSSNoProfileTag(tags map[string]*string) error {
	for name := range tags {
		if strings.EqualFold(name, aclIPESecurityProfileTag) {
			return fmt.Errorf("unexpected ACL IPE security profile tag %q on scenario VMSS/instance", name)
		}
	}
	return nil
}

const aclIPEPolicyStateProbe = `set -euo pipefail
ipe="${1:-/sys/kernel/security/ipe}"
fail() { echo "ACL IPE policy probe: $*" >&2; exit 1; }
[[ -f "$ipe/enforce" && -r "$ipe/enforce" ]] || fail "missing or unreadable IPE enforce"
[[ -d "$ipe/policies" && -r "$ipe/policies" && -x "$ipe/policies" ]] || fail "missing or unreadable IPE policies directory"
mode=$(stat -c '%a' "$ipe/policies") || fail "cannot stat IPE policies directory"
(( (8#$mode & 0444) != 0 && (8#$mode & 0111) != 0 )) || fail "unreadable IPE policies directory"
policies=( "$ipe"/policies/* )
[[ -d "${policies[0]}" && ${#policies[@]} -eq 1 ]] || fail "expected exactly one loaded IPE policy"
policy_dir="$ipe/policies/acl_ipe_boot_policy"
[[ "${policies[0]}" == "$policy_dir" && -f "$policy_dir/active" && -f "$policy_dir/policy" ]] || fail "PR52 IPE policy missing"
enforce=$(cat "$ipe/enforce") || fail "cannot read IPE enforce"
active=$(cat "$policy_dir/active") || fail "cannot read PR52 IPE active flag"
policy=$(base64 -w0 "$policy_dir/policy") || fail "cannot read PR52 IPE policy"
printf 'enforce=%s\n' "$enforce"
printf 'active=%s\n' "$active"
printf 'policy_base64=%s\n' "$policy"`

const aclIPEBootEvidenceProbe = `set -euo pipefail
fail() { echo "ACL IPE boot probe: $*" >&2; exit 1; }
boot_file="${1:-/proc/sys/kernel/random/boot_id}"
cmdline_file="${2:-/proc/cmdline}"
cache_file="${3:-/run/acl/node-security-profile}"
failure_file="${4:-/run/acl/node-security-profile.failed}"
boot_id=$(cat "$boot_file") || fail "cannot read boot ID"
cmdline=$(cat "$cmdline_file") || fail "cannot read kernel command line"
[[ ! -e "$failure_file" ]] || fail "initrd IMDS lookup failed"
[[ -f "$cache_file" && -r "$cache_file" ]] || fail "missing successful initrd IMDS profile cache"
cache=$(cat "$cache_file") || fail "cannot read initrd IMDS profile cache"
journal=$(journalctl -b -u acl-ipe-load.service -o cat --no-pager) || fail "cannot read current-boot IPE loader journal"
[[ -n "$journal" ]] || fail "current-boot IPE loader journal empty"
printf 'boot_id=%s\ncmdline=%s\ncache=%s\njournal_base64=%s\n' \
  "$boot_id" "$cmdline" "$cache" "$(printf '%s' "$journal" | base64 -w0)"`

func aclIPEPrivilegedProbe(probe string) string {
	return "#!/usr/bin/env bash\nsudo -n bash -s <<'ACL_IPE_PROBE'\n" + probe + "\nACL_IPE_PROBE"
}

type aclIPEPolicyState struct {
	enforce string
	active  string
	policy  string
}

type aclIPEBootEvidence struct {
	bootID  string
	cmdline string
	cache   string
	journal string
}

func aclIPEValidationRequested(s *Scenario) bool {
	return s != nil && s.Name == "ACL" && os.Getenv(aclIPEModeEnv) != ""
}

func ACLIPEExpectedMode() (string, error) {
	mode := os.Getenv(aclIPEModeEnv)
	if mode == "" {
		return "", nil
	}
	if mode != "off" && mode != "audit" {
		return "", fmt.Errorf("%s must be off or audit for the ACL scenario, got %q", aclIPEModeEnv, mode)
	}
	return mode, nil
}

func aclIPEExpectedMode(s *Scenario) (string, error) {
	if !aclIPEValidationRequested(s) {
		return "", nil
	}
	return ACLIPEExpectedMode()
}

func parseACLIPEPolicyState(output string) (aclIPEPolicyState, error) {
	var state aclIPEPolicyState
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok || value == "" {
			return state, fmt.Errorf("invalid IPE policy state line %q", line)
		}
		switch key {
		case "enforce":
			if state.enforce != "" {
				return state, fmt.Errorf("duplicate IPE enforce state")
			}
			state.enforce = value
		case "active":
			if state.active != "" {
				return state, fmt.Errorf("duplicate IPE active flag")
			}
			state.active = value
		case "policy_base64":
			if state.policy != "" {
				return state, fmt.Errorf("duplicate IPE policy")
			}
			decoded, err := base64.StdEncoding.DecodeString(value)
			if err != nil {
				return state, fmt.Errorf("decode IPE policy: %w", err)
			}
			state.policy = string(decoded)
		default:
			return state, fmt.Errorf("unexpected IPE policy state line %q", line)
		}
	}
	if state.enforce != "0" && state.enforce != "1" {
		return state, fmt.Errorf("missing or invalid IPE enforce state %q", state.enforce)
	}
	if (state.active != "0" && state.active != "1") || state.policy == "" {
		return state, fmt.Errorf("missing or invalid PR52 IPE policy or active flag")
	}
	return state, nil
}

func validateACLIPEPolicyState(state aclIPEPolicyState, expected string) error {
	wantPolicy := []string{
		"policy_name=acl_ipe_boot_policy policy_version=0.0.1",
		"DEFAULT action=ALLOW",
		"DEFAULT op=EXECUTE action=DENY",
		"op=EXECUTE boot_verified=TRUE action=ALLOW",
		"op=EXECUTE dmverity_signature=TRUE action=ALLOW",
	}
	var gotPolicy []string
	for _, line := range strings.Split(state.policy, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			gotPolicy = append(gotPolicy, strings.Join(strings.Fields(line), " "))
		}
	}
	if strings.Join(gotPolicy, "\n") != strings.Join(wantPolicy, "\n") {
		return fmt.Errorf("loaded IPE policy does not match PR52 %s version 0.0.1 and rules", aclIPEPolicyName)
	}
	switch expected {
	case "off":
		if state.active != "0" {
			return fmt.Errorf("expected loaded but inactive PR52 IPE policy (active=0), got %q", state.active)
		}
	case "audit":
		if state.active != "1" || state.enforce != "0" {
			return fmt.Errorf("expected active PR52 IPE policy in audit mode (active=1 enforce=0), got active=%q enforce=%q", state.active, state.enforce)
		}
	default:
		return fmt.Errorf("invalid expected IPE mode %q", expected)
	}
	return nil
}

func validateACLIPEIMDSTags(output string) error {
	var tags []struct {
		Name  *string `json:"name"`
		Value *string `json:"value"`
	}
	if err := json.Unmarshal([]byte(output), &tags); err != nil {
		return fmt.Errorf("invalid IMDS tagsList: %w", err)
	}
	if tags == nil {
		return fmt.Errorf("invalid IMDS tagsList: expected array")
	}
	for _, tag := range tags {
		if tag.Name == nil || tag.Value == nil {
			return fmt.Errorf("malformed IMDS tagsList entry")
		}
		if strings.EqualFold(*tag.Name, aclIPESecurityProfileTag) {
			return fmt.Errorf("unexpected ACL IPE security profile tag %q in live IMDS", *tag.Name)
		}
	}
	return nil
}

var aclIPEHashToken = regexp.MustCompile(`^[0-9a-f]{64}$`)

func parseACLIPEBootEvidence(output string) (aclIPEBootEvidence, error) {
	var evidence aclIPEBootEvidence
	seen := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSuffix(output, "\n"), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok || seen[key] {
			return evidence, fmt.Errorf("invalid or duplicate ACL IPE boot evidence field %q", key)
		}
		seen[key] = true
		switch key {
		case "boot_id":
			evidence.bootID = value
		case "cmdline":
			evidence.cmdline = value
		case "cache":
			evidence.cache = value
		case "journal_base64":
			journal, err := base64.StdEncoding.DecodeString(value)
			if err != nil {
				return evidence, fmt.Errorf("decode IPE loader journal: %w", err)
			}
			evidence.journal = string(journal)
		default:
			return evidence, fmt.Errorf("unexpected ACL IPE boot evidence field %q", key)
		}
	}
	if len(seen) != 4 {
		return evidence, fmt.Errorf("missing ACL IPE boot evidence fields")
	}
	return evidence, nil
}

func validateACLIPEBootEvidence(evidence aclIPEBootEvidence, expected string) error {
	if _, err := uuid.Parse(evidence.bootID); err != nil {
		return fmt.Errorf("invalid current boot ID %q: %w", evidence.bootID, err)
	}
	if evidence.cache != "" {
		return fmt.Errorf("expected empty initrd IMDS profile cache for untagged first boot, got %q", evidence.cache)
	}
	var hashes []string
	for _, word := range strings.Fields(evidence.cmdline) {
		if strings.HasPrefix(word, "acl.ipe.policy_sha256=") {
			hashes = append(hashes, strings.TrimPrefix(word, "acl.ipe.policy_sha256="))
		}
	}
	if len(hashes) != 1 || !aclIPEHashToken.MatchString(hashes[0]) || hashes[0] == strings.Repeat("0", 64) {
		return fmt.Errorf("expected one nonzero UKI-bound acl.ipe.policy_sha256 token")
	}
	if !strings.Contains(" "+evidence.cmdline+" ", " flatcar.first_boot=detected ") {
		return fmt.Errorf("kernel command line lacks first-boot UKI addon marker")
	}
	for _, line := range []string{
		"Credential SHA-256 verified: " + hashes[0],
		"Loaded policy " + aclIPEPolicyName + " into kernel IPE.",
		"Using IPE mode '" + expected + "'.",
	} {
		if !strings.Contains(evidence.journal, "acl-ipe-load: "+line) {
			return fmt.Errorf("current-boot loader journal lacks %q", line)
		}
	}
	modeEvidence := "IPE mode is off; policy loaded but activation skipped."
	if expected == "audit" {
		modeEvidence = "Activated policy " + aclIPEPolicyName + "."
	}
	if !strings.Contains(evidence.journal, "acl-ipe-load: "+modeEvidence) {
		return fmt.Errorf("current-boot loader journal lacks %q", modeEvidence)
	}
	return nil
}

func validateACLIPEIMDSField(field, got, want string) error {
	got = strings.TrimSpace(got)
	switch field {
	case "vmId":
		actualID, err := uuid.Parse(got)
		if err != nil {
			return fmt.Errorf("invalid IMDS vmId %q: %w", got, err)
		}
		expectedID, err := uuid.Parse(want)
		if err != nil {
			return fmt.Errorf("invalid Azure VM ID %q: %w", want, err)
		}
		if actualID != expectedID {
			return fmt.Errorf("IMDS vmId %q does not match scenario VM ID %q", got, want)
		}
	case "vmScaleSetName":
		if got == "" || got != want {
			return fmt.Errorf("IMDS vmScaleSetName %q does not match scenario VMSS %q", got, want)
		}
	default:
		return fmt.Errorf("unknown IMDS field %q", field)
	}
	return nil
}

func ValidateACLIPEFirstBoot(ctx context.Context, s *Scenario) (err error) {
	expected, err := aclIPEExpectedMode(s)
	if err != nil || expected == "" {
		return err
	}
	start := time.Now()
	defer func() {
		s.recordADOTestCase("ACL_IPE_FirstBoot_"+expected, "e2e.acl.ipe", time.Since(start), err)
	}()
	if s.Runtime == nil || s.Runtime.VM == nil || s.Runtime.VM.VM == nil ||
		s.Runtime.VM.VM.Properties == nil || s.Runtime.VM.VM.Properties.VMID == nil {
		return fmt.Errorf("ACL IPE validation requires the scenario VM's Azure VM ID")
	}
	if s.Runtime.VM.VMSS == nil || s.Runtime.VM.VMSS.Tags == nil {
		return fmt.Errorf("ACL IPE validation requires the created scenario VMSS tags")
	}
	if err := validateACLIPEVMSSNoProfileTag(s.Runtime.VM.VMSS.Tags); err != nil {
		return fmt.Errorf("created scenario VMSS tags: %w", err)
	}
	if err := validateACLIPEVMSSNoProfileTag(s.Runtime.VM.VM.Tags); err != nil {
		return fmt.Errorf("scenario VM instance tags: %w", err)
	}

	result, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, aclIPEPrivilegedProbe(aclIPEPolicyStateProbe), 0, "could not read IPE securityfs policy state")
	if err != nil {
		return fmt.Errorf("read ACL IPE policy state: %w", err)
	}
	state, err := parseACLIPEPolicyState(result.stdout)
	if err != nil {
		return err
	}
	if err := validateACLIPEPolicyState(state, expected); err != nil {
		return err
	}

	for _, check := range []struct {
		field, want string
	}{
		{"vmId", *s.Runtime.VM.VM.Properties.VMID},
		{"vmScaleSetName", s.Runtime.VMSSName},
	} {
		endpoint := fmt.Sprintf("http://169.254.169.254/metadata/instance/compute/%s?api-version=2021-02-01&format=text", check.field)
		command := fmt.Sprintf("curl --noproxy '*' --connect-timeout 5 --max-time 10 --retry 2 -fsS -H 'Metadata: true' '%s'", endpoint)
		imds, execErr := execScriptOnVMForScenarioValidateExitCode(ctx, s, command, 0, "could not read IMDS "+check.field)
		if execErr != nil {
			return fmt.Errorf("read ACL IMDS %s: %w", check.field, execErr)
		}
		if err := validateACLIPEIMDSField(check.field, imds.stdout, check.want); err != nil {
			return err
		}
	}

	tags, err := execScriptOnVMForScenarioValidateExitCode(ctx, s,
		"curl --noproxy '*' --connect-timeout 5 --max-time 10 --retry 2 -fsS -H 'Metadata: true' 'http://169.254.169.254/metadata/instance/compute/tagsList?api-version=2021-02-01'",
		0, "could not read IMDS tagsList")
	if err != nil {
		return fmt.Errorf("read ACL IMDS tagsList: %w", err)
	}
	if err := validateACLIPEIMDSTags(tags.stdout); err != nil {
		return err
	}
	boot, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, aclIPEPrivilegedProbe(aclIPEBootEvidenceProbe), 0, "could not read current-boot IPE evidence")
	if err != nil {
		return fmt.Errorf("read ACL IPE current-boot evidence: %w", err)
	}
	evidence, err := parseACLIPEBootEvidence(boot.stdout)
	if err != nil {
		return err
	}
	if err := validateACLIPEBootEvidence(evidence, expected); err != nil {
		return err
	}

	if err := ValidateSystemdUnitIsRunning(ctx, s, "kubelet"); err != nil {
		return fmt.Errorf("ACL IPE first-boot kubelet health: %w", err)
	}
	if err := ValidateSystemdUnitIsRunning(ctx, s, "containerd"); err != nil {
		return fmt.Errorf("ACL IPE first-boot containerd health: %w", err)
	}
	logging.Logf(ctx, "ACL IPE current-boot validated: expected=%s policy=%s active=%s enforce=%s bootId=%s vmss=%s vmId=%s", expected, aclIPEPolicyName, state.active, state.enforce, evidence.bootID, s.Runtime.VMSSName, *s.Runtime.VM.VM.Properties.VMID)
	return nil
}
