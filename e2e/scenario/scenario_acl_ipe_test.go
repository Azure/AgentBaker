package scenario

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Exercise the script's predicates with synthetic state and logs, without a VM
// or changes to the machine's IPE policy. The script's live entry point is not run.
func runACLScriptHelper(t *testing.T, script, command string, env ...string) error {
	t.Helper()
	path := filepath.Join(t.TempDir(), "acl-helpers.sh")
	require.NoError(t, os.WriteFile(path, []byte(script), 0o600))
	cmd := exec.Command("bash", "-c", "source \"$1\"\n"+command, "acl-test", filepath.ToSlash(path))
	cmd.Env = append(os.Environ(), env...)
	output, err := cmd.CombinedOutput()
	t.Logf("%s", output)
	return err
}

func TestIPEAuditProfilePreservesOtherKeys(t *testing.T) {
	require.Equal(t, "ipe=audit", ipeAuditProfile(""))
	require.Equal(t, "selinux=enforcing,other=value,ipe=audit",
		ipeAuditProfile("ipe=off,selinux=enforcing,ipe=disabled,other=value"))
}

func TestIPEAuditPolicyState(t *testing.T) {
	for _, tc := range []struct {
		name, enforce, active string
		ok                    bool
	}{
		{"active audit", "0", "1", true},
		{"enforcing", "1", "1", false},
		{"inactive", "0", "0", false},
		{"missing policy", "0", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "policies", "acl_ipe_boot_policy")
			require.NoError(t, os.MkdirAll(dir, 0o700))
			require.NoError(t, os.WriteFile(filepath.Join(root, "enforce"), []byte(tc.enforce), 0o600))
			if tc.active != "" {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "active"), []byte(tc.active), 0o600))
			}
			err := runACLScriptHelper(t, aclIPEAuditScript, `ipe_check_audit_mode "$IPE_ROOT"`, "IPE_ROOT="+filepath.ToSlash(root))
			require.Equal(t, tc.ok, err == nil)
		})
	}
}

func TestIPEAuditRequiresSignedVerity(t *testing.T) {
	hash := strings.Repeat("a", 64)
	cmdline := "quiet usrhash=" + hash + " audit=1"
	table := "0 1 verity 1 7:0 7:1 4096 4096 1 1 sha256 " + hash + " salt"
	signedTable := table + " 2 root_hash_sig_key_desc key"
	for _, tc := range []struct {
		name, table, cmdline string
		ok                   bool
	}{
		{"signed matching hash", signedTable, cmdline, true},
		{"uppercase hash", strings.Replace(signedTable, hash, strings.ToUpper(hash), 1), "usrhash=" + strings.ToUpper(hash), true},
		{"missing signature", table, cmdline, false},
		{"missing signature key", table + " 1 root_hash_sig_key_desc", cmdline, false},
		{"wrong mapping type", strings.Replace(signedTable, " verity ", " linear ", 1), cmdline, false},
		{"wrong hash", strings.Replace(signedTable, hash, strings.Repeat("b", 64), 1), cmdline, false},
		{"wrong algorithm", strings.Replace(signedTable, " sha256 ", " sha512 ", 1), cmdline, false},
		{"missing boot hash", signedTable, "quiet", false},
		{"malformed boot hash", signedTable, "usrhash=invalid", false},
		{"duplicate boot hash", signedTable, cmdline + " usrhash=" + hash, false},
		{"different parameter", signedTable, "systemd.verity_usr_hash=" + hash, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := runACLScriptHelper(t, aclIPEAuditScript, `ipe_check_signed_verity "$TABLE" "$CMDLINE"`, "TABLE="+tc.table, "CMDLINE="+tc.cmdline)
			require.Equal(t, tc.ok, err == nil)
		})
	}
}

func TestACLDefaultSnapshotterConfig(t *testing.T) {
	for _, tc := range []struct {
		name, dump string
		ok         bool
	}{
		{"overlayfs", "[plugins.'io.containerd.cri.v1.images']\n  snapshotter = 'overlayfs'", true},
		{"erofs", "[plugins.'io.containerd.cri.v1.images']\n  snapshotter = 'erofs'", false},
		{"missing key", "[plugins.'io.containerd.cri.v1.images']\n  discard_unpacked_layers = false", false},
		{"wrong section", "[plugins.'io.containerd.cri.v1.runtime']\n  snapshotter = 'overlayfs'", false},
		{"nested override", "[plugins.'io.containerd.cri.v1.images'.runtime_platform.runc]\n  snapshotter = 'overlayfs'", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := runACLScriptHelper(t, aclDefaultSnapshotterScript, `acl_check_overlayfs_config <<< "$DUMP"`, "DUMP="+tc.dump)
			require.Equal(t, tc.ok, err == nil)
		})
	}
}

