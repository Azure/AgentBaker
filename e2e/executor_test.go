package e2e

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/toolkit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppListsRegisteredScenarios(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := NewApp(&stdout, &stderr)

	assert.Equal(t, exitSuccess, app.Run(context.Background(), []string{"e2e", "list"}), stderr.String())
	assert.Contains(t, stdout.String(), "Ubuntu2204\n", "list did not contain a known scenario")
}

func TestAppRejectsUnknownScenario(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := NewApp(&stdout, &stderr)

	assert.Equal(t, exitUsage, app.Run(context.Background(), []string{"e2e", "run", "--log-dir", t.TempDir(), "DoesNotExist"}), "stderr: %s", stderr.String())
}

func TestAppReturnsUsageExitCodeForInvalidFlag(t *testing.T) {
	restoreRunnerConfig(t)
	var stderr bytes.Buffer
	app := NewApp(&bytes.Buffer{}, &stderr)

	assert.Equal(t, exitUsage, app.Run(context.Background(), []string{"e2e", "run", "--parallel", "nope"}), "stderr: %s", stderr.String())
}

func TestAppReturnsUsageExitCodeBeforeRunStarts(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		parallelEnv string
		message     string
	}{
		{name: "root flag", args: []string{"e2e", "--badflag"}, message: "flag provided but not defined"},
		{name: "list flag", args: []string{"e2e", "list", "--badflag"}, message: "flag provided but not defined"},
		{name: "environment value", args: []string{"e2e", "run", "DoesNotExist"}, parallelEnv: "nope", message: "E2E_PARALLEL"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			restoreRunnerConfig(t)
			if test.parallelEnv != "" {
				t.Setenv("E2E_PARALLEL", test.parallelEnv)
			}
			var stderr bytes.Buffer
			app := NewApp(&bytes.Buffer{}, &stderr)
			assert.Equal(t, exitUsage, app.Run(context.Background(), test.args), "stderr: %s", stderr.String())
			assert.Contains(t, stderr.String(), test.message)
		})
	}
}

func TestAppRejectsNonPositiveDurations(t *testing.T) {
	for _, test := range []struct {
		flag    string
		message string
	}{
		{flag: "--timeout=0s", message: "--timeout must be greater than zero"},
		{flag: "--suite-timeout=0s", message: "--suite-timeout must be greater than zero"},
		{flag: "--cluster-timeout=0s", message: "--cluster-timeout must be greater than zero"},
		{flag: "--vmss-timeout=0s", message: "--vmss-timeout must be greater than zero"},
		{flag: "--poll-interval=0s", message: "--poll-interval must be greater than zero"},
	} {
		t.Run(test.flag, func(t *testing.T) {
			restoreRunnerConfig(t)
			var stderr bytes.Buffer
			app := NewApp(&bytes.Buffer{}, &stderr)
			assert.Equal(t, exitUsage, app.Run(context.Background(), []string{"e2e", "run", test.flag, "DoesNotExist"}), "stderr: %s", stderr.String())
			assert.Contains(t, stderr.String(), test.message)
		})
	}
}

func TestRunnerFlagsUseEnvironmentAliases(t *testing.T) {
	restoreRunnerConfig(t)
	t.Setenv("PARALLEL", "7")
	t.Setenv("TEST_TIMEOUT", "3m")
	t.Setenv("E2E_GO_TEST_TIMEOUT", "4m")
	t.Setenv("TIMEOUT", "4m")
	t.Setenv("E2E_FAILED_TESTS_RETRY_COUNT", "2")
	t.Setenv("LOGGING_DIR", t.TempDir())
	t.Setenv("E2E_OUTPUT", "grouped")
	t.Setenv("TAGS_TO_RUN", "gpu=true")
	t.Setenv("TAGS_TO_SKIP", "os=windows")
	t.Setenv("GALLERY_NAME", "test-gallery")
	t.Setenv("KEEP_VMSS", "true")
	t.Setenv("SUBSCRIPTION_ID", "test-subscription")

	app := NewApp(&bytes.Buffer{}, &bytes.Buffer{})
	assert.Equal(t, exitUsage, app.Run(context.Background(), []string{"e2e", "run", "DoesNotExist"}))
	assert.Equal(t, 7, config.Config.Parallel)
	assert.Equal(t, 3*time.Minute, config.Config.TestTimeout)
	assert.Equal(t, 2, config.Config.Retries)
	opts := runOptionsFromConfig(nil)
	assert.Equal(t, 4*time.Minute, config.Config.SuiteTimeout)
	assert.Equal(t, "grouped", config.Config.OutputMode)
	assert.Equal(t, tagFilter{run: "gpu=true", skip: "os=windows"}, opts.tagFilter)
	assert.Equal(t, "test-gallery", config.Config.GalleryLinux.Name)
	assert.Equal(t, "test-gallery", config.Config.GalleryWindows.Name)
	assert.True(t, config.Config.KeepVMSS)
	assert.Equal(t, "test-subscription", config.Config.SubscriptionID)
}

