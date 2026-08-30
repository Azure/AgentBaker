package e2e

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/toolkit"
)

func TestAppListsRegisteredScenarios(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := NewApp(&stdout, &stderr)

	if code := app.Run(context.Background(), []string{"e2e", "list"}); code != exitSuccess {
		t.Fatalf("list returned %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Ubuntu2204\n") {
		t.Fatalf("list did not contain a known scenario:\n%s", stdout.String())
	}
}

func TestAppRejectsUnknownScenario(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := NewApp(&stdout, &stderr)

	if code := app.Run(context.Background(), []string{"e2e", "run", "--log-dir", t.TempDir(), "DoesNotExist"}); code != exitUsage {
		t.Fatalf("run returned %d, want %d; stderr: %s", code, exitUsage, stderr.String())
	}
}

func TestAppReturnsUsageExitCodeForInvalidFlag(t *testing.T) {
	restoreRunnerConfig(t)
	var stderr bytes.Buffer
	app := NewApp(&bytes.Buffer{}, &stderr)

	if code := app.Run(context.Background(), []string{"e2e", "run", "--parallel", "nope"}); code != exitUsage {
		t.Fatalf("run returned %d, want %d; stderr: %s", code, exitUsage, stderr.String())
	}
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
			if code := app.Run(context.Background(), test.args); code != exitUsage {
				t.Fatalf("run returned %d, want %d; stderr: %s", code, exitUsage, stderr.String())
			}
			if !strings.Contains(stderr.String(), test.message) {
				t.Fatalf("run did not report %q: %s", test.message, stderr.String())
			}
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
			if code := app.Run(context.Background(), []string{"e2e", "run", test.flag, "DoesNotExist"}); code != exitUsage {
				t.Fatalf("run returned %d, want %d; stderr: %s", code, exitUsage, stderr.String())
			}
			if !strings.Contains(stderr.String(), test.message) {
				t.Fatalf("run did not validate %s: %s", test.flag, stderr.String())
			}
		})
	}
}

func TestRunnerFlagsUseEnvironmentAliases(t *testing.T) {
	oldParallel := config.Config.Parallel
	oldTimeout := config.Config.TestTimeout
	oldSuiteTimeout := config.Config.SuiteTimeout
	oldRetries := config.Config.Retries
	oldLogDir := config.Config.E2ELoggingDir
	oldOutput := config.Config.OutputMode
	oldHidePassed := config.Config.HidePassedLogs
	oldTags := config.Config.TagsToRun
	oldSkipTags := config.Config.TagsToSkip
	oldGallery := config.Config.GalleryName
	oldKeepVMSS := config.Config.KeepVMSS
	oldSubscriptionID := config.Config.SubscriptionID
	t.Cleanup(func() {
		config.Config.Parallel = oldParallel
		config.Config.TestTimeout = oldTimeout
		config.Config.SuiteTimeout = oldSuiteTimeout
		config.Config.Retries = oldRetries
		config.Config.E2ELoggingDir = oldLogDir
		config.Config.OutputMode = oldOutput
		config.Config.HidePassedLogs = oldHidePassed
		config.Config.TagsToRun = oldTags
		config.Config.TagsToSkip = oldSkipTags
		config.Config.GalleryName = oldGallery
		config.Config.KeepVMSS = oldKeepVMSS
		config.Config.SubscriptionID = oldSubscriptionID
	})
	t.Setenv("PARALLEL", "7")
	t.Setenv("TEST_TIMEOUT", "3m")
	t.Setenv("E2E_GO_TEST_TIMEOUT", "4m")
	t.Setenv("TIMEOUT", "4m")
	t.Setenv("E2E_FAILED_TESTS_RETRY_COUNT", "2")
	t.Setenv("LOGGING_DIR", t.TempDir())
	t.Setenv("E2E_OUTPUT", "grouped")
	t.Setenv("E2E_HIDE_PASSED_LOGS", "true")
	t.Setenv("TAGS_TO_RUN", "gpu=true")
	t.Setenv("TAGS_TO_SKIP", "os=windows")
	t.Setenv("GALLERY_NAME", "test-gallery")
	t.Setenv("KEEP_VMSS", "true")
	t.Setenv("SUBSCRIPTION_ID", "test-subscription")

	app := NewApp(&bytes.Buffer{}, &bytes.Buffer{})
	if code := app.Run(context.Background(), []string{"e2e", "run", "DoesNotExist"}); code != exitUsage {
		t.Fatalf("run returned %d, want %d", code, exitUsage)
	}
	if config.Config.Parallel != 7 || config.Config.TestTimeout != 3*time.Minute || config.Config.Retries != 2 {
		t.Fatalf("environment aliases were not loaded: %+v", runOptionsFromConfig(nil))
	}
	opts := runOptionsFromConfig(nil)
	if config.Config.SuiteTimeout != 4*time.Minute || config.Config.OutputMode != "grouped" || !config.Config.HidePassedLogs ||
		opts.tagFilter != (tagFilter{run: "gpu=true", skip: "os=windows"}) {
		t.Fatalf("string environment aliases were not loaded: %+v", opts)
	}
	if config.Config.GalleryName != "test-gallery" || !config.Config.KeepVMSS || config.Config.SubscriptionID != "test-subscription" {
		t.Fatalf("infrastructure environment aliases were not loaded: %+v", config.Config)
	}
}

