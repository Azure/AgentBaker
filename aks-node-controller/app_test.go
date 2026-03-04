package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/Azure/agentbaker/aks-node-controller/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
// Note: in production, the CSE shell script writes provision.json and provision.complete on completion.
// On error, writeCompleteFileOnError writes them eagerly so provision-wait can unblock.
// In tests we simulate the CSE writing provision.complete after a successful run.
func TestApp_ProvisionThenWait(t *testing.T) {
	t.Run("provision success → provision-wait succeeds", func(t *testing.T) {
		dir := t.TempDir()
		jsonFile := filepath.Join(dir, "provision.json")
		completeFile := filepath.Join(dir, "provision.complete")

		provisionApp := newTestApp(t, nil)
		require.Equal(t, 0, provisionApp.Run(context.Background(), []string{
			"aks-node-controller", "provision",
			"--provision-config=parser/testdata/test_aksnodeconfig.json",
			"--provision-json-file=" + jsonFile,
			"--provision-complete-file=" + completeFile,
		}))

		// Simulate what the CSE shell script does on success: write provision.json + provision.complete
		require.NoError(t, os.WriteFile(jsonFile, []byte(`{"ExitCode":"0","Output":"ok","Error":""}`), 0644))
		_, err := os.Create(completeFile)
		require.NoError(t, err)

		waitApp := newTestApp(t, nil)
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		assert.Equal(t, 0, waitApp.Run(ctx, []string{
			"aks-node-controller", "provision-wait",
			"--provision-json-file=" + jsonFile,
			"--provision-complete-file=" + completeFile,
		}))
	})

	t.Run("provision failure → provision-wait unblocks with error", func(t *testing.T) {
		dir := t.TempDir()
		jsonFile := filepath.Join(dir, "provision.json")
		completeFile := filepath.Join(dir, "provision.complete")

		// writeCompleteFileOnError writes provision.json + provision.complete eagerly on failure
		provisionApp := newTestApp(t, func(*exec.Cmd) error { return &testExitError{Code: 1} })
		provisionApp.Run(context.Background(), []string{
			"aks-node-controller", "provision",
			"--provision-config=parser/testdata/test_aksnodeconfig.json",
			"--provision-json-file=" + jsonFile,
			"--provision-complete-file=" + completeFile,
		})

		waitApp := newTestApp(t, nil)
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		assert.Equal(t, 1, waitApp.Run(ctx, []string{
			"aks-node-controller", "provision-wait",
			"--provision-json-file=" + jsonFile,
			"--provision-complete-file=" + completeFile,
		}))
	})
}

// Test_parseFlags covers flag parsing defaults, custom values, and required-field validation.
func Test_parseFlags(t *testing.T) {
	t.Run("provision defaults", func(t *testing.T) {
		flags, err := parseProvisionFlags([]string{"--provision-config=foo.json"})
		require.NoError(t, err)
		assert.Equal(t, "foo.json", flags.ProvisionConfig)
		assert.False(t, flags.DryRun)
		assert.Equal(t, defaultEventsDir, flags.EventsDir)
		assert.Equal(t, defaultProvisionJSONFilePath, flags.ProvisionStatusFiles.ProvisionJSONFile)
		assert.Equal(t, defaultProvisionCompleteFilePath, flags.ProvisionStatusFiles.ProvisionCompleteFile)
	})

	t.Run("provision custom paths", func(t *testing.T) {
		flags, err := parseProvisionFlags([]string{
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
		_, err := parseProvisionFlags([]string{})
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

// Test_writeCompleteFileOnError covers the sentinel file writing behaviour.
func Test_writeCompleteFileOnError(t *testing.T) {
	t.Run("no-op on success", func(t *testing.T) {
		app := newTestApp(t, nil)
		dir := t.TempDir()
		p := ProvisionStatusFiles{
			ProvisionJSONFile:     filepath.Join(dir, "provision.json"),
			ProvisionCompleteFile: filepath.Join(dir, "provision.complete"),
		}
		app.writeCompleteFileOnError(p, &ProvisionResult{}, nil)
		_, err := os.Stat(p.ProvisionCompleteFile)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("writes both files on error, does not overwrite existing provision.json", func(t *testing.T) {
		app := newTestApp(t, nil)
		dir := t.TempDir()
		p := ProvisionStatusFiles{
			ProvisionJSONFile:     filepath.Join(dir, "custom", "prov.json"),
			ProvisionCompleteFile: filepath.Join(dir, "custom", "prov.complete"),
		}
		app.writeCompleteFileOnError(p, &ProvisionResult{ExitCode: "240", Error: "fail"}, assert.AnError)
		_, err := os.Stat(p.ProvisionJSONFile)
		assert.NoError(t, err)
		_, err = os.Stat(p.ProvisionCompleteFile)
		assert.NoError(t, err)

		// Write again — existing provision.json must not be overwritten
		original, _ := os.ReadFile(p.ProvisionJSONFile)
		app.writeCompleteFileOnError(p, &ProvisionResult{ExitCode: "1"}, assert.AnError)
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