func TestAppRejectsUnknownScenarioChild(t *testing.T) {
	restoreRunnerConfig(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := NewApp(&stdout, &stderr)

	assert.Equal(t, exitUsage, app.Run(context.Background(), []string{"e2e", "run", "--log-dir", t.TempDir(), "Ubuntu2204/not-a-scenario"}), "stderr: %s", stderr.String())
}

func TestAppSuggestsMistypedFlag(t *testing.T) {
	restoreRunnerConfig(t)

	var stderr bytes.Buffer
	app := NewApp(&bytes.Buffer{}, &stderr)
	assert.Equal(t, exitUsage, app.Run(context.Background(), []string{"e2e", "run", "--paralel", "1"}))
	assert.Contains(t, stderr.String(), "--parallel", "missing flag suggestion")
}

func TestSelectScenariosUsesParentNameAsGroup(t *testing.T) {
	scenarios := []*Scenario{
		{Name: "Group/one"},
		{Name: "Group/two"},
		{Name: "Other"},
	}
	assert.Len(t, selectScenarios(scenarios, []string{"Group"}), 2)
	assert.Len(t, selectScenarios(scenarios, []string{"Group/one"}), 1)
	assert.Empty(t, selectScenarios(scenarios, []string{"Group/missing"}))
}

func TestResetLogDirectoryRemovesStaleArtifacts(t *testing.T) {
	logDir := filepath.Join(t.TempDir(), "scenario-logs")
	require.NoError(t, os.MkdirAll(logDir, 0o755))
	stale := filepath.Join(logDir, "stale.log")
	require.NoError(t, os.WriteFile(stale, []byte("stale"), 0o600))

	require.NoError(t, resetLogDirectory(logDir))

	_, err := os.Stat(stale)
	assert.True(t, os.IsNotExist(err))
	info, err := os.Stat(logDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestResetLogDirectoryRejectsWorkingDirectoryAndParent(t *testing.T) {
	workingDirectory, err := os.Getwd()
	require.NoError(t, err)

	require.Error(t, resetLogDirectory(workingDirectory))
	require.Error(t, resetLogDirectory(filepath.Dir(workingDirectory)))
}

func TestExecutorRetriesAndReportsFlakyScenario(t *testing.T) {
	tmp := t.TempDir()
	var stdout bytes.Buffer
	opts := runOptions{
		parallel:   1,
		retries:    1,
		logDir:     filepath.Join(tmp, "logs"),
		junitFile:  filepath.Join(tmp, "report.xml"),
		outputMode: "grouped",
	}
	exec := newExecutor(context.Background(), &stdout, opts, 1)

	var calls atomic.Int32
	var artifactNames []string
	exec.runScenario = func(_ context.Context, _ string, logger toolkit.Logger, s *Scenario) error {
		logger.Log("attempt output")
		artifactNames = append(artifactNames, s.artifactName)
		if calls.Add(1) == 1 {
			err := errors.New("transient failure")
			s.recordADOTestCase("Task_example", "e2e.cse", time.Second, err)
			return err
		}
		s.recordADOTestCase("Task_example", "e2e.cse", time.Second, nil)
		return nil
	}

	exec.schedule("Retry", &Scenario{})
	exec.scenarios.Wait()
	require.NoError(t, writeJUnitReport(opts.junitFile, exec.snapshotResults(nil)))

	summary := exec.summary()
	assert.Equal(t, 1, summary.Flaky)
	assert.Zero(t, summary.Failed)
	report, err := os.ReadFile(opts.junitFile)
	require.NoError(t, err)
	assert.Contains(t, string(report), `value="flaky"`)
	assert.Contains(t, string(report), "[[ATTACHMENT|", "JUnit report does not describe flaky attempts")
	assert.Contains(t, string(report), `name="Retry/Task_example"`, "JUnit report does not contain the CSE child result")
	assert.Regexp(t, `##\[group\]🔴 Retry \(attempt 1/2, failed, [^)]+\)`, stdout.String())
	assert.Regexp(t, `##\[group\]Retry \(attempt 2/2, passed, [^)]+\)`, stdout.String())
	assert.Equal(t, []string{filepath.Join("Retry", "attempt-1"), filepath.Join("Retry", "attempt-2")}, artifactNames)
}

func TestExecutorPrintsPassedLogs(t *testing.T) {
	var stdout bytes.Buffer
	exec := newExecutor(context.Background(), &stdout, runOptions{
		parallel:   1,
		logDir:     t.TempDir(),
		outputMode: "grouped",
	}, 1)
	exec.runScenario = func(_ context.Context, _ string, logger toolkit.Logger, _ *Scenario) error {
		logger.Log("passing output")
		return nil
	}

	exec.schedule("Passed", &Scenario{Name: "Passed"})
	exec.scenarios.Wait()
	assert.Regexp(t, `##\[group\]Passed \(passed, [^)]+\)`, stdout.String())
	assert.Contains(t, stdout.String(), "passing output")
	assert.Contains(t, stdout.String(), "##[endgroup]")
}

func TestAttemptConsoleLabel(t *testing.T) {
	assert.Equal(t, "passed, 6m12s", attemptConsoleLabel(statusPassed, 1, 1, 6*time.Minute+12*time.Second+400*time.Millisecond))
	assert.Equal(t, "attempt 2/3, failed, 6m13s", attemptConsoleLabel(statusFailed, 2, 3, 6*time.Minute+12*time.Second+600*time.Millisecond))
}

func TestScenarioLogIncludesElapsedTime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "attempt.log")
	exec := newExecutor(context.Background(), &bytes.Buffer{}, runOptions{
		parallel:   1,
		logDir:     t.TempDir(),
		outputMode: "grouped",
	}, 1)
	logger, err := newScenarioLogger(exec, "Example", path)
	require.NoError(t, err)
	logger.Log("hello\nworld")
	require.NoError(t, logger.Close())
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Regexp(t, `^\[\d+\.\d{3}s\] hello\n\[\d+\.\d{3}s\] world\n$`, string(content))
}

func TestExecutorRecoversScenarioPanic(t *testing.T) {
	var stdout bytes.Buffer
	exec := newExecutor(context.Background(), &stdout, runOptions{
		parallel:   2,
		logDir:     t.TempDir(),
		outputMode: "grouped",
	}, 2)
	exec.runScenario = func(_ context.Context, name string, _ toolkit.Logger, _ *Scenario) error {
		if name == "Panics" {
			panic("boom")
		}
		return nil
	}

	exec.schedule("Panics", &Scenario{Name: "Panics"})
	exec.schedule("Passes", &Scenario{Name: "Passes"})
	exec.scenarios.Wait()
	summary := exec.summary()
	assert.Equal(t, 1, summary.Failed)
	assert.Equal(t, 1, summary.Passed)
	assert.Contains(t, stdout.String(), "🔴 FAIL: panic: boom", "panic was not reported")
}

func TestCleanupFailureOverridesSkip(t *testing.T) {
	err := errors.Join(&skipError{message: "skip"}, errors.New("cleanup failed"))
	status, _ := classifyAttempt(err)
	assert.Equal(t, statusFailed, status)
	assert.ErrorContains(t, err, "skip")
	assert.ErrorContains(t, err, "cleanup failed")
}

func newTestExecutor(t *testing.T) *executor {
	t.Helper()
	return newExecutor(context.Background(), &bytes.Buffer{}, runOptions{
		parallel:   1,
		logDir:     t.TempDir(),
		outputMode: "grouped",
	}, 1)
}

func TestExecutorRunsCleanupOncePerAttempt(t *testing.T) {
	exec := newTestExecutor(t)
	exec.opts.retries = 1

	var ran atomic.Int32
	var cleanups []*scenarioCleanup
	exec.runScenario = func(_ context.Context, _ string, _ toolkit.Logger, s *Scenario) error {
		cleanups = append(cleanups, s.cleanup)
		s.Cleanup(func(context.Context) error {
			ran.Add(1)
			return nil
		})
		if len(cleanups) == 1 {
			return errors.New("transient failure")
		}
		return nil
	}

	exec.schedule("Cleanups", &Scenario{Name: "Cleanups"})
	exec.scenarios.Wait()

	assert.Equal(t, int32(2), ran.Load(), "cleanups should run once per attempt")
	require.Len(t, cleanups, 2)
	assert.NotSame(t, cleanups[0], cleanups[1], "attempts did not get isolated cleanups")
	assert.Equal(t, 1, exec.summary().Flaky)
}

func TestExecutorCleanupFailureOverridesAttemptStatus(t *testing.T) {
	for _, test := range []struct {
		name   string
		runErr error
	}{
		{name: "passed", runErr: nil},
		{name: "skipped", runErr: &skipError{message: "not supported"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			exec := newTestExecutor(t)
			exec.runScenario = func(_ context.Context, _ string, _ toolkit.Logger, s *Scenario) error {
				s.Cleanup(func(context.Context) error {
					return errors.New("delete vmss")
				})
				return test.runErr
			}

			exec.schedule("CleanupFails", &Scenario{Name: "CleanupFails"})
			exec.scenarios.Wait()

			attempt := exec.results[0].Attempts[0]
			assert.Equal(t, statusFailed, attempt.Status)
			assert.Contains(t, attempt.Message, "delete vmss", "attempt message lost the cleanup failure")
			if test.runErr != nil {
				assert.Contains(t, attempt.Message, "not supported", "attempt message lost the original result")
			}
		})
	}
}

func TestExecutorRunsCleanupAfterScenarioPanic(t *testing.T) {
	exec := newTestExecutor(t)

	var ran atomic.Int32
	exec.runScenario = func(_ context.Context, _ string, _ toolkit.Logger, s *Scenario) error {
		s.Cleanup(func(context.Context) error {
			ran.Add(1)
			return nil
		})
		panic("boom")
	}

	exec.schedule("Panics", &Scenario{Name: "Panics"})
	exec.scenarios.Wait()

	assert.Equal(t, int32(1), ran.Load(), "cleanup should run after a panic")
	attempt := exec.results[0].Attempts[0]
	assert.Equal(t, statusFailed, attempt.Status)
	assert.Contains(t, attempt.Message, "panic: boom")
}

func TestFreshScenarioSharesAttemptCleanup(t *testing.T) {
	cleanup := &scenarioCleanup{}
	original := &Scenario{
		Name:         "Staged",
		cleanup:      cleanup,
		Runtime:      &ScenarioRuntime{},
		artifactName: "Staged",
		failed:       true,
		adoTestCases: []adoTestCase{{Name: "Task_example"}},
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

func TestExecutorReportsCancellationAsFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	exec := newExecutor(ctx, &bytes.Buffer{}, runOptions{
		parallel:   1,
		logDir:     t.TempDir(),
		outputMode: "grouped",
	}, 1)

	exec.schedule("Cancelled", &Scenario{Name: "Cancelled"})
	exec.scenarios.Wait()
	summary := exec.summary()
	assert.Equal(t, 1, summary.Failed)
	assert.Zero(t, summary.Skipped)
	assert.Len(t, exec.results[0].Attempts, 1)
}

func TestExecutorWaitReturnsGracefulCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	exec := newExecutor(ctx, &bytes.Buffer{}, runOptions{
		parallel:   1,
		logDir:     t.TempDir(),
		outputMode: "grouped",
	}, 1)
	started := make(chan struct{})
	exec.runScenario = func(ctx context.Context, _ string, _ toolkit.Logger, _ *Scenario) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}

	exec.schedule("Cancelled", &Scenario{Name: "Cancelled"})
	<-started
	cancel()

	require.ErrorIs(t, exec.wait(time.Second), context.Canceled)
}

