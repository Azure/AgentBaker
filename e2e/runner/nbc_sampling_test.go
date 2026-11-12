package runner

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/scenario"
	"github.com/stretchr/testify/require"
)

func nbcCandidates() []*scenario.Scenario {
	var candidates []*scenario.Scenario
	for _, vhd := range []*config.Image{
		config.VHDUbuntu2204Gen2Containerd, config.VHDUbuntu2404Gen2Containerd,
		config.VHDUbuntu2604MinimalGen2Containerd, config.VHDAzureLinuxV3Gen2,
	} {
		for i := 0; i < 8; i++ {
			candidates = append(candidates, &scenario.Scenario{
				Name:   fmt.Sprintf("%s/case%d", vhd.Name, i),
				Config: scenario.Config{VHD: vhd},
			})
		}
	}
	return candidates
}

func sampleNames(sampled []*scenario.Scenario) []string {
	names := make([]string, len(sampled))
	for i, s := range sampled {
		names[i] = s.Name
	}
	return names
}

func TestNBCSamplingCoverageAndReproducibility(t *testing.T) {
	restoreRunnerConfig(t)
	candidates := nbcCandidates()
	sampled, err := sampleNBCScenarios(candidates, "build-123/2026-11-03")
	require.NoError(t, err)
	require.Len(t, sampled, 8)
	counts := map[string]int{}
	seen := map[string]bool{}
	for _, s := range sampled {
		require.True(t, s.ForceScriptlessNBCCSECmd)
		require.False(t, seen[s.Name])
		seen[s.Name] = true
		counts[s.VHD.Name]++
	}
	require.Len(t, counts, 4)
	for _, count := range counts {
		require.Equal(t, 2, count)
	}
	for _, s := range candidates {
		require.False(t, s.ForceScriptlessNBCCSECmd)
		require.NotContains(t, s.Name, "ScriptlessNBCCSECmd")
	}
	slices.Reverse(candidates)
	repeated, err := sampleNBCScenarios(candidates, "build-123/2026-11-03")
	require.NoError(t, err)
	require.Equal(t, sampleNames(sampled), sampleNames(repeated))
	changed, err := sampleNBCScenarios(candidates, "build-123/2026-11-04")
	require.NoError(t, err)
	require.NotEqual(t, sampleNames(sampled), sampleNames(changed))
}

func TestNBCSamplingHonorsFiltersAndRequiresEveryOS(t *testing.T) {
	restoreRunnerConfig(t)
	candidates := nbcCandidates()
	for i, s := range candidates {
		s.Tags.GPU = i%4 != 0
	}
	runnable, _, err := partitionScenarios(candidates, tagFilter{skip: "gpu=true"})
	require.NoError(t, err)
	sampled, err := sampleNBCScenarios(runnable, "seed")
	require.NoError(t, err)
	for _, s := range sampled {
		require.False(t, s.Tags.GPU)
	}
	_, err = sampleNBCScenarios(selectScenarios(runnable, []string{runnable[0].Name}), "seed")
	require.ErrorContains(t, err, "after filtering; found")
	_, err = sampleNBCScenarios(runnable[:len(runnable)-2], "seed")
	require.ErrorContains(t, err, "AzureLinuxV3")
}

func TestNBCSamplingRejectsUnsupportedModesAndCollisions(t *testing.T) {
	restoreRunnerConfig(t)
	config.Config.DisableScriptless = true
	_, err := sampleNBCScenarios(nbcCandidates(), "seed")
	require.ErrorContains(t, err, "cannot be combined")
	config.Config.DisableScriptless = false
	config.Config.TestPreProvision = true
	_, err = sampleNBCScenarios(nbcCandidates(), "seed")
	require.ErrorContains(t, err, "cannot be combined")
	config.Config.TestPreProvision = false
	candidates := nbcCandidates()
	sampled, err := sampleNBCScenarios(candidates, "seed")
	require.NoError(t, err)
	_, err = sampleNBCScenarios(append(candidates, sampled[0]), "seed")
	require.ErrorContains(t, err, "collides")
}

