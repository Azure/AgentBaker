package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Azure/agentbaker/aks-node-controller/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pkgDir is the directory of this test file, used to locate the package for go build.
var pkgDir = func() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("runtime.Caller failed")
	}
	return filepath.Dir(file)
}()

type testExitError struct {
	Code int
}

func (e *testExitError) Error() string {
	return "exit status " + strconv.Itoa(e.ExitCode())
}

func (e *testExitError) ExitCode() int {
	return e.Code
}

func newTestApp(t *testing.T, runFunc func(*exec.Cmd) error) *App {
	t.Helper()
	if runFunc == nil {
		runFunc = func(*exec.Cmd) error { return nil }
	}
	return &App{
		cmdRun:      runFunc,
		eventLogger: helpers.NewEventLogger(t.TempDir()),
	}
}

// TestApp_Run covers top-level dispatch, exit code propagation, and event logging.
func TestApp_Run(t *testing.T) {
	t.Run("missing command", func(t *testing.T) {
		assert.Equal(t, 1, newTestApp(t, nil).Run(context.Background(), []string{"aks-node-controller"}))
	})

	t.Run("unknown command", func(t *testing.T) {
		assert.Equal(t, 1, newTestApp(t, nil).Run(context.Background(), []string{"aks-node-controller", "unknown"}))
	})

	t.Run("provision: missing --provision-config", func(t *testing.T) {
		assert.Equal(t, 1, newTestApp(t, nil).Run(context.Background(), []string{"aks-node-controller", "provision"}))
	})

	t.Run("provision: flag-parse error writes sentinel files", func(t *testing.T) {
		dir := t.TempDir()
		jsonFile := filepath.Join(dir, "provision.json")
		completeFile := filepath.Join(dir, "provision.complete")
		app := newTestApp(t, nil)
		exitCode := app.Run(context.Background(), []string{
			"aks-node-controller", "provision",
			"--provision-json-file=" + jsonFile,
			"--provision-complete-file=" + completeFile,
			// no --provision-config → parse validation error
		})
		assert.Equal(t, 1, exitCode)
		_, err := os.Stat(jsonFile)
		assert.NoError(t, err, "provision.json should be written on flag-parse error")
		_, err = os.Stat(completeFile)
		assert.NoError(t, err, "provision.complete should be written on flag-parse error")
	})

	t.Run("provision: success emits Starting+Completed events", func(t *testing.T) {
		app := newTestApp(t, nil)
		exitCode := app.Run(context.Background(), []string{"aks-node-controller", "provision", "--provision-config=parser/testdata/test_aksnodeconfig.json"})
		assert.Equal(t, 0, exitCode)
		events := app.eventLogger.Events()
		require.Len(t, events, 2)
		assert.Contains(t, events[0].Message, "Starting")
		assert.Contains(t, events[1].Message, "Completed")
	})

	t.Run("provision: exit code preserved from runner", func(t *testing.T) {
		app := newTestApp(t, func(*exec.Cmd) error { return &testExitError{Code: 42} })
		exitCode := app.Run(context.Background(), []string{"aks-node-controller", "provision", "--provision-config=parser/testdata/test_aksnodeconfig.json"})
		assert.Equal(t, 42, exitCode)
		assert.Equal(t, "Error", app.eventLogger.Events()[1].EventLevel)
	})

	t.Run("provision: --dry-run switches runner", func(t *testing.T) {
		app := newTestApp(t, cmdRunner)
		exitCode := app.Run(context.Background(), []string{"aks-node-controller", "provision", "--provision-config=parser/testdata/test_aksnodeconfig.json", "--dry-run"})
		assert.Equal(t, 0, exitCode)
		assert.Equal(t, reflect.ValueOf(cmdRunnerDryRun).Pointer(), reflect.ValueOf(app.cmdRun).Pointer())
	})

	t.Run("provision: panic writes sentinel files and returns error", func(t *testing.T) {
		dir := t.TempDir()
		jsonFile := filepath.Join(dir, "provision.json")
		completeFile := filepath.Join(dir, "provision.complete")
		app := newTestApp(t, func(*exec.Cmd) error { panic("test panic") })
		exitCode := app.Run(context.Background(), []string{
			"aks-node-controller", "provision",
			"--provision-config=parser/testdata/test_aksnodeconfig.json",
			"--provision-json-file=" + jsonFile,
			"--provision-complete-file=" + completeFile,
		})
		assert.Equal(t, 1, exitCode)
		_, err := os.Stat(completeFile)
		assert.NoError(t, err, "provision.complete should be written after panic")
	})
}