func TestExecutorWaitStopsAfterGracePeriod(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	tmp := t.TempDir()
	exec := newExecutor(ctx, &bytes.Buffer{}, runOptions{
		parallel:   1,
		logDir:     filepath.Join(tmp, "logs"),
		junitFile:  filepath.Join(tmp, "report.xml"),
		outputMode: "grouped",
	}, 1)
	started := make(chan struct{})
	release := make(chan struct{})
	exec.runScenario = func(context.Context, string, toolkit.Logger, *Scenario) error {
		close(started)
		<-release
		return nil
	}

	exec.schedule("Stuck", &Scenario{Name: "Stuck"})
	<-started
	cancel()

	require.ErrorContains(t, exec.wait(10*time.Millisecond), "did not stop")
	summary := exec.summary()
	assert.Equal(t, 1, summary.Total)
	assert.Equal(t, 1, summary.Failed)
	require.NoError(t, writeJUnitReport(exec.opts.junitFile, exec.snapshotResults(nil)))
	report, err := os.ReadFile(exec.opts.junitFile)
	require.NoError(t, err)
	assert.Contains(t, string(report), `failure message="scenarios did not stop`, "JUnit report dropped the unfinished scenario")
	assert.NotContains(t, string(report), "attached scenario log", "JUnit report claimed a missing attachment")
	close(release)
	exec.scenarios.Wait()
	assert.Len(t, exec.results, 1, "late scenario result created a duplicate")
}