func TestNBCSamplingRealLinuxPipelinePool(t *testing.T) {
	restoreRunnerConfig(t)
	runnable, _, err := partitionScenarios(scenario.List(), tagFilter{skip: "os=windows,gpu=true,cvm=true"})
	require.NoError(t, err)
	sampled, err := sampleNBCScenarios(runnable, "189022832/2026-11-03")
	require.NoError(t, err)
	require.Len(t, sampled, 8)
	for _, s := range sampled {
		t.Log(s.Name)
		require.False(t, s.SkipDefaultValidation)
		require.False(t, s.SkipSSHConnectivityValidation)
		require.Empty(t, s.ExpectedError)
	}
}

func TestNBCSamplingCLIEnvironmentAndFailFast(t *testing.T) {
	restoreRunnerConfig(t)
	t.Setenv("E2E_RANDOM_NBC_COVERAGE", "true")
	t.Setenv("BUILD_ID", "build-id")
	var stderr bytes.Buffer
	app := NewApp(&bytes.Buffer{}, &stderr)
	require.Equal(t, exitUsage, app.Run(t.Context(), []string{
		"e2e", "run", "--disable-scriptless=false", "--test-pre-provision=false", "--tags", "", "--skip-tags", "",
		"--junit-file", filepath.Join(t.TempDir(), "report.xml"), "Ubuntu2404Gen1",
	}))
	require.Contains(t, stderr.String(), "NBC sampling requires 2 eligible")
	opts := runOptionsFromConfig(nil)
	require.True(t, opts.randomNBCCoverage)
	require.Equal(t, "build-id", config.Config.BuildID)
	require.Equal(t, exitUsage, app.Run(t.Context(), []string{"e2e", "run", "--random-nbc-coverage=false", "DoesNotExist"}))
	require.False(t, runOptionsFromConfig(nil).randomNBCCoverage)
}

func TestNBCSamplingSeedIncludesUTCDate(t *testing.T) {
	now := time.Date(2026, 11, 3, 23, 30, 0, 0, time.FixedZone("UTC-8", -8*60*60))
	require.Equal(t, "123/2026-11-04", nbcSamplingSeed("123", now))
	require.Equal(t, nbcSamplingSeed("123", now), nbcSamplingSeed("123", now.UTC()))
	require.NotEqual(t, nbcSamplingSeed("123", now), nbcSamplingSeed("123", now.AddDate(0, 0, 1)))
	require.NotEqual(t, nbcSamplingSeed("123", now), nbcSamplingSeed("456", now))
	require.False(t, config.DefaultConfiguration().RandomNBCCoverage)
}

func TestNBCSamplesHaveIndependentResultsAndLogs(t *testing.T) {
	restoreRunnerConfig(t)
	originals := nbcCandidates()
	sampled, err := sampleNBCScenarios(originals, "seed")
	require.NoError(t, err)
	exec := newExecutor(t.Context(), &bytes.Buffer{}, runOptions{
		parallel: 4, logDir: t.TempDir(), outputMode: "grouped",
	}, len(originals)+len(sampled))
	exec.runScenario = func(_ context.Context, name, artifactName string, s *scenario.Scenario) scenario.Outcome {
		if name != artifactName || strings.HasSuffix(name, "/ScriptlessNBCCSECmd") != s.ForceScriptlessNBCCSECmd {
			return scenario.Outcome{Error: fmt.Errorf("incorrect sample identity: %s / %s", name, artifactName)}
		}
		return scenario.Outcome{}
	}
	for _, s := range append(originals, sampled...) {
		exec.schedule(s.Name, s)
	}
	require.NoError(t, exec.wait(time.Second))
	results := exec.snapshotResults(nil)
	require.Len(t, results, 40)
	for _, result := range results {
		require.Equal(t, statusPassed, result.Status, result.Name)
		require.Equal(t, filepath.Join(exec.opts.logDir, result.Name, "attempt-1.log"), result.Attempts[0].LogPath)
		require.FileExists(t, result.Attempts[0].LogPath)
	}
}
