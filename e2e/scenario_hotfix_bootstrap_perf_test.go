package e2e

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ancOutputLog is written by the launcher during bootstrap, before the node joins the
// cluster. Reading timings out of it is what makes this a bootstrap-time measurement:
// the numbers were produced while cloud-init was still running, contending with image
// pulls and service startup on a cold apt cache. Timing the same commands over SSH on a
// Ready node measures a warm, idle machine instead -- a different, much friendlier
// environment that reads several seconds faster.
const ancOutputLog = "/var/log/azure/aks-node-controller.output"

// Both hotfix paths emit durationMs on completion, but with distinct messages, so a run
// cannot be silently misattributed to the wrong path. Note fastPathMsg has pmcMsg as a
// prefix; check for the fast path first.
const (
	hotfixFastPathMsg = "downloaded ANC hotfix through authenticated repository fast path"
	hotfixPMCMsg      = "downloaded ANC hotfix"
)

var durationMsRe = regexp.MustCompile(`durationMs=(\d+)`)

// hotfixPathTiming is one observation of a hotfix install during bootstrap.
type hotfixPathTiming struct {
	// FastPath is true when the repository fast path completed, false when the run fell
	// through to apt/dnf.
	FastPath bool
	Duration int64
	Line     string
}

// parseHotfixTiming extracts the hotfix timing from the ANC bootstrap log.
func parseHotfixTiming(log string) (*hotfixPathTiming, error) {
	var found *hotfixPathTiming
	for _, line := range strings.Split(log, "\n") {
		isFast := strings.Contains(line, hotfixFastPathMsg)
		isPMC := !isFast && strings.Contains(line, hotfixPMCMsg)
		if !isFast && !isPMC {
			continue
		}
		match := durationMsRe.FindStringSubmatch(line)
		if match == nil {
			return nil, fmt.Errorf("hotfix completion line carries no durationMs: %q", line)
		}
		ms, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse durationMs from %q: %w", line, err)
		}
		found = &hotfixPathTiming{FastPath: isFast, Duration: ms, Line: strings.TrimSpace(line)}
	}
	if found == nil {
		return nil, errors.New("no hotfix completion line found")
	}
	return found, nil
}

// validateHotfixBootstrapTiming reads the timing the node recorded for itself during
// bootstrap and reports it. It reads past events from the log rather than re-running any
// command, so the result is unaffected by the node now being warm and idle.
//
// It reports rather than asserting a threshold: a single sample is not a budget, and a
// flaky perf gate is worse than no gate. Run a scenario a few times and read the values.
func validateHotfixBootstrapTiming(ctx context.Context, s *Scenario) error {
	result, err := execScriptOnVMForScenarioValidateExitCode(
		ctx, s, "sudo cat "+ancOutputLog, 0,
		"could not read the ANC bootstrap log",
	)
	if err != nil {
		return err
	}

	timing, err := parseHotfixTiming(result.stdout + "\n" + result.stderr)
	if err != nil {
		// Not a failure of the code under test: if no hotfix was configured for this run
		// there is nothing to time. Say so plainly rather than reporting a misleading zero.
		s.Logger.Logf("no hotfix timing available: %v", err)
		s.T.Skip("no hotfix ran during bootstrap; nothing to measure")
		return nil
	}

	path := "package-manager (apt/dnf)"
	if timing.FastPath {
		path = "repository fast path"
	}
	s.Logger.Logf("BOOTSTRAP HOTFIX TIMING: distro=%s path=%s durationMs=%d",
		s.VHD.Name, path, timing.Duration)
	s.Logger.Logf("  source line: %s", timing.Line)
	s.Logger.Logf("  measured during provisioning, on a node that had not yet joined")

	// Surface the fallback reason when the fast path did not win, so a slow run can be
	// explained rather than guessed at.
	if !timing.FastPath {
		fallback, ferr := execScriptOnVMForScenario(ctx, s,
			"sudo grep -F 'falling back to package manager' "+ancOutputLog+" || true")
		if ferr == nil && strings.TrimSpace(fallback.stdout) != "" {
			s.Logger.Logf("  fell back because: %s", strings.TrimSpace(fallback.stdout))
		}
	}
	return nil
}