func TestJUnitFailureUsesReadableSummary(t *testing.T) {
	message := "create vmss: GET https://management.azure.com/example\n" +
		"--------------------------------------------------------------------------------\n" +
		"RESPONSE 200: 200 OK\n" +
		"ERROR CODE: VMExtensionProvisioningError\n" +
		strings.Repeat(`{\"Output\":\"escaped payload\"}`, 500) +
		` stderr="acr-mirror.service does not exist"`
	failure := junitFailureSummary(message, true)

	assert.Contains(t, failure.Message, "create vmss: GET")
	assert.Contains(t, failure.Message, "RESPONSE 200")
	assert.Contains(t, failure.Message, "ERROR CODE: VMExtensionProvisioningError")
	assert.NotContains(t, failure.Message, "--------------------------------------------------------------------------------", "failure summary included separator noise")
	assert.LessOrEqual(t, len([]rune(failure.Message)), 500, "failure summary was not bounded")
	assert.Contains(t, failure.Message, " ... ", "failure summary was not bounded")
	assert.Contains(t, failure.Message, `stderr="acr-mirror.service does not exist"`, "failure summary omitted diagnostic tail")
	assert.Contains(t, failure.Body, "attached scenario log", "failure body omitted attachment guidance")
}

func TestJUnitFailureSummaryDoesNotClaimChildAttachment(t *testing.T) {
	failure := junitFailureSummary("CSE timing failed", false)
	assert.NotContains(t, failure.Body, "attached", "child failure claimed a parent attachment")
}

