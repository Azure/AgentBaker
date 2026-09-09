package scenario

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestSSHServiceDisabledScript(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is required for Linux SSH validation tests")
	}

	tests := []struct {
		name      string
		osID      string
		variant   string
		active    string
		enabled   string
		stateUnit string
		listeners string
		ssStatus  string
		want      string
		wantError bool
	}{
		{name: "ACL", osID: "azurelinux", variant: "azurecontainerlinux", want: "sshd.socket: active=inactive enabled=disabled"},
		{name: "legacy ACL", osID: "azurecontainerlinux", want: "sshd.socket: active=inactive enabled=disabled"},
		{name: "Ubuntu", osID: "ubuntu", want: "ssh.service: active=inactive enabled=disabled"},
		{name: "Azure Linux", osID: "azurelinux", want: "sshd.service: active=inactive enabled=disabled"},
		{name: "Mariner", osID: "mariner", want: "sshd.service: active=inactive enabled=disabled"},
		{name: "active socket", active: "active", want: "sshd.socket is not inactive", wantError: true},
		{name: "enabled socket", enabled: "enabled", want: "sshd.socket is not disabled", wantError: true},
		{name: "masked socket", enabled: "masked", want: "sshd.socket is not disabled", wantError: true},
		{name: "active service", active: "active", stateUnit: "sshd.service", want: "sshd.service is not inactive", wantError: true},
		{name: "enabled service", enabled: "enabled", stateUnit: "sshd.service", want: "sshd.service is not disabled", wantError: true},
		{name: "missing unit", enabled: "not-found", want: "sshd.socket is not disabled", wantError: true},
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
        [[ "$2" == ssh.service ]] || return 1
    elif [[ "$ID" != azurecontainerlinux && "$VARIANT_ID" != azurecontainerlinux ]]; then
        [[ "$2" == sshd.service ]] || return 1
    fi
    case "$1" in
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
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, "bash", "-c", mocks+validateSSHServiceDisabledScript)
			command.Env = append(os.Environ(),
				"TEST_OS="+test.osID, "TEST_VARIANT="+test.variant,
				"TEST_ACTIVE="+test.active, "TEST_ENABLED="+test.enabled,
				"TEST_STATE_UNIT="+test.stateUnit,
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
