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

func TestRunnerFlagsUseEnvironmentAliases(t *testing.T) {
	oldParallel := config.Config.Parallel
	oldTimeout := config.Config.TestTimeout
	oldSuiteTimeout := config.Config.SuiteTimeout
	oldRetries := config.Config.Retries
	oldLogDir := config.Config.E2ELoggingDir
	oldOutput := config.Config.OutputMode
	oldHidePassed := config.Config.HidePassedLogs
	oldTags := config.Config.TagsToRun
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
	if config.Config.SuiteTimeout != 4*time.Minute || config.Config.OutputMode != "grouped" || !config.Config.HidePassedLogs || config.Config.TagsToRun != "gpu=true" {
		t.Fatalf("string environment aliases were not loaded: %+v", runOptionsFromConfig(nil))
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
		timeout:    time.Second,
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
	if err := exec.writeReports(); err != nil {
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
		timeout:    time.Second,
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
		timeout:    time.Second,
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
		timeout:    time.Second,
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
	err := addCleanupFailure(&skipError{message: "skip"}, errors.New("cleanup failed"))
	var skip *skipError
	if errors.As(err, &skip) {
		t.Fatalf("cleanup failure remained classified as skip: %v", err)
	}
	if !strings.Contains(err.Error(), "skip") || !strings.Contains(err.Error(), "cleanup failed") {
		t.Fatalf("combined error lost context: %v", err)
	}
}

func TestExecutorReportsCancellationAsFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	exec := newExecutor(ctx, &bytes.Buffer{}, runOptions{
		parallel:   1,
		timeout:    time.Second,
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
	exec := newExecutor(context.Background(), &bytes.Buffer{}, runOptions{
		parallel:   1,
		timeout:    time.Second,
		logDir:     t.TempDir(),
		outputMode: "grouped",
	}, 1)

	exec.schedule("Disabled", &Scenario{
		Name:   "Disabled",
		SkipIf: skipScenario("not supported"),
	})
	exec.scenarios.Wait()

	summary := exec.summary()
	if summary.Total != 1 || summary.Skipped != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}
