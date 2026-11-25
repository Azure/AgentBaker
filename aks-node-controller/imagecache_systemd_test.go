package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Run with IMAGE_CACHE_SYSTEMD_TEST=1 against a private user's systemd manager.
func TestImageCacheSystemdLifecycle(t *testing.T) {
	if os.Getenv("IMAGE_CACHE_SYSTEMD_TEST") != "1" {
		t.Skip("requires a systemd user manager")
	}
	systemdRun, err := exec.LookPath("systemd-run")
	require.NoError(t, err)
	source, err := os.ReadFile("../parts/linux/cloud-init/artifacts/cse_main.sh")
	require.NoError(t, err)
	_, function, ok := strings.Cut(string(source), "prepullImages() (")
	require.True(t, ok)
	function, _, ok = strings.Cut(function, "\n)\n")
	require.True(t, ok)
	function = "prepullImages() (" + function + "\n)\n"

	for _, test := range []struct {
		name, mode, state string
		exitCode          int
		timeout           bool
	}{
		{name: "JSON survives launcher", mode: "json", state: "exited"},
		{name: "NBC survives launcher", mode: "nbc", state: "exited"},
		{name: "worker failure", mode: "json", state: "failed", exitCode: 7},
		{name: "worker timeout", mode: "json", state: "failed", timeout: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			id := fmt.Sprintf("image-cache-test-%d-%d", os.Getpid(), time.Now().UnixNano())
			parent, worker := id+"-parent.service", id+"-worker.service"
			t.Cleanup(func() {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				_ = exec.CommandContext(ctx, "systemctl", "--user", "stop", parent, worker).Run()
				_ = exec.CommandContext(ctx, "systemctl", "--user", "reset-failed", parent, worker).Run()
			})
			write := func(name, data string) string {
				path := filepath.Join(dir, name)
				require.NoError(t, os.WriteFile(path, []byte(data), 0700))
				return path
			}
			runtimeSeconds, runtimeProperty := 330, "5min 30s"
			if test.timeout {
				runtimeSeconds, runtimeProperty = 3, "3s"
			}
			// Use a user-owned unit and shorten only the timeout case.
			write("systemd-run", fmt.Sprintf(`#!/bin/bash
args=()
for arg in "$@"; do
    case "$arg" in
        --unit=aks-image-prefetch) args+=("--unit=%s");;
        --property=RuntimeMaxSec=330) args+=("--property=RuntimeMaxSec=%d");;
        *) args+=("$arg");;
    esac
done
exec %q --user "${args[@]}"
`, worker, runtimeSeconds, systemdRun))
			binary := write("selected-anc", fmt.Sprintf(`#!/bin/bash
set -eu
printf '%%s\n' "$ENABLE_NETWORKING_IMAGE_CACHE" "$ANC_IMAGE_CACHE_MODE" \
    "$ANC_IMAGE_CACHE_INPUT" "$HTTP_PROXY" "$SSL_CERT_FILE" \
    "$CONTAINERD_NAMESPACE" "${KUBELET_CLIENT_KEY-unset}" "$*" > %q
while [ ! -f %q ]; do sleep 0.05; done
exit %d
`, filepath.Join(dir, "started"), filepath.Join(dir, "release"), test.exitCode))
			launcher := write("launcher", fmt.Sprintf(`#!/bin/bash
set -eu
export PATH=%q:"$PATH"
export HTTP_PROXY='http://proxy.example:3128'
export SSL_CERT_FILE='/etc/custom certificates.pem'
export CONTAINERD_NAMESPACE=k8s.io
export KUBELET_CLIENT_KEY=must-not-be-forwarded
ENABLE_NETWORKING_IMAGE_CACHE=true
ANC_IMAGE_CACHE_MODE=%q
ANC_IMAGE_CACHE_INPUT='/opt/azure/config with spaces.json'
ANC_SELECTED_BIN_PATH=%q
%s
prepullImages
`, dir, test.mode, binary, function))
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			output, err := exec.CommandContext(ctx, systemdRun, "--user", "--unit="+parent,
				"--property=Type=exec", "--remain-after-exit", launcher).CombinedOutput()
			require.NoError(t, err, "%s", output)
			property := func(unit, name string) string {
				readCtx, readCancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer readCancel()
				value, readErr := exec.CommandContext(readCtx, "systemctl", "--user", "show",
					unit, "--property="+name, "--value").Output()
				if readErr != nil {
					return ""
				}
				return strings.TrimSpace(string(value))
			}
			require.Eventually(t, func() bool {
				return property(parent, "SubState") == "exited"
			}, 5*time.Second, 25*time.Millisecond, "launcher waited for blocked worker")
			require.Equal(t, "0", property(parent, "ExecMainStatus"))
			require.Eventually(t, func() bool {
				_, statErr := os.Stat(filepath.Join(dir, "started"))
				return statErr == nil
			}, 5*time.Second, 25*time.Millisecond)
			data, err := os.ReadFile(filepath.Join(dir, "started"))
			require.NoError(t, err)
			require.Equal(t, "true\n"+test.mode+"\n/opt/azure/config with spaces.json\n"+
				"http://proxy.example:3128\n/etc/custom certificates.pem\nk8s.io\nunset\n"+
				"prepull-images --source networking\n", string(data))
			require.Equal(t, runtimeProperty, property(worker, "RuntimeMaxUSec"))
			require.Equal(t, "10s", property(worker, "TimeoutStopUSec"))
			require.Equal(t, "control-group", property(worker, "KillMode"))
			group := property(worker, "ControlGroup")
			require.NotEmpty(t, group)
			require.NotEqual(t, property(parent, "ControlGroup"), group)
			require.NoError(t, exec.CommandContext(ctx, "systemctl", "--user", "stop", parent).Run())
			require.Equal(t, "running", property(worker, "SubState"))
			if !test.timeout {
				write("release", "")
			}
			require.Eventually(t, func() bool {
				return property(worker, "SubState") == test.state
			}, 5*time.Second, 25*time.Millisecond)
			if test.timeout {
				require.Equal(t, "timeout", property(worker, "Result"))
			} else {
				require.Equal(t, fmt.Sprint(test.exitCode), property(worker, "ExecMainStatus"))
			}
		})
	}
}