func TestScenarioSkipReasonRecordsSkip(t *testing.T) {
	tmp := t.TempDir()
	var stdout bytes.Buffer
	opts := runOptions{
		parallel:   1,
		logDir:     filepath.Join(tmp, "logs"),
		junitFile:  filepath.Join(tmp, "report.xml"),
		outputMode: "grouped",
	}
	exec := newExecutor(context.Background(), &stdout, opts, 1)

	exec.schedule("Disabled", &Scenario{
		Name:       "Disabled",
		SkipReason: "not supported",
	})
	exec.scenarios.Wait()
	require.NoError(t, writeJUnitReport(opts.junitFile, exec.snapshotResults(nil)))
	exec.printSummary(exec.snapshotResults(nil))

	summary := exec.summary()
	assert.Equal(t, 1, summary.Total)
	assert.Equal(t, 1, summary.Skipped)
	assert.Contains(t, stdout.String(), "Skipped:\n- Disabled: not supported", "ordinary skip was not listed in the summary")
	assert.NotContains(t, stdout.String(), "##[group]", "ordinary skip created a noisy console group")
	report, err := os.ReadFile(opts.junitFile)
	require.NoError(t, err)
	assert.Contains(t, string(report), `<skipped message="not supported"`, "JUnit report dropped the ordinary skip")
}

