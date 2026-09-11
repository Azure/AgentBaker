package scenario

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/Azure/agentbaker/e2e/toolkit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type discardLogger struct{}

func (discardLogger) Log(...any)          {}
func (discardLogger) Logf(string, ...any) {}

func TestExecutionRunsCleanupOnceWithFreshState(t *testing.T) {
	original := &Scenario{Name: "Retry", Runtime: &ScenarioRuntime{}, failed: true}
	var cleanups []*scenarioCleanup
	var runs []*Scenario
	var cleanupCount int
	for attempt := 1; attempt <= 2; attempt++ {
		artifactName := filepath.Join("Retry", fmt.Sprintf("attempt-%d", attempt))
		outcome := runExecution(context.Background(), original.Name, artifactName, discardLogger{}, original,
			func(_ context.Context, _ string, _ toolkit.Logger, s *Scenario) error {
				assert.Nil(t, s.Runtime)
				assert.False(t, s.failed)
				assert.Empty(t, s.adoTestCases)
				assert.Equal(t, artifactName, s.artifactName)
				cleanups = append(cleanups, s.cleanup)
				runs = append(runs, s)
				s.Runtime = &ScenarioRuntime{}
				s.recordADOTestCase("Task_example", "e2e.cse", time.Second, nil)
				s.Cleanup(func(ctx context.Context) error {
					assert.NoError(t, ctx.Err())
					cleanupCount++
					return nil
				})
				if attempt == 1 {
					return errors.New("transient failure")
				}
				return nil
			})
		require.Len(t, outcome.Measurements, 1)
		if attempt == 1 {
			require.ErrorContains(t, outcome.Error, "transient failure")
		} else {
			require.NoError(t, outcome.Error)
		}
		outcome.Measurements[0].Name = "changed"
		assert.Equal(t, "Task_example", runs[attempt-1].adoTestCases[0].Name)
	}
	assert.Equal(t, 2, cleanupCount)
	assert.NotSame(t, cleanups[0], cleanups[1])
	assert.NotSame(t, runs[0].Runtime, runs[1].Runtime)
	assert.Nil(t, original.cleanup)
	assert.Empty(t, original.adoTestCases)
	assert.True(t, original.failed)
}

func TestExecutionCleanupFailureOverridesOutcome(t *testing.T) {
	for _, test := range []struct {
		name   string
		runErr error
	}{
		{name: "passed"},
		{name: "skipped", runErr: &skipError{message: "not supported"}},
		{name: "failed", runErr: errors.New("validation failed")},
	} {
		t.Run(test.name, func(t *testing.T) {
			outcome := runExecution(context.Background(), "CleanupFails", "CleanupFails", discardLogger{}, &Scenario{},
				func(_ context.Context, _ string, _ toolkit.Logger, s *Scenario) error {
					s.Cleanup(func(context.Context) error { return errors.New("delete vmss") })
					return test.runErr
				})
			require.ErrorContains(t, outcome.Error, "delete vmss")
			assert.Empty(t, outcome.SkipReason)
			if test.runErr != nil {
				assert.ErrorContains(t, outcome.Error, test.runErr.Error())
			}
		})
	}
}

func TestExecutionSharesCleanupAcrossVHDStages(t *testing.T) {
	var cleaned []string
	outcome := runExecution(context.Background(), "VHD", "VHD/attempt-2", discardLogger{}, &Scenario{},
		func(_ context.Context, _ string, _ toolkit.Logger, original *Scenario) error {
			for _, name := range []string{"vhd-bake", "vhd-provision"} {
				stage := freshScenario(original)
				assert.Same(t, original.cleanup, stage.cleanup)
				stage.Cleanup(func(context.Context) error {
					cleaned = append(cleaned, name)
					return nil
				})
			}
			assert.Empty(t, cleaned)
			return nil
		})
	require.NoError(t, outcome.Error)
	assert.Equal(t, []string{"vhd-provision", "vhd-bake"}, cleaned)
}

func TestExecutionPanicMarksFailureAndRunsCleanup(t *testing.T) {
	for _, markedByFlow := range []bool{false, true} {
		var cleanupCount int
		outcome := runExecution(context.Background(), "Panics", "Panics", discardLogger{}, &Scenario{},
			func(_ context.Context, _ string, _ toolkit.Logger, s *Scenario) (runErr error) {
				if markedByFlow {
					defer func() { markScenarioOutcome(s, runErr, recover()) }()
				}
				s.Cleanup(func(context.Context) error {
					assert.True(t, s.failed)
					cleanupCount++
					return nil
				})
				panic("boom")
			})
		assert.Equal(t, 1, cleanupCount)
		require.ErrorContains(t, outcome.Error, "panic: boom")
		assert.Empty(t, outcome.SkipReason)
	}
}