func TestIPEAuditRuntimeRequiresEROFS(t *testing.T) {
	dump := `[plugins."io.containerd.cri.v1.images"]
snapshotter = "erofs"
[plugins."io.containerd.snapshotter.v1.erofs"]
enable_dmverity_referrers = true
[plugins."io.containerd.service.v1.diff-service"]
default = ["erofs", "walking"]
`
	require.NoError(t, runACLScriptHelper(t, aclIPEAuditScript, `ipe_check_containerd <<< "$DUMP"`, "DUMP="+dump))
	for _, bad := range []string{
		strings.ReplaceAll(dump, `snapshotter = "erofs"`, `snapshotter = "overlayfs"`),
		strings.ReplaceAll(dump, "enable_dmverity_referrers = true", "enable_dmverity_referrers = false"),
		strings.ReplaceAll(dump, `["erofs", "walking"]`, `["walking"]`),
	} {
		require.Error(t, runACLScriptHelper(t, aclIPEAuditScript, `ipe_check_containerd <<< "$DUMP"`, "DUMP="+bad))
	}
}

func TestIPEAuditEvidenceCorrelation(t *testing.T) {
	event := `type=1420 pid=123 path="/var/tmp/acl-ipe-test" ipe_op=EXECUTE ipe_hook=BPRM_CHECK enforcing=0 rule="DEFAULT op=EXECUTE action=DENY"`
	require.NoError(t, runACLScriptHelper(t, aclIPEAuditScript, `ipe_denial 123 /var/tmp/acl-ipe-test <<< "$EVENT"`, "EVENT="+event))
	require.NoError(t, runACLScriptHelper(t, aclIPEAuditScript, `ipe_denial 123 /var/tmp/acl-ipe-test <<< "$EVENT"`, "EVENT="+strings.ReplaceAll(event, "type=1420", "AUDIT1420")))
	for _, bad := range []string{
		"", strings.ReplaceAll(event, "pid=123", "pid=1234"),
		strings.ReplaceAll(event, "acl-ipe-test", "acl-ipe-other"),
		strings.ReplaceAll(event, "enforcing=0", "enforcing=1"),
		strings.ReplaceAll(event, "action=DENY", "action=ALLOW"),
	} {
		require.Error(t, runACLScriptHelper(t, aclIPEAuditScript, `ipe_denial 123 /var/tmp/acl-ipe-test <<< "$EVENT"`, "EVENT="+bad))
	}

	require.NoError(t, runACLScriptHelper(t, aclIPEAuditScript, `ipe_check_audit_window boot boot "$EVENT"`, "EVENT="+event))
	require.Error(t, runACLScriptHelper(t, aclIPEAuditScript, `ipe_check_audit_window old new "$EVENT"`, "EVENT="+event))
	for _, loss := range []string{
		"audit_lost=1",
		"audit: backlog limit exceeded",
		"Suppressed 25 messages",
		"kauditd_printk_skb: 12 callbacks suppressed",
		"other_caller: 3 callbacks suppressed",
		"Missed 7 kernel messages",
	} {
		require.Error(t, runACLScriptHelper(t, aclIPEAuditScript, `ipe_check_audit_window boot boot "$EVENT"`, "EVENT="+event+"\n"+loss))
	}
}

func TestIPEAuditUsrExecution(t *testing.T) {
	// A denial for a shared library is also a failure of the /usr execution path,
	// even when the executable itself has no denial record.
	event := `AUDIT1420 pid=321 path="/usr/bin/true" ipe_op=EXECUTE ipe_hook=BPRM_CHECK enforcing=0 rule="DEFAULT op=EXECUTE action=DENY"`
	for _, tc := range []struct {
		name, logs string
		ok         bool
	}{
		{"no denial", "", true},
		{"other process", strings.ReplaceAll(event, "pid=321", "pid=123"), true},
		{"base executable denial", event, false},
		{"dependency denial", strings.NewReplacer("/usr/bin/true", "/usr/lib64/libc.so.6", "BPRM_CHECK", "MMAP_CHECK").Replace(event), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := runACLScriptHelper(t, aclIPEAuditScript, `ipe_check_usr_audit 321 <<< "$EVENT"`, "EVENT="+tc.logs)
			require.Equal(t, tc.ok, err == nil)
		})
	}
	require.Error(t, runACLScriptHelper(t, aclIPEAuditScript, `ipe_check_usr_audit 0 <<< ""`))
	require.Error(t, runACLScriptHelper(t, aclIPEAuditScript, `ipe_denial() { return 2; }; ipe_check_usr_audit 321 <<< ""`))
}

func TestIPEAuditDriverLossCollection(t *testing.T) {
	const command = `
ipe_read_journal() {
    [[ "$1" == boot ]] || return 2
    case "$2:$3" in
        audit:audit-start|kernel:kernel-start) : ;;
        driver:driver-start)
            [[ "$DRIVER_READ_FAILURE" == 0 ]] || return 2
            printf '%s\n' "$DRIVER_LOGS"
            ;;
        *) return 2 ;;
    esac
}
cat() {
    [[ "$1" == /proc/sys/kernel/random/boot_id ]] || return 2
    printf 'boot\n'
}
ipe_collect_audit_logs boot audit-start kernel-start driver-start
`
	for _, tc := range []struct {
		name, logs, readFailure string
		ok                      bool
	}{
		{"empty driver window", "", "0", true},
		{"lost kernel records", "Missed 7 kernel messages", "0", false},
		{"suppressed records", "Suppressed 25 messages", "0", false},
		{"unreadable driver", "", "1", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := runACLScriptHelper(t, aclIPEAuditScript, command, "DRIVER_LOGS="+tc.logs, "DRIVER_READ_FAILURE="+tc.readFailure)
			require.Equal(t, tc.ok, err == nil)
		})
	}
}