func TestAppRejectsUnknownScenarioChild(t *testing.T) {
	oldLogDir := config.Config.E2ELoggingDir
	oldTimeout := config.Config.TestTimeout
	t.Cleanup(func() {
		config.Config.E2ELoggingDir = oldLogDir
		config.Config.TestTimeout = oldTimeout
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := NewApp(&stdout, &stderr)

	if code := app.Run(context.Background(), []string{"e2e", "run", "--log-dir", t.TempDir(), "Ubuntu2204/not-a-scenario"}); code != exitUsage {
		t.Fatalf("run returned %d, want %d; stderr: %s", code, exitUsage, stderr.String())
	}
}

func TestSelectEntriesUsesParentNameAsGroup(t *testing.T) {
	entries := []scenarioEntry{
		{name: "Group/one"},
		{name: "Group/two"},
		{name: "Other"},
	}
	if got := len(selectEntries(entries, []string{"Group"})); got != 2 {
		t.Fatalf("parent selected %d scenarios, want 2", got)
	}
	if got := len(selectEntries(entries, []string{"Group/one"})); got != 1 {
		t.Fatalf("child selected %d scenarios, want 1", got)
	}
	if got := len(selectEntries(entries, []string{"Group/missing"})); got != 0 {
		t.Fatalf("missing child selected %d scenarios, want 0", got)
	}
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
	exec.runScenario = func(_ context.Context, _ string, logger toolkit.Logger, s *Scenario) error {
		logger.Log("attempt output")
		if calls.Add(1) == 1 {
			err := errors.New("transient failure")
			s.recordCheck("Task_example", time.Second, err)
			return err
		}
		s.recordCheck("Task_example", time.Second, nil)
		return nil
	}

	exec.schedule("Retry", &Scenario{})
	exec.scenarios.Wait()
	if err := exec.writeReports(nil); err != nil {
		t.Fatal(err)
	}

	summary := exec.summary()
	if summary.Flaky != 1 || summary.Failed != 0 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	report, err := os.ReadFile(opts.junitFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(report, []byte(`value="flaky"`)) || !bytes.Contains(report, []byte("[[ATTACHMENT|")) {
		t.Fatalf("JUnit report does not describe flaky attempts:\n%s", report)
	}
	if !bytes.Contains(report, []byte(`name="Retry/Task_example"`)) {
		t.Fatalf("JUnit report does not contain the CSE child result:\n%s", report)
	}
	if !strings.Contains(stdout.String(), "##[group]Retry") {
		t.Fatalf("grouped output missing scenario group:\n%s", stdout.String())
	}
}

func TestExecutorCanHidePassedLogs(t *testing.T) {
	var stdout bytes.Buffer
	exec := newExecutor(context.Background(), &stdout, runOptions{
		parallel:   1,
		logDir:     t.TempDir(),
		outputMode: "grouped",
		hidePassed: true,
	}, 1)
	exec.runScenario = func(_ context.Context, _ string, logger toolkit.Logger, _ *Scenario) error {
		logger.Log("passing output")
		return nil
	}

	exec.schedule("Passed", &Scenario{Name: "Passed"})
	exec.scenarios.Wait()
	if strings.Contains(stdout.String(), "passing output") {
		t.Fatalf("passed output was not hidden:\n%s", stdout.String())
	}
}

func TestScenarioLogIncludesElapsedTime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "attempt.log")
	exec := newExecutor(context.Background(), &bytes.Buffer{}, runOptions{
		parallel:   1,
		logDir:     t.TempDir(),
		outputMode: "grouped",
	}, 1)
	logger, err := newScenarioLogger(exec, "Example", path)
	if err != nil {
		t.Fatal(err)
	}
	logger.Log("hello\nworld")
	if err := logger.Close(); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^\[\d+\.\d{3}s\] hello\n\[\d+\.\d{3}s\] world\n$`).Match(content) {
		t.Fatalf("unexpected scenario log line: %q", content)
	}
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
	if summary.Failed != 1 || summary.Passed != 1 {
		t.Fatalf("unexpected panic summary: %+v", summary)
	}
	if !strings.Contains(stdout.String(), "🔴 FAIL: panic: boom") {
		t.Fatalf("panic was not reported:\n%s", stdout.String())
	}
}

func TestCleanupFailureOverridesSkip(t *testing.T) {
	err := addFailure(&skipError{message: "skip"}, errors.New("cleanup failed"))
	var skip *skipError
	if errors.As(err, &skip) {
		t.Fatalf("cleanup failure remained classified as skip: %v", err)
	}
	if !strings.Contains(err.Error(), "skip") || !strings.Contains(err.Error(), "cleanup failed") {
		t.Fatalf("combined error lost context: %v", err)
	}
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

	if got := ran.Load(); got != 2 {
		t.Fatalf("cleanups ran %d times, want 2 (once per attempt)", got)
	}
	if len(cleanups) != 2 || cleanups[0] == cleanups[1] {
		t.Fatalf("attempts did not get isolated cleanups: %v", cleanups)
	}
	if summary := exec.summary(); summary.Flaky != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
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
			if attempt.Status != statusFailed {
				t.Fatalf("attempt status = %q, want %q", attempt.Status, statusFailed)
			}
			if !strings.Contains(attempt.Message, "delete vmss") {
				t.Fatalf("attempt message lost the cleanup failure: %q", attempt.Message)
			}
			if test.runErr != nil && !strings.Contains(attempt.Message, "not supported") {
				t.Fatalf("attempt message lost the original result: %q", attempt.Message)
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

	if got := ran.Load(); got != 1 {
		t.Fatalf("cleanups ran %d times after a panic, want 1", got)
	}
	attempt := exec.results[0].Attempts[0]
	if attempt.Status != statusFailed || !strings.Contains(attempt.Message, "panic: boom") {
		t.Fatalf("panic attempt = %+v", attempt)
	}
}

func TestFreshScenarioSharesAttemptCleanup(t *testing.T) {
	cleanup := &scenarioCleanup{}
	original := &Scenario{
		Name:     "Staged",
		cleanup:  cleanup,
		Runtime:  &ScenarioRuntime{},
		testName: "Staged",
		failed:   true,
		checks:   []scenarioCheck{{Name: "Task_example"}},
	}

	copied := freshScenario(original)

	if copied.cleanup != cleanup {
		t.Fatal("freshScenario dropped the attempt cleanup")
	}
	if copied.Runtime != nil || copied.Logger != nil || copied.testName != "" || copied.failed || copied.checks != nil {
		t.Fatalf("freshScenario kept per-run state: %+v", copied)
	}
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
	if summary.Failed != 1 || summary.Skipped != 0 {
		t.Fatalf("unexpected cancellation summary: %+v", summary)
	}
	if got := len(exec.results[0].Attempts); got != 1 {
		t.Fatalf("cancellation recorded %d attempts, want 1", got)
	}
}

func TestExecutorWaitAllowsGracefulCancellation(t *testing.T) {
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

	if err := exec.wait(time.Second); err != nil {
		t.Fatalf("graceful cancellation returned an error: %v", err)
	}
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

	if err := exec.wait(10 * time.Millisecond); err == nil || !strings.Contains(err.Error(), "did not stop") {
		t.Fatalf("wait returned %v, want shutdown deadline error", err)
	}
	summary := exec.summary()
	if summary.Total != 1 || summary.Failed != 1 {
		t.Fatalf("unfinished scenario was not recorded as failed: %+v", summary)
	}
	if err := exec.writeReports(nil); err != nil {
		t.Fatal(err)
	}
	report, err := os.ReadFile(exec.opts.junitFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(report, []byte(`failure message="scenarios did not stop`)) {
		t.Fatalf("JUnit report dropped the unfinished scenario:\n%s", report)
	}
	close(release)
	exec.scenarios.Wait()
	if len(exec.results) != 1 {
		t.Fatalf("late scenario result created a duplicate: %+v", exec.results)
	}
}