func TestScenarioTimeoutCoversSkipAndRun(t *testing.T) {
	restoreRunnerConfig(t)
	config.Config.TestTimeout = time.Second

	exec := newTestExecutor(t)
	var skipDeadline, runDeadline time.Time
	exec.runScenario = func(ctx context.Context, _ string, _ toolkit.Logger, _ *Scenario) error {
		runDeadline, _ = ctx.Deadline()
		return nil
	}
	exec.schedule("Deadline", &Scenario{
		Name: "Deadline",
		SkipIf: func(ctx context.Context) string {
			skipDeadline, _ = ctx.Deadline()
			return ""
		},
	})
	exec.scenarios.Wait()

	assert.False(t, skipDeadline.IsZero())
	assert.Equal(t, runDeadline, skipDeadline, "SkipIf and scenario received different attempt deadlines")
}

func TestScenarioTimeoutCannotPassOrSkip(t *testing.T) {
	restoreRunnerConfig(t)
	config.Config.TestTimeout = 10 * time.Millisecond

	for _, test := range []struct {
		name   string
		skipIf func(context.Context) string
		run    func(context.Context) error
	}{
		{
			name: "late success",
			run: func(ctx context.Context) error {
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
			exec := newTestExecutor(t)
			exec.runScenario = func(ctx context.Context, _ string, _ toolkit.Logger, _ *Scenario) error {
				return test.run(ctx)
			}
			exec.schedule("Deadline", &Scenario{Name: "Deadline", SkipIf: test.skipIf})
			exec.scenarios.Wait()

			result := exec.results[0]
			assert.Equal(t, statusFailed, result.Status)
			assert.Contains(t, result.Attempts[0].Message, "scenario attempt deadline exceeded")
		})
	}
}

func TestSummaryOrdersImportantResultsLast(t *testing.T) {
	var stdout bytes.Buffer
	exec := &executor{
		stdout: &stdout,
		results: []scenarioResult{
			{Name: "Passed", Status: statusPassed, Attempts: []attemptResult{{Attempt: 1, Status: statusPassed}}},
			{Name: "Skipped", Status: statusSkipped, Attempts: []attemptResult{{Attempt: 1, Status: statusSkipped, Message: "not supported"}}},
			{Name: "Flaky", Status: statusFlaky, Attempts: []attemptResult{
				{Attempt: 1, Status: statusFailed, Message: "transient failure"},
				{Attempt: 2, Status: statusPassed},
			}},
			{Name: "Failed", Status: statusFailed, Attempts: []attemptResult{{Attempt: 1, Status: statusFailed, Message: "final failure\ndetails"}}},
		},
	}

	summary := exec.printSummary(exec.snapshotResults(nil))
	assert.Equal(t, runSummary{Total: 4, Passed: 1, Failed: 1, Skipped: 1, Flaky: 1}, summary)
	output := stdout.String()
	skippedIndex := strings.Index(output, "\nSkipped:")
	flakyIndex := strings.Index(output, "\nFlaky:")
	failedIndex := strings.Index(output, "\nFailed:")
	assert.NotEqual(t, -1, skippedIndex, "summary omitted skipped results")
	assert.Greater(t, flakyIndex, skippedIndex, "summary order is not skipped, flaky, failed")
	assert.Greater(t, failedIndex, flakyIndex, "summary order is not skipped, flaky, failed")
	assert.NotContains(t, output, "- Passed", "passing scenario name was listed")
	assert.Contains(t, output, "- Flaky (passed on attempt 2): transient failure", "flaky scenario details were omitted")
	assert.Contains(t, output, "- Failed: final failure; details", "failed scenario summary was not concise")
}