// Test_Ubuntu2204_HotfixBootstrapTiming records how long the ANC hotfix install takes
// during real provisioning, on a node that has not yet joined the cluster.
//
// Why this exists: timings collected over SSH on an already-Ready node measure a warm
// apt cache on an idle machine. That came out at ~3.5s, far below the ~10.2s and 22s+
// previously reported from AgentBaker e2e. The gap is the environment, not the code.
// During bootstrap /var/lib/apt/lists may be cold, CPU and IO contend with image pulls
// and service startup, dpkg may be mid-operation, and networking has just come up. This
// scenario reads the timing the node recorded while all of that was true, so it is a
// genuine bootstrap-time measurement rather than an approximation of one.
func Test_Ubuntu2204_HotfixBootstrapTiming(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "records ANC hotfix install duration during bootstrap, before the node joins",
		Config: Config{
			Cluster:   ClusterKubenet,
			VHD:       config.VHDUbuntu2204Gen2Containerd,
			Validator: validateHotfixBootstrapTiming,
		},
	})
}

// Test_AzureLinuxV3_HotfixBootstrapTiming is the dnf counterpart. Azure Linux is measured
// separately because it was the slowest observed path: 21.23s on a standalone node against
// 13.20s for Ubuntu 22.04, both including binary staging. dnf, its metadata handling, and
// the RPM fast path (repomd.xml plus primary.xml.gz, rather than InRelease plus
// Packages.gz) are a different code path with a different cost, so an Ubuntu number does
// not stand in for it.
func Test_AzureLinuxV3_HotfixBootstrapTiming(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "records ANC hotfix install duration during bootstrap on Azure Linux 3 (dnf)",
		Config: Config{
			Cluster:   ClusterKubenet,
			VHD:       config.VHDAzureLinuxV3Gen2,
			Validator: validateHotfixBootstrapTiming,
		},
	})
}

func TestParseHotfixTiming(t *testing.T) {
	t.Run("fast path", func(t *testing.T) {
		log := `time=2026-09-03T01:00:00Z level=INFO msg="downloading ANC hotfix" current=202608.21.0 target=202608.21.1
time=2026-09-03T01:00:01Z level=INFO msg="downloaded ANC hotfix through authenticated repository fast path" target=202608.21.1 format=deb path=/opt/azure/containers/aks-node-controller-hotfix durationMs=1843`
		timing, err := parseHotfixTiming(log)
		require.NoError(t, err)
		assert.True(t, timing.FastPath)
		assert.Equal(t, int64(1843), timing.Duration)
	})

	// The fast-path message contains the package-manager message as a prefix, so a naive
	// substring check would report a fast-path run as a package-manager run.
	t.Run("package manager path is not confused with fast path", func(t *testing.T) {
		log := `time=2026-09-03T01:00:00Z level=WARN msg="safe repository download unavailable, falling back to package manager" version=202608.21.1
time=2026-09-03T01:00:13Z level=INFO msg="downloaded ANC hotfix" target=202608.21.1 path=/opt/azure/containers/aks-node-controller-hotfix durationMs=13204`
		timing, err := parseHotfixTiming(log)
		require.NoError(t, err)
		assert.False(t, timing.FastPath, "this run fell back and must not be reported as the fast path")
		assert.Equal(t, int64(13204), timing.Duration)
	})

	t.Run("no hotfix ran", func(t *testing.T) {
		_, err := parseHotfixTiming("time=... msg=\"ANC version not targeted by hotfix, skipping download\"")
		assert.Error(t, err)
	})

	// A completion line without durationMs means the build predates the instrumentation;
	// reporting zero would look like an impossibly fast run.
	t.Run("completion without durationMs is an error, not a zero", func(t *testing.T) {
		_, err := parseHotfixTiming(`msg="downloaded ANC hotfix" target=202608.21.1`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no durationMs")
	})
}