func TestConciseFailureKeepsTail(t *testing.T) {
	message := strings.Repeat("old context ", 500) + "important final error"
	got := concise(message)
	if !strings.HasSuffix(got, "important final error") {
		t.Fatalf("failure tail was lost: %q", got)
	}
	if !strings.HasPrefix(got, "... beginning truncated") {
		t.Fatalf("missing truncation marker: %q", got)
	}
}

func TestScenarioSkipIfRecordsSkip(t *testing.T) {
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
		Name:   "Disabled",
		SkipIf: skipScenario("not supported"),
	})
	exec.scenarios.Wait()
	if err := exec.writeReports(nil); err != nil {
		t.Fatal(err)
	}
	exec.printSummary(nil)

	summary := exec.summary()
	if summary.Total != 1 || summary.Skipped != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if !strings.Contains(stdout.String(), "Skipped:\n- Disabled: not supported") {
		t.Fatalf("ordinary skip was not listed in the summary: %q", stdout.String())
	}
	if strings.Contains(stdout.String(), "##[group]") {
		t.Fatalf("ordinary skip created a noisy console group: %q", stdout.String())
	}
	report, err := os.ReadFile(opts.junitFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(report, []byte(`<skipped message="not supported"`)) {
		t.Fatalf("JUnit report dropped the ordinary skip:\n%s", report)
	}
}