func TestFilteredScenariosAreNotScheduled(t *testing.T) {
	tmp := t.TempDir()
	var stdout bytes.Buffer
	opts := runOptions{
		parallel:   1,
		logDir:     filepath.Join(tmp, "logs"),
		junitFile:  filepath.Join(tmp, "report.xml"),
		outputMode: "grouped",
		tagFilter:  tagFilter{skip: "Name=Excluded"},
	}
	scenarios := []*Scenario{
		{
			Name:       "Excluded",
			SkipReason: "must not be consulted",
		},
		{Name: "Kept"},
	}

	runnable, filtered, err := partitionScenarios(scenarios, opts.tagFilter)
	require.NoError(t, err)
	require.Len(t, runnable, 1)
	assert.Equal(t, "Kept", runnable[0].Name)
	require.Len(t, filtered, 1)

	exec := newExecutor(context.Background(), &stdout, opts, len(runnable))
	exec.runScenario = func(context.Context, string, toolkit.Logger, *Scenario) error { return nil }
	for _, scenario := range runnable {
		exec.schedule(scenario.Name, scenario)
	}
	exec.scenarios.Wait()
	require.NoError(t, writeJUnitReport(opts.junitFile, exec.snapshotResults(filtered)))

	summary := exec.printSummary(exec.snapshotResults(filtered))
	assert.Equal(t, 2, summary.Total)
	assert.Equal(t, 1, summary.Passed)
	assert.Equal(t, 1, summary.Skipped)
	_, err = os.Stat(filepath.Join(opts.logDir, "Excluded"))
	assert.True(t, os.IsNotExist(err), "filtered scenario created an attempt log directory: %v", err)
	assert.Contains(t, stdout.String(), "DONE 2 scenarios: 1 passed, 0 flaky, 1 skipped, 0 failed")
	assert.Contains(t, stdout.String(), "- Excluded: filtered:", "filtered scenario was not listed as skipped")

	report, err := os.ReadFile(opts.junitFile)
	require.NoError(t, err)
	assert.Contains(t, string(report), `name="Excluded"`)
	assert.Contains(t, string(report), "<skipped message=\"filtered: scenario &#34;Excluded&#34;", "JUnit report dropped the filtered scenario")
	assert.Contains(t, string(report), `name="Kept"`, "JUnit report dropped the runnable scenario")
}

func TestAutoOutputUsesRunnableCount(t *testing.T) {
	scenarios := []*Scenario{
		{Name: "Only"},
		{Name: "Second"},
		{Name: "Third"},
		{Name: "Fourth"},
	}
	opts := runOptions{parallel: 1, logDir: t.TempDir(), outputMode: "auto"}

	runnable, filtered, err := partitionScenarios(scenarios, tagFilter{})
	require.NoError(t, err)
	assert.Len(t, runnable, 4)
	assert.Empty(t, filtered)
	assert.False(t, newExecutor(context.Background(), &bytes.Buffer{}, opts, len(runnable)).stream, "auto mode streamed more than three scenarios")

	runnable, filtered, err = partitionScenarios(scenarios, tagFilter{run: "Name=Only"})
	require.NoError(t, err)
	assert.Len(t, runnable, 1)
	assert.Len(t, filtered, 3)
	assert.True(t, newExecutor(context.Background(), &bytes.Buffer{}, opts, len(runnable)).stream, "auto mode did not stream a single runnable scenario")
}

func TestExecutorDoesNotReadGlobalTagFilters(t *testing.T) {
	restoreRunnerConfig(t)
	config.Config.TagsToRun = "gpu=true"
	config.Config.TagsToSkip = "Name=Runs"

	exec := newTestExecutor(t)
	exec.runScenario = func(context.Context, string, toolkit.Logger, *Scenario) error {
		return nil
	}
	exec.schedule("Runs", &Scenario{Name: "Runs"})
	exec.scenarios.Wait()

	assert.Equal(t, statusPassed, exec.results[0].Status, "global filters changed executor result")
}

func TestExecutorFailsAttemptOnLogWriteFailure(t *testing.T) {
	exec := newTestExecutor(t)
	exec.runScenario = func(_ context.Context, _ string, logger toolkit.Logger, _ *Scenario) error {
		scenarioLog := logger.(*scenarioLogger)
		scenarioLog.mu.Lock()
		_ = scenarioLog.file.Close()
		scenarioLog.mu.Unlock()
		logger.Log("output that cannot be persisted")
		return nil
	}

	exec.schedule("LogWriteFails", &Scenario{Name: "LogWriteFails"})
	exec.scenarios.Wait()

	attempt := exec.results[0].Attempts[0]
	assert.Equal(t, statusFailed, attempt.Status)
	assert.Equal(t, statusFailed, exec.results[0].Status, "log failure did not fail the attempt")
	assert.Contains(t, attempt.Message, "write scenario log", "attempt message lost the log failure")
	assert.Contains(t, exec.stdout.(*bytes.Buffer).String(), "write scenario log", "console output lost the log failure")
}

