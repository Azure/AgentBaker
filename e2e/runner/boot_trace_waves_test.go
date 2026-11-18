package runner

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/scenario"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testBootTraceMatrix() []*scenario.Scenario {
	var scenarios []*scenario.Scenario
	for variant := range 7 {
		for wave := 1; wave <= 3; wave++ {
			scenarios = append(scenarios, &scenario.Scenario{
				Name: fmt.Sprintf("Variant%d_%d", variant, wave),
				Config: scenario.Config{
					VHD:                 &config.Image{Name: fmt.Sprintf("test-image-%d", variant/5)},
					BootTraceBackend:    "ftrace",
					BootTraceExperiment: &scenario.BootTraceExperiment{Variant: fmt.Sprint(variant % 5)},
					BootTraceWave:       wave,
				},
			})
		}
	}
	return scenarios
}

func TestBootTraceWavesRejectsIncompleteOrUnbalancedMatrices(t *testing.T) {
	tests := []struct {
		name   string
		change func([]*scenario.Scenario) []*scenario.Scenario
	}{
		{"missing scenario", func(s []*scenario.Scenario) []*scenario.Scenario { return s[:20] }},
		{"duplicate variant", func(s []*scenario.Scenario) []*scenario.Scenario { s[3] = s[0]; return s }},
		{"missing wave metadata", func(s []*scenario.Scenario) []*scenario.Scenario { s[0].BootTraceWave = 0; return s }},
		{"different variants between waves", func(s []*scenario.Scenario) []*scenario.Scenario {
			s[1].BootTraceExperiment.Variant = "unmatched"
			return s
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := bootTraceWaves(test.change(testBootTraceMatrix()))
			require.ErrorContains(t, err, "--boot-trace-waves")
		})
	}
}

func TestExecutorRunsBalancedWavesWithBarrierAndOneFinalReport(t *testing.T) {
	restoreRunnerConfig(t)
	waves, err := bootTraceWaves(testBootTraceMatrix())
	require.NoError(t, err)
	opts := runOptions{parallel: 7, logDir: t.TempDir(), outputMode: "grouped"}
	exec := newExecutor(t.Context(), &bytes.Buffer{}, opts, 21)
	started := make(chan int, 21)
	finished := make(chan int, 21)
	release := make([]chan struct{}, 21)
	for index := range release {
		release[index] = make(chan struct{})
		t.Cleanup(func() {
			select {
			case <-release[index]:
			default:
				close(release[index])
			}
		})
	}
	var completed [3]atomic.Int32
	var overlapping atomic.Bool
	exec.runScenario = func(_ context.Context, name, _ string, s *scenario.Scenario) scenario.Outcome {
		wave := s.BootTraceWave - 1
		if wave > 0 && completed[wave-1].Load() != 7 {
			overlapping.Store(true)
		}
		started <- wave
		var variant, repeat int
		_, scanErr := fmt.Sscanf(name, "Variant%d_%d", &variant, &repeat)
		if scanErr != nil {
			return scenario.Outcome{Error: scanErr}
		}
		<-release[wave*7+variant]
		completed[wave].Add(1)
		finished <- wave
		if name == "Variant0_1" {
			return scenario.Outcome{Error: errors.New("retained first-wave failure")}
		}
		return scenario.Outcome{}
	}
	done := make(chan error, 1)
	go func() { done <- exec.runWaves(waves) }()
	receive := func(channel <-chan int, expected int) {
		t.Helper()
		select {
		case actual := <-channel:
			require.Equal(t, expected, actual)
		case <-time.After(5 * time.Second):
			t.Fatal("wave did not make progress")
		}
	}
	for wave := range 3 {
		for range 7 {
			receive(started, wave)
		}
		for variant := range 6 {
			close(release[wave*7+variant])
			receive(finished, wave)
		}
		select {
		case next := <-started:
			t.Fatalf("wave %d started before the last scenario in wave %d finished", next, wave)
		default:
		}
		close(release[wave*7+6])
		receive(finished, wave)
	}
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("completed waves did not finish")
	}
	assert.False(t, overlapping.Load())
	results := exec.snapshotResults(nil)
	require.Len(t, results, 21)
	assert.Equal(t, 1, exec.summary().Failed)
	assert.Equal(t, 20, exec.summary().Passed)
	path := filepath.Join(t.TempDir(), "junit.xml")
	require.NoError(t, writeJUnitReport(path, results))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var report junitSuites
	require.NoError(t, xml.Unmarshal(data, &report))
	assert.Equal(t, 21, report.Tests)
	assert.Equal(t, 1, report.Failures)
	for _, result := range results {
		_, err := os.Stat(result.Attempts[0].LogPath)
		require.NoError(t, err, "an earlier wave lost its logs")
	}
}

func TestExecutorCancellationDoesNotStartLaterWaves(t *testing.T) {
	restoreRunnerConfig(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	exec := newExecutor(ctx, &bytes.Buffer{}, runOptions{
		parallel: 7, logDir: t.TempDir(), outputMode: "grouped",
	}, 21)
	waves, err := bootTraceWaves(testBootTraceMatrix())
	require.NoError(t, err)
	var later atomic.Bool
	exec.runScenario = func(_ context.Context, _, _ string, s *scenario.Scenario) scenario.Outcome {
		if s.BootTraceWave > 1 {
			later.Store(true)
		}
		cancel()
		return scenario.Outcome{}
	}
	require.ErrorIs(t, exec.runWaves(waves), context.Canceled)
	assert.False(t, later.Load())
	assert.LessOrEqual(t, len(exec.snapshotResults(nil)), 7)
}

func TestAppRejectsInvalidWaveOptionsBeforeAzureInitialization(t *testing.T) {
	t.Setenv("TAGS_TO_RUN", "Name=unrelated-pipeline-selection")
	t.Setenv("TAGS_TO_SKIP", "Name=Ubuntu2204_CustomLinuxOSConfig_Taints_ANC")
	for _, flags := range [][]string{
		{"--parallel", "3"},
		{"--parallel", "7", "--retries", "1"},
		{"--parallel", "7", "--random-nbc-coverage"},
		{"--parallel", "7"},
	} {
		t.Run(fmt.Sprint(flags), func(t *testing.T) {
			restoreRunnerConfig(t)
			var stderr bytes.Buffer
			app := NewApp(&bytes.Buffer{}, &stderr)
			args := append([]string{"e2e", "run", "--boot-trace-waves", "--tags", "", "--skip-tags", ""}, flags...)
			args = append(args, "Ubuntu2204_CustomLinuxOSConfig_Taints_ANC")
			assert.Equal(t, exitUsage, app.Run(t.Context(), args), stderr.String())
			assert.Contains(t, stderr.String(), "--boot-trace-waves")
		})
	}
}
