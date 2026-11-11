package scenario

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSSHServiceDisabledScript(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is required for Linux SSH validation tests")
	}

	tests := []struct {
		name            string
		osID            string
		variant         string
		active          string
		enabled         string
		stateUnit       string
		listeners       string
		ssStatus        string
		ubuntuSocket    string
		missingPolicy   bool
		conditionResult string
		want            string
		wantError       bool
	}{
		{name: "ACL", osID: "azurelinux", variant: "azurecontainerlinux", want: "sshd.socket: active=inactive enabled=disabled"},
		{name: "legacy ACL", osID: "azurecontainerlinux", want: "sshd.socket: active=inactive enabled=disabled"},
		{name: "Ubuntu", osID: "ubuntu", want: "ssh.service: active=inactive enabled=disabled"},
		{name: "Ubuntu with socket activation", osID: "ubuntu", ubuntuSocket: "true", want: "ssh.socket: active=inactive enabled=disabled"},
		{name: "Ubuntu socket enabled but blocked", osID: "ubuntu", ubuntuSocket: "true", enabled: "enabled", stateUnit: "ssh.socket", want: "SUCCESS:"},
		{name: "Ubuntu socket active", osID: "ubuntu", ubuntuSocket: "true", active: "active", stateUnit: "ssh.socket", want: "ssh.socket is not inactive", wantError: true},
		{name: "Azure Linux", osID: "azurelinux", want: "sshd.service: active=inactive enabled=disabled"},
		{name: "Mariner", osID: "mariner", want: "sshd.service: active=inactive enabled=disabled"},
		{name: "active socket", active: "active", want: "sshd.socket is not inactive", wantError: true},
		{name: "enabled socket remains blocked", enabled: "enabled", want: "SUCCESS:"},
		{name: "external masked socket", enabled: "masked", want: "SUCCESS:"},
		{name: "active service", active: "active", stateUnit: "sshd.service", want: "sshd.service is not inactive", wantError: true},
		{name: "enabled service remains blocked", enabled: "enabled", stateUnit: "sshd.service", want: "SUCCESS:"},
		{name: "missing unit", enabled: "not-found", want: "sshd.socket is not loaded", wantError: true},
		{name: "stop alone is insufficient", missingPolicy: true, want: "persistent SSH disable marker is missing", wantError: true},
		{name: "condition bypass", conditionResult: "yes", want: "not blocked by its persistent condition", wantError: true},
		{name: "inspection error", active: "unknown", want: "sshd.socket is not inactive", wantError: true},
		{name: "IPv4 listener", listeners: "LISTEN 0 128 0.0.0.0:22 0.0.0.0:*", want: "TCP port 22 is listening", wantError: true},
		{name: "IPv6 listener", listeners: "LISTEN 0 128 [::]:22 [::]:*", want: "TCP port 22 is listening", wantError: true},
		{name: "ss failed", ssStatus: "1", want: "unable to inspect TCP port 22", wantError: true},
		{name: "ss missing", ssStatus: "127", want: "unable to inspect TCP port 22", wantError: true},
		{name: "ss failed with output", ssStatus: "1", listeners: "partial output", want: "unable to inspect TCP port 22", wantError: true},
	}

	const mocks = `
function .() {
    ID="${TEST_OS:-azurelinux}"
    VARIANT_ID="${TEST_VARIANT}"
    if [[ -z "$TEST_OS" ]]; then VARIANT_ID=azurecontainerlinux; fi
}
systemctl() {
	local active="${TEST_ACTIVE:-inactive}" enabled="${TEST_ENABLED:-disabled}"
	if [[ -n "$TEST_STATE_UNIT" && "$2" != "$TEST_STATE_UNIT" ]]; then
		active=inactive
		enabled=disabled
	fi
    if [[ "$ID" == ubuntu ]]; then
        if [[ "$2" == ssh.socket ]]; then
            if [[ "$1" == show && "$3" == --property=LoadState && "${TEST_UBUNTU_SOCKET:-}" != true ]]; then
                echo not-found
                return 0
            fi
            [[ "${TEST_UBUNTU_SOCKET:-}" == true ]] || return 1
        else
            [[ "$2" == ssh.service ]] || return 1
        fi
    elif [[ "$ID" != azurecontainerlinux && "$VARIANT_ID" != azurecontainerlinux ]]; then
        [[ "$2" == sshd.service ]] || return 1
    fi
    case "$1" in
        cat) return 0 ;;
        start) return 0 ;;
        show)
            case "$3" in
                --property=LoadState)
                    case "$enabled" in
                        masked|not-found) echo "$enabled" ;;
                        *) echo loaded ;;
                    esac ;;
                --property=ConditionResult) echo "${TEST_CONDITION_RESULT:-no}" ;;
                *) return 1 ;;
            esac ;;
		is-active) printf '%s\n' "$active"; [[ "$active" == active ]] && return 0; return 3 ;;
		is-enabled) printf '%s\n' "$enabled"; [[ "$enabled" == enabled ]] && return 0; return 1 ;;
        *) return 1 ;;
    esac
}
ss() {
    [[ "$#" == 3 && "$1" == -H && "$2" == -ltn && "$3" == 'sport = :22' ]] || return 99
    printf '%s' "$TEST_LISTENERS"
    return "${TEST_SS_STATUS:-0}"
}
`
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policyDir := t.TempDir()
			unitDir := t.TempDir()
			if !test.missingPolicy {
				if err := os.WriteFile(filepath.Join(policyDir, "disabled"), nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			for _, unit := range []string{"ssh.service", "sshd.service", "ssh.socket", "sshd.socket"} {
				dir := filepath.Join(unitDir, unit+".d")
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "90-aks-ssh-access.conf"),
					[]byte("[Unit]\nConditionPathExists=!"+policyDir+"/disabled\n"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, "bash", "-c", mocks+validateSSHServiceDisabledScript)
			command.Env = append(os.Environ(),
				"TEST_OS="+test.osID, "TEST_VARIANT="+test.variant,
				"TEST_ACTIVE="+test.active, "TEST_ENABLED="+test.enabled,
				"TEST_STATE_UNIT="+test.stateUnit,
				"TEST_UBUNTU_SOCKET="+test.ubuntuSocket,
				"SSH_POLICY_DIR="+policyDir, "SSH_SYSTEMD_UNIT_DIR="+unitDir,
				"TEST_CONDITION_RESULT="+test.conditionResult,
				"TEST_LISTENERS="+test.listeners, "TEST_SS_STATUS="+test.ssStatus)
			output, err := command.CombinedOutput()
			if (err != nil) != test.wantError {
				t.Fatalf("error = %v, wantError = %v; output: %s", err, test.wantError, output)
			}
			if !strings.Contains(string(output), test.want) {
				t.Fatalf("output missing %q: %s", test.want, output)
			}
			if strings.Contains(string(output), "SUCCESS:") == test.wantError {
				t.Fatalf("unexpected success marker: %s", output)
			}
		})
	}
}