func TestScenarioLoggerCloseReportsFailure(t *testing.T) {
	exec := newTestExecutor(t)
	logger, err := newScenarioLogger(exec, "Example", filepath.Join(t.TempDir(), "attempt.log"))
	require.NoError(t, err)
	require.NoError(t, logger.file.Close())

	closeErr := logger.Close()
	require.ErrorContains(t, closeErr, "scenario log")
	require.Error(t, logger.Close(), "Close forgot the sticky log failure")
}

func TestExecutorDoesNotReportMissingLogAttachment(t *testing.T) {
	logDir := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(logDir, []byte("file"), 0o600))
	exec := newExecutor(context.Background(), &bytes.Buffer{}, runOptions{
		parallel:   1,
		logDir:     logDir,
		outputMode: "grouped",
	}, 1)

	exec.schedule("LogOpenFails", &Scenario{Name: "LogOpenFails"})
	exec.scenarios.Wait()

	attempt := exec.results[0].Attempts[0]
	assert.Equal(t, statusFailed, attempt.Status)
	assert.Empty(t, attempt.LogPath)
	assert.Contains(t, attempt.Message, "create scenario log directory")
}

func TestSkippedScenarioIsNotMarkedFailed(t *testing.T) {
	exec := newTestExecutor(t)

	var failedDuringCleanup bool
	exec.runScenario = func(_ context.Context, _ string, _ toolkit.Logger, s *Scenario) (runErr error) {
		defer func() { markScenarioOutcome(s, runErr, recover()) }()
		s.Cleanup(func(context.Context) error {
			failedDuringCleanup = s.failed
			return nil
		})
		return &skipError{message: "SKU not available"}
	}

	exec.schedule("Skipped", &Scenario{Name: "Skipped"})
	exec.scenarios.Wait()

	assert.False(t, failedDuringCleanup, "a skipped scenario was marked failed")
	assert.Equal(t, statusSkipped, exec.results[0].Status)
}

func TestPanicMarksScenarioFailedBeforeCleanup(t *testing.T) {
	exec := newTestExecutor(t)

	var failedDuringCleanup bool
	exec.runScenario = func(_ context.Context, _ string, _ toolkit.Logger, s *Scenario) (runErr error) {
		defer func() { markScenarioOutcome(s, runErr, recover()) }()
		s.Cleanup(func(context.Context) error {
			failedDuringCleanup = s.failed
			return nil
		})
		panic("boom")
	}

	exec.schedule("Panics", &Scenario{Name: "Panics"})
	exec.scenarios.Wait()

	assert.True(t, failedDuringCleanup, "panicking scenario was not marked failed before cleanup")
	attempt := exec.results[0].Attempts[0]
	assert.Equal(t, statusFailed, attempt.Status)
	assert.Contains(t, attempt.Message, "panic: boom", "executor did not recover the re-raised panic")
}

func TestExecutorKeepsFailureWhenRetrySkips(t *testing.T) {
	tmp := t.TempDir()
	opts := runOptions{
		parallel:   1,
		retries:    1,
		logDir:     filepath.Join(tmp, "logs"),
		junitFile:  filepath.Join(tmp, "report.xml"),
		outputMode: "grouped",
	}
	exec := newExecutor(context.Background(), &bytes.Buffer{}, opts, 1)

	var calls atomic.Int32
	exec.runScenario = func(_ context.Context, _ string, _ toolkit.Logger, _ *Scenario) error {
		if calls.Add(1) == 1 {
			return errors.New("validation failed")
		}
		return &skipError{message: "SKU not available"}
	}

	exec.schedule("FailsThenSkips", &Scenario{Name: "FailsThenSkips"})
	exec.scenarios.Wait()
	require.NoError(t, writeJUnitReport(opts.junitFile, exec.snapshotResults(nil)))

	summary := exec.summary()
	assert.Equal(t, 1, summary.Failed)
	assert.Zero(t, summary.Skipped)
	assert.Equal(t, 1, summary.Total)
	report, err := os.ReadFile(opts.junitFile)
	require.NoError(t, err)
	assert.Contains(t, string(report), "validation failed", "JUnit report dropped the original failure")
	assert.NotContains(t, string(report), "<skipped", "JUnit report recorded the scenario as skipped")
}