func TestSkippedScenarioIsNotMarkedFailed(t *testing.T) {
	var cleaned bool
	outcome := runExecution(context.Background(), "Skipped", "Skipped", discardLogger{}, &Scenario{},
		func(_ context.Context, _ string, _ toolkit.Logger, s *Scenario) (runErr error) {
			defer func() { markScenarioOutcome(s, runErr, recover()) }()
			s.Cleanup(func(context.Context) error {
				assert.False(t, s.failed)
				cleaned = true
				return nil
			})
			return &skipError{message: "SKU not available"}
		})
	require.NoError(t, outcome.Error)
	assert.Equal(t, "SKU not available", outcome.SkipReason)
	assert.True(t, cleaned)
}

func TestExecutionTimeoutCoversSkipAndRun(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var skipDeadline, runDeadline time.Time
	outcome := runExecution(ctx, "Deadline", "Deadline", discardLogger{}, &Scenario{
		SkipIf: func(ctx context.Context) string {
			skipDeadline, _ = ctx.Deadline()
			return ""
		},
	}, func(ctx context.Context, _ string, _ toolkit.Logger, _ *Scenario) error {
		runDeadline, _ = ctx.Deadline()
		return nil
	})
	require.NoError(t, outcome.Error)
	assert.False(t, skipDeadline.IsZero())
	assert.Equal(t, runDeadline, skipDeadline)
}

func TestExecutionTimeoutCannotPassOrSkip(t *testing.T) {
	for _, test := range []struct {
		name   string
		skipIf func(context.Context) string
		run    func(context.Context, string, toolkit.Logger, *Scenario) error
	}{
		{
			name: "late success",
			run: func(ctx context.Context, _ string, _ toolkit.Logger, s *Scenario) error {
				s.Cleanup(func(cleanupCtx context.Context) error {
					assert.NoError(t, cleanupCtx.Err())
					assert.True(t, s.failed)
					deadline, ok := cleanupCtx.Deadline()
					assert.True(t, ok)
					assert.Positive(t, time.Until(deadline))
					assert.LessOrEqual(t, time.Until(deadline), CleanupTimeout)
					return nil
				})
				<-ctx.Done()
				return nil
			},
		},
		{
			name: "late skip",
			skipIf: func(ctx context.Context) string {
				<-ctx.Done()
				return "not configured"
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			defer cancel()
			outcome := runExecution(ctx, "Deadline", "Deadline", discardLogger{}, &Scenario{SkipIf: test.skipIf}, test.run)
			require.ErrorContains(t, outcome.Error, "scenario attempt deadline exceeded")
			require.ErrorIs(t, outcome.Error, context.DeadlineExceeded)
			assert.Empty(t, outcome.SkipReason)
		})
	}
}

func TestExecutionSkipDecisionsDoNotRunFlow(t *testing.T) {
	for _, s := range []*Scenario{
		{SkipReason: "not supported"},
		{SkipIf: func(context.Context) string { return "not supported" }},
	} {
		outcome := runExecution(context.Background(), "Skip", "Skip", discardLogger{}, s,
			func(context.Context, string, toolkit.Logger, *Scenario) error {
				t.Fatal("flow ran for skipped scenario")
				return nil
			})
		require.NoError(t, outcome.Error)
		assert.Equal(t, "not supported", outcome.SkipReason)
	}
}

func TestFreshScenarioSharesAttemptCleanup(t *testing.T) {
	cleanup := &scenarioCleanup{}
	original := &Scenario{
		Name:         "Staged",
		cleanup:      cleanup,
		Runtime:      &ScenarioRuntime{},
		artifactName: "Staged",
		failed:       true,
		adoTestCases: []Measurement{{Name: "Task_example"}},
	}

	copied := freshScenario(original)

	assert.Same(t, cleanup, copied.cleanup)
	assert.Nil(t, copied.Runtime)
	assert.Nil(t, copied.Logger)
	assert.Equal(t, original.artifactName, copied.artifactName)
	assert.False(t, copied.failed)
	assert.Nil(t, copied.adoTestCases)
}

func TestVHDStageArtifactNamePreservesAttempt(t *testing.T) {
	scenario := &Scenario{artifactName: filepath.Join("Retry", "attempt-2")}

	assert.Equal(t, filepath.Join("Retry", "attempt-2", "vhd-bake"), vhdStageArtifactName(scenario, "Retry", "vhd-bake"))
	assert.Equal(t, filepath.Join("Retry", "attempt-2", "vhd-provision"), vhdStageArtifactName(scenario, "Retry", "vhd-provision"))
}

func TestAnnotateVMSSCreateErrorPreservesSkip(t *testing.T) {
	skip := &skipError{message: "SKU not available"}
	scenario := &Scenario{
		Runtime:      &ScenarioRuntime{VMSSName: "vmss"},
		artifactName: "Scenario",
	}

	assert.Same(t, skip, annotateVMSSCreateError(scenario, skip))
	assert.ErrorContains(t, annotateVMSSCreateError(scenario, errors.New("create failed")), "check "+artifactDir(scenario.artifactName)+" for vm logs")
}
