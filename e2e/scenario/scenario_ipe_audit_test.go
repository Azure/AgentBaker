package scenario

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestACLIPEAuditPolicy(t *testing.T) {
	policy := "policy_name=acl_ipe_boot_policy policy_version=0.0.1\n" +
		"op=EXECUTE boot_verified=TRUE action=ALLOW\n" +
		"DEFAULT op=EXECUTE action=DENY\nDEFAULT action=ALLOW\n"
	require.NoError(t, validateACLIPEAuditPolicy(policy))
	require.ErrorContains(t, validateACLIPEAuditPolicy(strings.Replace(policy, aclIPEAuditPolicyName, "other", 1)), "is not")
	require.ErrorContains(t, validateACLIPEAuditPolicy(strings.Replace(policy, aclIPEAuditDenyRule, "DEFAULT op=EXECUTE action=ALLOW", 1)), "deny rule")
}

func aclIPEAuditCollectorForTest(t *testing.T) string {
	t.Helper()
	_, collector, ok := strings.Cut(aclIPEAuditProbeScript, "deadline=$((SECONDS + 30))\n")
	require.True(t, ok)
	return "deadline=$((SECONDS + 30))\n" + collector
}

func TestACLIPEAuditNativeJournalShellProbe(t *testing.T) {
	bash := aclIPETestBash(t)
	syntax := exec.Command(bash, "-n")
	syntax.Stdin = strings.NewReader(aclIPEAuditProbeScript)
	syntaxOutput, err := syntax.CombinedOutput()
	require.NoError(t, err, string(syntaxOutput))

	const probe = "/var/tmp/acl-ipe-audit.aB123456/probe"
	script := `set -euo pipefail
probe="` + probe + `"
poll=1
sleep() { poll=$((poll + 1)); }
sudo() {
  [[ "$1" == "-n" && "$2" == "timeout" ]] || return 1
  shift 2
  timeout "$@"
}
timeout() {
  [[ "$1" == "--kill-after=1s" && "$2" == "5s" ]] || return 1
  shift 2
  "$@"
}
journalctl() {
  if [[ "$1" == "-b" && "$2" == "-o" ]]; then
    if [ "$poll" -eq 1 ]; then
      op=READ
    else
      op=EXECUTE
    fi
    printf '{"_TRANSPORT": "audit", "_AUDIT_TYPE": "1420", "MESSAGE": "audit(123:456): ipe_op=%s enforcing=0 path=\\"%s\\" rule=\\"DEFAULT op=EXECUTE action=DENY\\""}\n' "$op" "$probe"
  fi
}
dmesg() { :; }
` + aclIPEAuditCollectorForTest(t)
	cmd := exec.Command(bash, "-c", script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	require.NoError(t, cmd.Run(), stderr.String())
	require.Contains(t, stderr.String(), "path-bearing IPE audit candidates including nonmatches (attempt 1)")
	require.Contains(t, stderr.String(), "ipe_op=READ")
	require.NotContains(t, stdout.String(), "ipe_op=READ")
	require.Contains(t, stdout.String(), "ipe_op=EXECUTE")
	_, err = validateACLIPEAuditRecord(stdout.String())
	require.NoError(t, err, stdout.String())
}

func TestACLIPEAuditKernelJournalShellFallback(t *testing.T) {
	const probe = "/var/tmp/acl-ipe-audit.aB123456/probe"
	script := `set -euo pipefail
probe="` + probe + `"
sudo() {
  [[ "$1" == "-n" && "$2" == "timeout" ]] || return 1
  shift 2
  timeout "$@"
}
timeout() {
  [[ "$1" == "--kill-after=1s" && "$2" == "5s" ]] || return 1
  shift 2
  "$@"
}
journalctl() {
  if [[ "$1" == "-k" ]]; then
    printf 'type=1420 audit(123:456): ipe_op=EXECUTE enforcing=0 path="%s" rule="DEFAULT op=EXECUTE action=DENY"\n' "$probe"
  fi
}
dmesg() { :; }
` + aclIPEAuditCollectorForTest(t)
	cmd := exec.Command(aclIPETestBash(t), "-c", script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	require.NoError(t, cmd.Run(), stderr.String())
	_, err := validateACLIPEAuditRecord(stdout.String())
	require.NoError(t, err, stdout.String())
}

func TestACLIPEAuditCollectorTimeoutAndDeadline(t *testing.T) {
	const probe = "/var/tmp/acl-ipe-audit.aB123456/probe"
	for _, tc := range []struct {
		name, script, want string
		wantFailure        bool
	}{
		{
			name: "hung native collector falls back to kernel",
			script: `timeout() {
  [[ "$1" == "--kill-after=1s" && "$2" == "5s" ]] || return 1
  shift 2
  if [[ "$1" == journalctl && "$2" == "-b" ]]; then
    echo 'simulated hung native audit read' >&2
    command sleep 0.01
    return 124
  fi
  "$@"
}
journalctl() {
  if [[ "$1" == "-k" ]]; then
    printf 'type=1420 audit(123:456): ipe_op=EXECUTE enforcing=0 path="%s" rule="DEFAULT op=EXECUTE action=DENY"\n' "$probe"
  fi
}
dmesg() { :; }`,
			want: "ACL IPE audit collector timed out after 5s",
		},
		{
			name: "all collectors time out",
			script: `timeout() {
  [[ "$1" == "--kill-after=1s" && "$2" == "5s" ]] || return 1
  return 124
}`,
			want:        "native audit journal unavailable: ACL IPE audit collector timed out after 5s",
			wantFailure: true,
		},
		{
			name: "overall deadline stops retries",
			script: `timeout() {
  [[ "$1" == "--kill-after=1s" && "$2" == "5s" ]] || return 1
}
sleep() {
  echo 'sleep invoked' >&2
  SECONDS=$((deadline + 1))
}`,
			want:        "within the 30-second poll budget",
			wantFailure: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			script := `set -euo pipefail
probe="` + probe + `"
sudo() {
  [[ "$1" == "-n" && "$2" == "timeout" ]] || return 1
  shift
  "$@"
}
` + tc.script + "\n" + aclIPEAuditCollectorForTest(t)
			cmd := exec.Command(aclIPETestBash(t), "-c", script)
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			require.Contains(t, stderr.String(), tc.want)
			if tc.wantFailure {
				require.Error(t, err)
				require.Empty(t, stdout.String())
			} else {
				require.NoError(t, err, stderr.String())
				_, err = validateACLIPEAuditRecord(stdout.String())
				require.NoError(t, err, stdout.String())
			}
			if tc.name == "overall deadline stops retries" {
				require.Equal(t, 1, strings.Count(stderr.String(), "sleep invoked"))
			}
			if tc.name == "all collectors time out" {
				require.Contains(t, stderr.String(), "kernel journal unavailable: ACL IPE audit collector timed out after 5s")
				require.Contains(t, stderr.String(), "dmesg unavailable: ACL IPE audit collector timed out after 5s")
			}
		})
	}
}

func TestACLIPEAuditRecord(t *testing.T) {
	const probe = "/var/tmp/acl-ipe-audit.aB123456/probe"
	event := `type=1420 audit(123:456): ipe_op=EXECUTE ipe_hook=BPRM enforcing=0 pid=42 path="` +
		probe + `" rule="DEFAULT op=EXECUTE action=DENY"`
	message := strings.TrimPrefix(event, "type=1420 ")
	native := fmt.Sprintf(`{"_TRANSPORT":"audit","_AUDIT_TYPE":"1420","MESSAGE":%q}`, message)
	for _, tc := range []struct {
		name, output, wantErr string
	}{
		{"matching kernel event", "probe=" + probe + "\n" + event, ""},
		{"matching native-only event", "probe=" + probe + "\n" + native, ""},
		{"wrong native audit type", "probe=" + probe + "\n" + strings.Replace(native, `"1420"`, `"1421"`, 1), "no permissive"},
		{"wrong native transport", "probe=" + probe + "\n" + strings.Replace(native, `"audit"`, `"kernel"`, 1), "no permissive"},
		{"native wrong path", "probe=" + probe + "\n" + strings.Replace(native, probe, "/var/tmp/acl-ipe-audit.ZZ123456/probe", 1), "no permissive"},
		{"native wrong rule", "probe=" + probe + "\n" + strings.Replace(native, "action=DENY", "action=ALLOW", 1), "no permissive"},
		{"native enforcing", "probe=" + probe + "\n" + strings.Replace(native, "enforcing=0", "enforcing=1", 1), "no permissive"},
		{"native wrong operation", "probe=" + probe + "\n" + strings.Replace(native, "ipe_op=EXECUTE", "ipe_op=READ", 1), "no permissive"},
		{"native malformed entry", "probe=" + probe + "\n" + `{"_TRANSPORT":"audit","MESSAGE":"` + probe, "decode native IPE audit"},
		{"no path", event, "missing unique"},
		{"wrong path", "probe=/var/tmp/acl-ipe-audit.bad/probe\n" + event, "invalid"},
		{"wrong rule", "probe=" + probe + "\n" + strings.Replace(event, "action=DENY", "action=ALLOW", 1), "no permissive"},
		{"enforcing", "probe=" + probe + "\n" + strings.Replace(event, "enforcing=0", "enforcing=1", 1), "no permissive"},
		{"invalid op", "probe=" + probe + "\n" + strings.Replace(event, "ipe_op=EXECUTE", "ipe_op=READ", 1), "no permissive"},
		{"different probe", "probe=" + probe + "\n" + strings.Replace(event, probe, "/var/tmp/acl-ipe-audit.ZZ123456/probe", 1), "no permissive"},
		{"split fields", "probe=" + probe + "\n" + strings.Replace(event, " enforcing=0", "\n enforcing=0", 1), "no permissive"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := validateACLIPEAuditRecord(tc.output)
			if tc.wantErr == "" {
				require.NoError(t, err)
				require.Equal(t, probe, got)
			} else {
				require.ErrorContains(t, err, tc.wantErr)
			}
		})
	}
}

func TestACLIPEAuditDenyRequiresExplicitAuditMode(t *testing.T) {
	acl := &Scenario{Name: "ACL"}
	t.Setenv(aclIPEModeEnv, "")
	require.NoError(t, ValidateACLIPEAuditDeny(context.Background(), acl))
	t.Setenv(aclIPEModeEnv, "off")
	require.NoError(t, ValidateACLIPEAuditDeny(context.Background(), acl))
	require.Empty(t, acl.adoTestCases)
}