func TestScenarioTimeoutCoversSkipAndRun(t *testing.T) {
	oldTimeout := config.Config.TestTimeout
	config.Config.TestTimeout = time.Second
	t.Cleanup(func() { config.Config.TestTimeout = oldTimeout })

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

	if skipDeadline.IsZero() || !skipDeadline.Equal(runDeadline) {
		t.Fatalf("SkipIf and scenario received different attempt deadlines: skip=%s run=%s", skipDeadline, runDeadline)
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

	summary := exec.printSummary(nil)
	if summary != (runSummary{Total: 4, Passed: 1, Failed: 1, Skipped: 1, Flaky: 1}) {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	output := stdout.String()
	skippedIndex := strings.Index(output, "\nSkipped:")
	flakyIndex := strings.Index(output, "\nFlaky:")
	failedIndex := strings.Index(output, "\nFailed:")
	if skippedIndex < 0 || flakyIndex <= skippedIndex || failedIndex <= flakyIndex {
		t.Fatalf("summary order is not skipped, flaky, failed:\n%s", output)
	}
	if strings.Contains(output, "- Passed") {
		t.Fatalf("passing scenario name was listed:\n%s", output)
	}
	if !strings.Contains(output, "- Flaky (passed on attempt 2): transient failure") {
		t.Fatalf("flaky scenario details were omitted:\n%s", output)
	}
	if !strings.Contains(output, "- Failed: final failure") || strings.Contains(output, "details") {
		t.Fatalf("failed scenario summary was not concise:\n%s", output)
	}
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
	entries := []scenarioEntry{
		{name: "Excluded", scenario: &Scenario{
			Name:   "Excluded",
			SkipIf: skipScenario("must not be consulted"),
		}},
		{name: "Kept", scenario: &Scenario{Name: "Kept"}},
	}

	runnable, filtered, err := partitionEntries(entries, opts.tagFilter)
	if err != nil {
		t.Fatal(err)
	}
	if len(runnable) != 1 || runnable[0].name != "Kept" || len(filtered) != 1 {
		t.Fatalf("unexpected partition: runnable=%+v filtered=%+v", runnable, filtered)
	}

	exec := newExecutor(context.Background(), &stdout, opts, len(runnable))
	exec.runScenario = func(context.Context, string, toolkit.Logger, *Scenario) error { return nil }
	for _, entry := range runnable {
		exec.schedule(entry.name, entry.scenario)
	}
	exec.scenarios.Wait()
	if err := exec.writeReports(filtered); err != nil {
		t.Fatal(err)
	}

	summary := exec.printSummary(filtered)
	if summary.Total != 2 || summary.Passed != 1 || summary.Skipped != 1 {
		t.Fatalf("filtered scenario was not counted as skipped: %+v", summary)
	}
	if _, err := os.Stat(filepath.Join(opts.logDir, "Excluded")); !os.IsNotExist(err) {
		t.Fatalf("filtered scenario created an attempt log directory: %v", err)
	}
	if !strings.Contains(stdout.String(), "DONE 2 scenarios: 1 passed, 0 flaky, 1 skipped, 0 failed") ||
		!strings.Contains(stdout.String(), "- Excluded: filtered:") {
		t.Fatalf("filtered scenario was not listed as skipped: %q", stdout.String())
	}

	report, err := os.ReadFile(opts.junitFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(report, []byte(`name="Excluded"`)) || !bytes.Contains(report, []byte("<skipped message=\"filtered: scenario &#34;Excluded&#34;")) {
		t.Fatalf("JUnit report dropped the filtered scenario:\n%s", report)
	}
	if !bytes.Contains(report, []byte(`name="Kept"`)) {
		t.Fatalf("JUnit report dropped the runnable scenario:\n%s", report)
	}
}

func TestAutoOutputUsesRunnableCount(t *testing.T) {
	entries := []scenarioEntry{
		{name: "Only", scenario: &Scenario{Name: "Only"}},
		{name: "Second", scenario: &Scenario{Name: "Second"}},
		{name: "Third", scenario: &Scenario{Name: "Third"}},
		{name: "Fourth", scenario: &Scenario{Name: "Fourth"}},
	}
	opts := runOptions{parallel: 1, logDir: t.TempDir(), outputMode: "auto"}

	runnable, filtered, err := partitionEntries(entries, tagFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(runnable) != 4 || len(filtered) != 0 {
		t.Fatalf("unfiltered partition = %d runnable, %d filtered", len(runnable), len(filtered))
	}
	if newExecutor(context.Background(), &bytes.Buffer{}, opts, len(runnable)).stream {
		t.Fatal("auto mode streamed more than three scenarios")
	}

	runnable, filtered, err = partitionEntries(entries, tagFilter{run: "Name=Only"})
	if err != nil {
		t.Fatal(err)
	}
	if len(runnable) != 1 || len(filtered) != 3 {
		t.Fatalf("filtered partition = %d runnable, %d filtered", len(runnable), len(filtered))
	}
	if !newExecutor(context.Background(), &bytes.Buffer{}, opts, len(runnable)).stream {
		t.Fatal("auto mode did not stream a single runnable scenario")
	}
}

func TestExecutorDoesNotReadGlobalTagFilters(t *testing.T) {
	oldRun, oldSkip := config.Config.TagsToRun, config.Config.TagsToSkip
	t.Cleanup(func() {
		config.Config.TagsToRun = oldRun
		config.Config.TagsToSkip = oldSkip
	})
	config.Config.TagsToRun = "gpu=true"
	config.Config.TagsToSkip = "Name=Runs"

	exec := newTestExecutor(t)
	exec.runScenario = func(context.Context, string, toolkit.Logger, *Scenario) error {
		return nil
	}
	exec.schedule("Runs", &Scenario{Name: "Runs"})
	exec.scenarios.Wait()

	if result := exec.results[0]; result.Status != statusPassed {
		t.Fatalf("global filters changed executor result: %+v", result)
	}
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
	if attempt.Status != statusFailed || exec.results[0].Status != statusFailed {
		t.Fatalf("log failure did not fail the attempt: %+v", exec.results[0])
	}
	if !strings.Contains(attempt.Message, "write scenario log") {
		t.Fatalf("attempt message lost the log failure: %q", attempt.Message)
	}
	if !strings.Contains(exec.stdout.(*bytes.Buffer).String(), "write scenario log") {
		t.Fatalf("console output lost the log failure: %q", exec.stdout.(*bytes.Buffer).String())
	}
}

func TestScenarioLoggerCloseReportsFailure(t *testing.T) {
	exec := newTestExecutor(t)
	logger, err := newScenarioLogger(exec, "Example", filepath.Join(t.TempDir(), "attempt.log"))
	if err != nil {
		t.Fatal(err)
	}
	if err := logger.file.Close(); err != nil {
		t.Fatal(err)
	}

	closeErr := logger.Close()
	if closeErr == nil || !strings.Contains(closeErr.Error(), "scenario log") {
		t.Fatalf("Close did not report the failure: %v", closeErr)
	}
	if err := logger.Close(); err == nil {
		t.Fatal("Close forgot the sticky log failure")
	}
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

	if failedDuringCleanup {
		t.Fatal("a skipped scenario was marked failed")
	}
	if exec.results[0].Status != statusSkipped {
		t.Fatalf("unexpected result: %+v", exec.results[0])
	}
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

	if !failedDuringCleanup {
		t.Fatal("panicking scenario was not marked failed before cleanup")
	}
	attempt := exec.results[0].Attempts[0]
	if attempt.Status != statusFailed || !strings.Contains(attempt.Message, "panic: boom") {
		t.Fatalf("executor did not recover the re-raised panic: %+v", attempt)
	}
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
	if err := exec.writeReports(nil); err != nil {
		t.Fatal(err)
	}

	summary := exec.summary()
	if summary.Failed != 1 || summary.Skipped != 0 || summary.Total != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	report, err := os.ReadFile(opts.junitFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(report, []byte("validation failed")) {
		t.Fatalf("JUnit report dropped the original failure:\n%s", report)
	}
	if bytes.Contains(report, []byte("<skipped")) {
		t.Fatalf("JUnit report recorded the scenario as skipped:\n%s", report)
	}
}