// TestApp_ProvisionWait covers the file-watching logic.
func TestApp_ProvisionWait(t *testing.T) {
	successJSON := `{"ExitCode": "0", "Output": "hello world", "Error": ""}`

	t.Run("fast path: file exists before call", func(t *testing.T) {
		app := newTestApp(t, nil)
		dir := t.TempDir()
		p := ProvisionStatusFiles{
			ProvisionJSONFile:     filepath.Join(dir, "provision.json"),
			ProvisionCompleteFile: filepath.Join(dir, "provision.complete"),
		}
		require.NoError(t, os.WriteFile(p.ProvisionJSONFile, []byte(successJSON), 0644))
		_, err := os.Create(p.ProvisionCompleteFile)
		require.NoError(t, err)

		data, err := app.ProvisionWait(context.Background(), p)
		assert.NoError(t, err)
		assert.Equal(t, successJSON, data)
	})

	t.Run("event path: file created after call", func(t *testing.T) {
		app := newTestApp(t, nil)
		dir := t.TempDir()
		p := ProvisionStatusFiles{
			ProvisionJSONFile:     filepath.Join(dir, "provision.json"),
			ProvisionCompleteFile: filepath.Join(dir, "provision.complete"),
		}
		go func() {
			time.Sleep(100 * time.Millisecond)
			_ = os.WriteFile(p.ProvisionJSONFile, []byte(successJSON), 0644)
			_, _ = os.Create(p.ProvisionCompleteFile)
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		data, err := app.ProvisionWait(ctx, p)
		assert.NoError(t, err)
		assert.Equal(t, successJSON, data)
	})

	t.Run("timeout", func(t *testing.T) {
		app := newTestApp(t, nil)
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		_, err := app.ProvisionWait(ctx, ProvisionStatusFiles{
			ProvisionJSONFile:     filepath.Join(t.TempDir(), "provision.json"),
			ProvisionCompleteFile: filepath.Join(t.TempDir(), "provision.complete"),
		})
		assert.ErrorContains(t, err, "context deadline exceeded")
	})
}

// TestApp_ProvisionThenWait is an integration test covering the full provision → provision-wait flow
// using custom paths, verifying both commands agree on the same files.
// Note: in production, provision-wait runs concurrently with provision at an unknown start time.
// On any outcome (success or failure), notifyProvisionComplete writes provision.json + provision.complete
// so provision-wait unblocks.
func TestApp_ProvisionThenWait(t *testing.T) {
	t.Run("provision success → provision-wait succeeds (concurrent)", func(t *testing.T) {
		dir := t.TempDir()
		jsonFile := filepath.Join(dir, "provision.json")
		completeFile := filepath.Join(dir, "provision.complete")

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// Start provision-wait first, before provision runs — simulates the race in production
		waitDone := make(chan int, 1)
		go func() {
			waitApp := newTestApp(t, nil)
			waitDone <- waitApp.Run(ctx, []string{
				"aks-node-controller", "provision-wait",
				"--provision-json-file=" + jsonFile,
				"--provision-complete-file=" + completeFile,
			})
		}()

		// Run provision with a real command so ProcessState is populated and ExitCode is 0.
		// notifyProvisionComplete writes provision.json + provision.complete on completion,
		// unblocking provision-wait without any extra goroutine.
		provisionApp := newTestApp(t, func(cmd *exec.Cmd) error {
			// Run a no-op subprocess so cmd.ProcessState is set with ExitCode 0.
			noop := exec.Command("true")
			if err := noop.Run(); err != nil {
				return err
			}
			cmd.ProcessState = noop.ProcessState
			return nil
		})
		require.Equal(t, 0, provisionApp.Run(context.Background(), []string{
			"aks-node-controller", "provision",
			"--provision-config=parser/testdata/test_aksnodeconfig.json",
			"--provision-json-file=" + jsonFile,
			"--provision-complete-file=" + completeFile,
		}))

		assert.Equal(t, 0, <-waitDone)
	})

	t.Run("provision failure → provision-wait unblocks with error (concurrent)", func(t *testing.T) {
		dir := t.TempDir()
		jsonFile := filepath.Join(dir, "provision.json")
		completeFile := filepath.Join(dir, "provision.complete")

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// Start provision-wait before provision runs
		waitDone := make(chan int, 1)
		go func() {
			waitApp := newTestApp(t, nil)
			waitDone <- waitApp.Run(ctx, []string{
				"aks-node-controller", "provision-wait",
				"--provision-json-file=" + jsonFile,
				"--provision-complete-file=" + completeFile,
			})
		}()

		// Run provision — writeCompleteFileOnError writes the sentinel files eagerly on failure
		provisionApp := newTestApp(t, func(*exec.Cmd) error { return &testExitError{Code: 1} })
		provisionApp.Run(context.Background(), []string{
			"aks-node-controller", "provision",
			"--provision-config=parser/testdata/test_aksnodeconfig.json",
			"--provision-json-file=" + jsonFile,
			"--provision-complete-file=" + completeFile,
		})

		assert.Equal(t, 1, <-waitDone)
	})
}

// Test_parseFlags covers flag parsing defaults, custom values, and required-field validation.
func Test_parseFlags(t *testing.T) {
	t.Run("provision defaults", func(t *testing.T) {
		_, flags, err := parseProvisionFlags([]string{"--provision-config=foo.json"})
		require.NoError(t, err)
		assert.Equal(t, "foo.json", flags.ProvisionConfig)
		assert.False(t, flags.DryRun)
		assert.Equal(t, defaultEventsDir, flags.EventsDir)
		assert.Equal(t, defaultProvisionJSONFilePath, flags.ProvisionStatusFiles.ProvisionJSONFile)
		assert.Equal(t, defaultProvisionCompleteFilePath, flags.ProvisionStatusFiles.ProvisionCompleteFile)
	})

	t.Run("provision custom paths", func(t *testing.T) {
		_, flags, err := parseProvisionFlags([]string{
			"--provision-config=foo.json",
			"--events-dir=/tmp/e",
			"--provision-json-file=/tmp/p.json",
			"--provision-complete-file=/tmp/p.complete",
		})
		require.NoError(t, err)
		assert.Equal(t, "/tmp/e", flags.EventsDir)
		assert.Equal(t, "/tmp/p.json", flags.ProvisionStatusFiles.ProvisionJSONFile)
		assert.Equal(t, "/tmp/p.complete", flags.ProvisionStatusFiles.ProvisionCompleteFile)
	})

	t.Run("provision missing --provision-config", func(t *testing.T) {
		_, _, err := parseProvisionFlags([]string{})
		assert.ErrorContains(t, err, "--provision-config is required")
	})

	t.Run("provision-wait defaults", func(t *testing.T) {
		flags, err := parseProvisionWaitFlags([]string{})
		require.NoError(t, err)
		assert.Equal(t, defaultEventsDir, flags.EventsDir)
		assert.Equal(t, defaultProvisionJSONFilePath, flags.ProvisionStatusFiles.ProvisionJSONFile)
		assert.Equal(t, defaultProvisionCompleteFilePath, flags.ProvisionStatusFiles.ProvisionCompleteFile)
	})

	t.Run("provision-wait custom paths", func(t *testing.T) {
		flags, err := parseProvisionWaitFlags([]string{
			"--events-dir=/tmp/e",
			"--provision-json-file=/tmp/p.json",
			"--provision-complete-file=/tmp/p.complete",
		})
		require.NoError(t, err)
		assert.Equal(t, "/tmp/e", flags.EventsDir)
		assert.Equal(t, "/tmp/p.json", flags.ProvisionStatusFiles.ProvisionJSONFile)
		assert.Equal(t, "/tmp/p.complete", flags.ProvisionStatusFiles.ProvisionCompleteFile)
	})
}

// Test_notifyProvisionComplete covers the sentinel file writing behaviour.
func Test_notifyProvisionComplete(t *testing.T) {
	t.Run("writes both files", func(t *testing.T) {
		dir := t.TempDir()
		p := ProvisionStatusFiles{
			ProvisionJSONFile:     filepath.Join(dir, "custom", "prov.json"),
			ProvisionCompleteFile: filepath.Join(dir, "custom", "prov.complete"),
		}
		p.notifyProvisionComplete(&ProvisionResult{ExitCode: "240", Error: "fail"})
		_, err := os.Stat(p.ProvisionJSONFile)
		assert.NoError(t, err)
		_, err = os.Stat(p.ProvisionCompleteFile)
		assert.NoError(t, err)
	})

	t.Run("idempotent: second call does not overwrite provision.json", func(t *testing.T) {
		dir := t.TempDir()
		p := ProvisionStatusFiles{
			ProvisionJSONFile:     filepath.Join(dir, "custom", "prov.json"),
			ProvisionCompleteFile: filepath.Join(dir, "custom", "prov.complete"),
		}
		p.notifyProvisionComplete(&ProvisionResult{ExitCode: "240", Error: "fail"})
		original, _ := os.ReadFile(p.ProvisionJSONFile)
		p.notifyProvisionComplete(&ProvisionResult{ExitCode: "1"})
		after, _ := os.ReadFile(p.ProvisionJSONFile)
		assert.Equal(t, original, after)
	})
}

// Test_readAndEvaluateProvision covers the JSON parsing and exit-code evaluation.
func Test_readAndEvaluateProvision(t *testing.T) {
	write := func(t *testing.T, content string) string {
		t.Helper()
		f, err := os.CreateTemp(t.TempDir(), "provision_*.json")
		require.NoError(t, err)
		_, err = f.WriteString(content)
		require.NoError(t, err)
		f.Close()
		return f.Name()
	}

	t.Run("success", func(t *testing.T) {
		got, err := readAndEvaluateProvision(write(t, `{"ExitCode":"0","Output":"ok","Error":""}`))
		assert.NoError(t, err)
		assert.Contains(t, got, `"ExitCode":"0"`)
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := readAndEvaluateProvision(filepath.Join(t.TempDir(), "missing.json"))
		assert.ErrorContains(t, err, "no such file")
	})

	t.Run("non-zero exit code", func(t *testing.T) {
		_, err := readAndEvaluateProvision(write(t, `{"ExitCode":"7","Output":"boom","Error":"bad"}`))
		assert.ErrorContains(t, err, "provision failed")
	})

	t.Run("malformed JSON", func(t *testing.T) {
		_, err := readAndEvaluateProvision(write(t, `not-json`))
		assert.Error(t, err)
	})
}

// TestProvisionWait_Stdout runs provision-wait as a subprocess and verifies that
// provision.json content is printed to stdout — the machine-readable interface.
// Using a subprocess avoids os.Stdout mutation races present in in-process capture.
// The sentinel files are written after a short delay to exercise the fsnotify event path.
func TestProvisionWait_Stdout(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "aks-node-controller")
	buildCmd := exec.Command("go", "build", "-o", bin, ".")
	buildCmd.Dir = pkgDir
	require.NoError(t, buildCmd.Run())

	dir := t.TempDir()
	jsonContent := `{"ExitCode":"0","Output":"ok","Error":""}`
	jsonFile := filepath.Join(dir, "provision.json")
	completeFile := filepath.Join(dir, "provision.complete")

	// Write sentinel files after a delay — binary is already built, so the delay
	// reliably fires after the process has started, exercising the fsnotify event path.
	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = os.WriteFile(jsonFile, []byte(jsonContent), 0644)
		_, _ = os.Create(completeFile)
	}()

	cmd := exec.Command(bin, "provision-wait",
		"--provision-json-file="+jsonFile,
		"--provision-complete-file="+completeFile,
		"--log-path="+filepath.Join(dir, "test.log"),
	)
	out, err := cmd.Output()
	require.NoError(t, err)
	assert.JSONEq(t, jsonContent, strings.TrimSpace(string(out)))
}
