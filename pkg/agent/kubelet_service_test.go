// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"strings"
	"testing"

	"github.com/Azure/agentbaker/parts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseSystemdUnit returns a section -> directive -> values map for a systemd unit file.
// Directives may legitimately repeat (ExecStartPre), so values are collected in order.
func parseSystemdUnit(t *testing.T, content string) map[string]map[string][]string {
	t.Helper()
	unit := map[string]map[string][]string{}
	section := ""
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.Trim(line, "[]")
			if _, ok := unit[section]; !ok {
				unit[section] = map[string][]string{}
			}
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		require.NotEmpty(t, section, "directive %q appears before any section header", line)
		key = strings.TrimSpace(key)
		unit[section][key] = append(unit[section][key], strings.TrimSpace(value))
	}
	return unit
}

func firstValue(unit map[string]map[string][]string, section, key string) (string, bool) {
	values := unit[section][key]
	if len(values) == 0 {
		return "", false
	}
	return values[0], true
}

// TestKubeletServiceWatchdogRecovery locks in the systemd settings that let kubelet recover
// from a watchdog timeout.
//
// WatchdogSec=60s is written to /etc/systemd/system/kubelet.service.d/10-watchdog.conf by
// ensureKubelet() for Kubernetes >= 1.32, which turns a stalled kubelet into a SIGABRT. The Go
// runtime handles SIGABRT by dumping goroutines and calling exit(2), so systemd observes
// "Main process exited, code=exited, status=2". Recovery from that state depends on:
//
//   - KillMode=mixed: under the default control-group mode systemd sends the watchdog SIGABRT to
//     every process in the cgroup and then holds the unit in "deactivating" until the cgroup
//     drains. A single wedged child blocks the restart, and no restart job is logged while the
//     unit deactivates. mixed signals kubelet alone; the final SIGKILL still covers the cgroup.
//   - TimeoutStopSec: bounds each stop step (stop-watchdog, final-SIGTERM, final-SIGKILL) instead
//     of inheriting DefaultTimeoutStopSec.
//   - StartLimitIntervalSec=0: systemd's default limiter (StartLimitBurst=5 within
//     StartLimitIntervalSec=10s) sets the unit result to start-limit-hit, and
//     service_shall_restart() returns false for Restart=always in that state. The unit then
//     stays failed with no further restart jobs until reset-failed or a reboot. Upstream
//     Kubernetes ships the same override.
//   - No RestartPreventExitStatus: it would exclude the exit code the watchdog abort produces.
func TestKubeletServiceWatchdogRecovery(t *testing.T) {
	raw, err := parts.Templates.ReadFile(kubeletSystemdService)
	require.NoError(t, err)
	unit := parseSystemdUnit(t, string(raw))

	restart, ok := firstValue(unit, "Service", "Restart")
	require.True(t, ok, "kubelet.service must set Restart")
	assert.Equal(t, "always", restart)

	startLimit, ok := firstValue(unit, "Unit", "StartLimitIntervalSec")
	require.True(t, ok, "kubelet.service must disable the systemd start rate limit, otherwise a "+
		"burst of fast failures permanently disables Restart=always and leaves the node without a kubelet")
	assert.Equal(t, "0", startLimit)

	assert.Empty(t, unit["Service"]["RestartPreventExitStatus"],
		"RestartPreventExitStatus must stay unset so the exit code produced by a watchdog SIGABRT still restarts kubelet")

	assert.NotEmpty(t, unit["Service"]["TimeoutStopSec"],
		"kubelet.service must bound the stop path so a wedged child process cannot hold the unit in 'deactivating'")

	killMode, ok := firstValue(unit, "Service", "KillMode")
	require.True(t, ok, "kubelet.service must set KillMode")
	assert.Equal(t, "mixed", killMode)

	// Preserved from before the watchdog recovery fix: SIGTERM is a graceful stop, not a failure.
	assert.Equal(t, []string{"143"}, unit["Service"]["SuccessExitStatus"])
}

// TestKubeletServiceSurvivesCommentStripping guards the CustomData delivery path. The unit is
// piped through removeComments() before it is gzipped into CustomData, so a directive that only
// survives in the VHD-baked copy would silently not apply to nodes provisioned from CustomData.
func TestKubeletServiceSurvivesCommentStripping(t *testing.T) {
	raw, err := parts.Templates.ReadFile(kubeletSystemdService)
	require.NoError(t, err)

	delivered := parseSystemdUnit(t, string(removeComments(raw)))

	for _, tc := range []struct {
		section string
		key     string
		want    string
	}{
		{"Unit", "StartLimitIntervalSec", "0"},
		{"Service", "Restart", "always"},
		{"Service", "RestartSec", "2"},
		{"Service", "KillMode", "mixed"},
		{"Service", "TimeoutStopSec", "30"},
		{"Service", "SuccessExitStatus", "143"},
	} {
		got, ok := firstValue(delivered, tc.section, tc.key)
		require.Truef(t, ok, "[%s] %s missing from the comment-stripped unit delivered via CustomData", tc.section, tc.key)
		assert.Equalf(t, tc.want, got, "[%s] %s", tc.section, tc.key)
	}
}
