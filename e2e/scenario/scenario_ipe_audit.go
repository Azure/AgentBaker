package scenario

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/logging"
)

const aclIPEAuditPolicyName = "acl_ipe_boot_policy"
const aclIPEAuditDenyRule = "DEFAULT op=EXECUTE action=DENY"

var aclIPEAuditProbePath = regexp.MustCompile(`^/var/tmp/acl-ipe-audit\.[a-zA-Z0-9]{8}/probe$`)

const aclIPEAuditProbeScript = `#!/usr/bin/env bash
set -euo pipefail
dir=$(mktemp -d /var/tmp/acl-ipe-audit.XXXXXXXX)
probe="$dir/probe"
cleanup() {
  rm -f -- "$probe" && rmdir -- "$dir"
}
trap 'status=$?; cleanup || status=1; exit "$status"' EXIT
cp /usr/bin/true "$probe"
chmod 0755 "$probe"
"$probe"
command -v timeout >/dev/null || { echo 'ACL IPE audit probe requires timeout' >&2; exit 1; }
deadline=$((SECONDS + 30))
collect() {
  local remaining=$((deadline - SECONDS)) status
  if (( remaining <= 0 )); then
    echo 'ACL IPE audit probe deadline reached' >&2
    return 124
  fi
  if (( remaining > 5 )); then remaining=5; fi
  if sudo -n timeout --kill-after=1s "${remaining}s" "$@"; then
    return 0
  else
    status=$?
    if (( status == 124 || status == 137 )); then
      printf 'ACL IPE audit collector timed out after %ss\n' "$remaining" >&2
    fi
    return "$status"
  fi
}
for attempt in {1..15}; do
  if (( SECONDS >= deadline )); then break; fi
  native=""
  journal=""
  kernel=""
  native_error=""
  journal_error=""
  kernel_error=""
  if ! native=$(collect journalctl -b -o json --no-pager _TRANSPORT=audit _AUDIT_TYPE=1420 2>&1); then
    native_error=${native:-no-diagnostic}
    native=""
  fi
  if ! journal=$(collect journalctl -k -b --no-pager 2>&1); then
    journal_error=${journal:-no-diagnostic}
    journal=""
  fi
  if ! kernel=$(collect dmesg 2>&1); then
    kernel_error=${kernel:-no-diagnostic}
    kernel=""
  fi
  if [ -n "$native_error" ]; then
    printf 'native audit journal unavailable: %s\n' "$native_error" >&2
  fi
  if [ -n "$journal_error" ]; then
    printf 'kernel journal unavailable: %s\n' "$journal_error" >&2
  fi
  if [ -n "$kernel_error" ]; then
    printf 'dmesg unavailable: %s\n' "$kernel_error" >&2
  fi
  if [ -n "$native_error" ] && [ -n "$journal_error" ] && [ -n "$kernel_error" ]; then
    exit 1
  fi
  native_candidates=$(printf '%s\n' "$native" | grep -F -- "$probe") || native_candidates=""
  kernel_candidates=$(printf '%s\n%s\n' "$journal" "$kernel" | grep -F -- "$probe") || kernel_candidates=""
  native_matches=""
  kernel_matches=""
  if [ -n "$native_candidates" ]; then
    native_matches=$(printf '%s\n' "$native_candidates" |
        grep -E '"_TRANSPORT"[[:space:]]*:[[:space:]]*"audit"' |
        grep -E '"_AUDIT_TYPE"[[:space:]]*:[[:space:]]*"1420"' |
        grep -F -- "path=\\\"$probe\\\"" |
        grep -F ' ipe_op=EXECUTE ' |
        grep -F ' enforcing=0 ' |
        grep -F 'rule=\"DEFAULT op=EXECUTE action=DENY\"') || native_matches=""
  fi
  if [ -n "$kernel_candidates" ]; then
    kernel_matches=$(printf '%s\n' "$kernel_candidates" |
        grep -E '(^|[[:space:]])type=1420[[:space:]]' |
        grep -F -- "path=\"$probe\"" |
        grep -F ' ipe_op=EXECUTE ' |
        grep -F ' enforcing=0 ' |
        grep -F 'rule="DEFAULT op=EXECUTE action=DENY"') || kernel_matches=""
  fi
  if { [ -n "$native_candidates" ] && [ "$native_candidates" != "$native_matches" ]; } ||
     { [ -n "$kernel_candidates" ] && [ "$kernel_candidates" != "$kernel_matches" ]; }; then
    printf 'path-bearing IPE audit candidates including nonmatches (attempt %s):\n%s\n%s\n' \
      "$attempt" "$native_candidates" "$kernel_candidates" >&2
  fi
  if [ -n "$native_matches" ] || [ -n "$kernel_matches" ]; then
    printf 'probe=%s\n%s\n%s\n' "$probe" "$native_matches" "$kernel_matches"
    exit 0
  fi
  remaining=$((deadline - SECONDS))
  if (( remaining <= 0 || attempt == 15 )); then break; fi
  if (( remaining > 2 )); then remaining=2; fi
  sleep "$remaining"
done
printf 'no IPE audit access record for %s within the 30-second poll budget\n' "$probe" >&2
exit 1`

