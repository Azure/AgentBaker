package e2e

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/Azure/agentbaker/e2e/config"
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
		// there is nothing to time. Log it plainly and pass, rather than failing on absent
		// data or reporting a misleading zero. Validators no longer control test outcome
		// (see 70d6199c3e), so this cannot skip the test from here.
		s.Logger.Logf("NO BOOTSTRAP HOTFIX TIMING: %v (no hotfix ran on this node)", err)
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

// Ubuntu2204_HotfixBootstrapTiming records how long the ANC hotfix install takes during
// real provisioning, on a node that has not yet joined the cluster.
//
// Why this exists: timings collected over SSH on an already-Ready node measure a warm apt
// cache on an idle machine. That produced ~1.7s for scoped apt against ~0.2s for a serial
// approximation of the fast path, well under the ~10.2s and 22s+ previously reported from
// AgentBaker e2e. The gap is the environment, not the code. During bootstrap
// /var/lib/apt/lists may be cold, CPU and IO contend with image pulls and service startup,
// dpkg may be mid-operation, and networking has just come up. This scenario reads the
// timing the node recorded while all of that was true, so it is a genuine bootstrap-time
// measurement rather than an approximation of one.
//
// It reports rather than asserting a threshold: a single sample is not a budget, and a
// flaky perf gate is worse than no gate.
var _ = Register(&Scenario{
	Name:        "Ubuntu2204_HotfixBootstrapTiming",
	Description: "records ANC hotfix install duration during bootstrap, before the node joins",
	Config: Config{
		Cluster:   ClusterKubenet,
		VHD:       config.VHDUbuntu2204Gen2Containerd,
		Validator: validateHotfixBootstrapTiming,
	},
})

// AzureLinuxV3_HotfixBootstrapTiming is the dnf counterpart. Azure Linux is measured
// separately because it was the slowest observed path: 21.23s on a standalone node against
// 13.20s for Ubuntu 22.04, both including binary staging. dnf, its metadata handling, and
// the RPM fast path (repomd.xml plus primary.xml.gz, rather than InRelease plus
// Packages.gz) are a different code path with a different cost, so an Ubuntu number does
// not stand in for it.
var _ = Register(&Scenario{
	Name:        "AzureLinuxV3_HotfixBootstrapTiming",
	Description: "records ANC hotfix install duration during bootstrap on Azure Linux 3 (dnf)",
	Config: Config{
		Cluster:   ClusterKubenet,
		VHD:       config.VHDAzureLinuxV3Gen2,
		Validator: validateHotfixBootstrapTiming,
	},
})
