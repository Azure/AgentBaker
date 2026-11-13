package scenario

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestIGDiagnosticsValidationScript(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is required for IG diagnostics validation tests")
	}

	const mocks = `
systemctl() {
    printf 'systemctl %s\n' "$*" >> "$TEST_CALLS"
    case "$1" in
        is-enabled) [ "$TEST_ENABLED" = true ] ;;
        is-active)
            [ "$TEST_ACTIVE" = true ] &&
                { [ "$TEST_STARTING" != true ] || [ -f "$TEST_POLLED" ]; }
            ;;
        show) echo "ActiveState=failed" ;;
        *) return 99 ;;
    esac
}
python3() {
    printf 'reader %s\n' "$*" >> "$TEST_CALLS"
    [ "$#" = 3 ] && [ "$1" = "$IG_DIAGNOSTICS_READER" ] &&
        [ "$2" = --check-startup ] &&
        [ "$3" = "$IG_DIAGNOSTICS_RUNTIME_DIR/collectors.json" ] || return 99
    if [ "$TEST_REPORTS" != fresh ]; then
        echo "IG diagnostics unavailable: $TEST_REPORTS" >&2
        return 1
    fi
}
timeout() {
    printf 'timeout %s\n' "$*" >> "$TEST_CALLS"
    shift 3
    "$@"
}
sleep() {
    if [ "$TEST_STARTING" = true ] && [ ! -f "$TEST_POLLED" ]; then
        touch "$TEST_POLLED"
    else
        SECONDS=$((SECONDS + 120))
    fi
}
`

	cases := []struct {
		name        string
		missing     string
		version     string
		active      bool
		disabled    bool
		starting    bool
		reports     string
		want        string
		wantError   bool
		wantReader  bool
		wantService bool
	}{
		{name: "healthy", active: true, want: "PASS:", wantReader: true, wantService: true},
		{name: "startup in progress", active: true, starting: true, want: "PASS:", wantReader: true, wantService: true},
		{name: "old image", missing: "diagnostics-version", want: "SKIP: image has no"},
		{name: "future capability", version: "2", want: "SKIP: unsupported"},
		{name: "cgroup v1", missing: "sys/fs/cgroup/cgroup.controllers", want: "SKIP: continuous IG diagnostics requires cgroup v2"},
		{name: "missing ownership", missing: "skip_vhd_ig", want: "missing IG ownership marker", wantError: true},
		{name: "missing reader", missing: "reader", want: "requires Python and the packaged reader", wantError: true},
		{name: "disabled unit", disabled: true, want: "must be enabled", wantError: true, wantService: true},
		{name: "failed unit", want: "did not become active", wantError: true, wantService: true},
		{name: "active without manifest", active: true, missing: "collectors.json", want: "no readable collector manifest", wantError: true, wantService: true},
		{name: "stale collector", active: true, reports: "throttle: no fresh report", want: "throttle: no fresh report", wantError: true, wantReader: true, wantService: true},
		{name: "invalid generation", active: true, reports: "Incompatible collector manifest or boot generation", want: "boot generation", wantError: true, wantReader: true, wantService: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			directory := t.TempDir()
			sys := filepath.Join(directory, "sys", "fs", "cgroup")
			require.NoError(t, os.MkdirAll(sys, 0o700))
			files := map[string]string{
				"diagnostics-version":              "1\n",
				"skip_vhd_ig":                      "",
				"reader":                           "",
				"collectors.json":                  "{}",
				"sys/fs/cgroup/cgroup.controllers": "",
			}
			if tc.version != "" {
				files["diagnostics-version"] = tc.version + "\n"
			}
			for name, content := range files {
				if name == tc.missing {
					continue
				}
				require.NoError(t, os.WriteFile(filepath.Join(directory, name), []byte(content), 0o600))
			}

			reports := tc.reports
			if reports == "" {
				reports = "fresh"
			}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, "bash", "-c", mocks+validateIGDiagnosticsScript)
			calls := filepath.Join(directory, "calls")
			command.Env = append(os.Environ(),
				"IG_DIAGNOSTICS_CONFIG_DIR="+directory,
				"IG_DIAGNOSTICS_RUNTIME_DIR="+directory,
				"IG_DIAGNOSTICS_READER="+filepath.Join(directory, "reader"),
				"SYS_DIR="+filepath.Join(directory, "sys"),
				"TEST_CALLS="+calls,
				"TEST_POLLED="+filepath.Join(directory, "polled"),
				"TEST_ACTIVE="+strconv.FormatBool(tc.active),
				"TEST_ENABLED="+strconv.FormatBool(!tc.disabled),
				"TEST_STARTING="+strconv.FormatBool(tc.starting),
				"TEST_REPORTS="+reports)
			output, err := command.CombinedOutput()
			require.NoError(t, ctx.Err(), string(output))
			require.Equal(t, tc.wantError, err != nil, string(output))
			require.Equal(t, !tc.wantError && tc.wantReader, strings.Contains(string(output), "PASS:"))
			require.Contains(t, string(output), tc.want)
			trace, err := os.ReadFile(calls)
			if tc.wantService {
				require.NoError(t, err)
				require.Contains(t, string(trace), "systemctl is-enabled")
			} else {
				require.ErrorIs(t, err, os.ErrNotExist)
			}
			require.Equal(t, tc.wantReader, strings.Contains(string(trace), "reader "))
			require.NotContains(t, string(trace), "systemctl start")
			require.NotContains(t, string(trace), "systemctl restart")
		})
	}
}