func ValidateACLIPE(ctx context.Context, s *Scenario) error {
	if err := ValidateACLIPEFirstBoot(ctx, s); err != nil {
		return err
	}
	if err := ValidateACLIPEAuditDeny(ctx, s); err != nil {
		return err
	}
	return ValidateACLIPETransition(ctx, s)
}

func validateACLIPEAuditRecord(output string) (string, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 || !strings.HasPrefix(lines[0], "probe=/var/tmp/acl-ipe-audit.") {
		return "", fmt.Errorf("missing unique ACL IPE audit probe path")
	}
	probe := strings.TrimPrefix(lines[0], "probe=")
	if !aclIPEAuditProbePath.MatchString(probe) {
		return "", fmt.Errorf("invalid ACL IPE audit probe path %q", probe)
	}
	for _, line := range lines[1:] {
		message := line
		if strings.HasPrefix(line, "{") {
			var entry struct {
				Transport string `json:"_TRANSPORT"`
				AuditType string `json:"_AUDIT_TYPE"`
				Message   string `json:"MESSAGE"`
			}
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				return "", fmt.Errorf("decode native IPE audit journal entry: %w", err)
			}
			if entry.Transport != "audit" || entry.AuditType != "1420" {
				continue
			}
			message = entry.Message
		} else if !strings.Contains(" "+line+" ", " type=1420 ") {
			continue
		}
		if strings.Contains(message, `path="`+probe+`"`) &&
			strings.Contains(" "+message+" ", " ipe_op=EXECUTE ") &&
			strings.Contains(" "+message+" ", " enforcing=0 ") &&
			strings.Contains(message, `rule="`+aclIPEAuditDenyRule+`"`) {
			return probe, nil
		}
	}
	return "", fmt.Errorf("no permissive IPE EXECUTE audit record for %q matching %q", probe, aclIPEAuditDenyRule)
}

func validateACLIPEAuditPolicy(policy string) error {
	if !strings.HasPrefix(policy, "policy_name="+aclIPEAuditPolicyName+" ") {
		return fmt.Errorf("active ACL IPE policy is not %s", aclIPEAuditPolicyName)
	}
	for _, line := range strings.Split(policy, "\n") {
		if strings.TrimSpace(line) == aclIPEAuditDenyRule {
			return nil
		}
	}
	return fmt.Errorf("active ACL IPE policy does not contain the PR52 EXECUTE deny rule")
}

func ValidateACLIPEAuditDeny(ctx context.Context, s *Scenario) (err error) {
	mode, err := aclIPEExpectedMode(s)
	if err != nil || mode != "audit" {
		return err
	}
	start := time.Now()
	defer func() {
		s.recordADOTestCase("ACL_IPE_AuditDeny", "e2e.acl.ipe", time.Since(start), err)
	}()
	return validateACLIPEAuditDeny(ctx, s)
}

func validateACLIPEAuditDeny(ctx context.Context, s *Scenario) error {
	policyPath := "/sys/kernel/security/ipe/policies/" + aclIPEAuditPolicyName
	active, err := getFileContent(ctx, s, policyPath+"/active")
	if err != nil {
		return fmt.Errorf("read ACL IPE policy activation: %w", err)
	}
	if strings.TrimSpace(active) != "1" {
		return fmt.Errorf("expected PR52 IPE policy %s to be active, got %q", aclIPEAuditPolicyName, strings.TrimSpace(active))
	}
	policy, err := getFileContent(ctx, s, policyPath+"/policy")
	if err != nil {
		return fmt.Errorf("read active ACL IPE policy: %w", err)
	}
	if err := validateACLIPEAuditPolicy(policy); err != nil {
		return err
	}
	probeResult, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, aclIPEAuditProbeScript, 0, "ACL IPE audit probe failed")
	if err != nil {
		return fmt.Errorf("run ACL IPE audit probe: %w", err)
	}
	if probeResult.stderr != "" {
		logging.Logf(ctx, "ACL IPE audit probe diagnostics: %s", strings.TrimSpace(probeResult.stderr))
	}
	probe, err := validateACLIPEAuditRecord(probeResult.stdout)
	if err != nil {
		return err
	}
	logging.Logf(ctx, "ACL IPE audit-deny probe succeeded: %s", probe)
	return nil
}
