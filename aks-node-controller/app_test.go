package main

import (
	"context"
	"errors"
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

type TestAppConfig struct {
	RunFunc func(*exec.Cmd) error
}

type TestApp struct {
	App         *App
	eventLogger *helpers.EventLogger
}

func NewTestApp(t *testing.T, cfg TestAppConfig) *TestApp {
	eventsDir := t.TempDir()
	runFunc := cfg.RunFunc
	if runFunc == nil {
		runFunc = func(*exec.Cmd) error { return nil }
	}
	eventLogger := helpers.NewEventLogger(eventsDir)
	return &TestApp{
		eventLogger: eventLogger,
		App: &App{
			cmdRunner:   runFunc,
			eventLogger: eventLogger,
		},
	}
}

func TestApp_Run(t *testing.T) {
	t.Run("missing command argument", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		exitCode := tt.App.Run(context.Background(), []string{"aks-node-controller"})
		assert.Equal(t, 1, exitCode)
	})

	t.Run("unknown command", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		exitCode := tt.App.Run(context.Background(), []string{"aks-node-controller", "unknown"})
		assert.Equal(t, 1, exitCode)
	})

	t.Run("provision command with missing flag", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		exitCode := tt.App.Run(context.Background(), []string{"aks-node-controller", "provision"})
		assert.Equal(t, 1, exitCode)
	})

	t.Run("provision command with valid flag", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		exitCode := tt.App.Run(context.Background(), []string{"aks-node-controller", "provision", "--provision-config=parser/testdata/test_aksnodeconfig.json"})
		assert.Equal(t, 0, exitCode)

		events := tt.eventLogger.Events()
		assert.Len(t, events, 2)
		assert.Contains(t, events[0].Message, "Starting")
		assert.Contains(t, events[1].Message, "Completed")
	})

	t.Run("provision command with command runner error", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{
			RunFunc: func(*exec.Cmd) error { return &testExitError{Code: 666} },
		})
		exitCode := tt.App.Run(context.Background(), []string{"aks-node-controller", "provision", "--provision-config=parser/testdata/test_aksnodeconfig.json"})
		assert.Equal(t, 666, exitCode)

		events := tt.eventLogger.Events()
		assert.Len(t, events, 2)
		assert.Equal(t, "Error", events[1].EventLevel)
	})
}

func TestApp_Provision(t *testing.T) {
	t.Run("valid provision config", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		_, err := tt.App.Provision(context.Background(), ProvisionFlags{ProvisionConfig: "parser/testdata/test_aksnodeconfig.json"})
		assert.NoError(t, err)
	})

	t.Run("invalid provision config path", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		_, err := tt.App.Provision(context.Background(), ProvisionFlags{ProvisionConfig: "invalid.json"})
		assert.Error(t, err)
	})

	t.Run("command runner error", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{
			RunFunc: func(*exec.Cmd) error { return errors.New("command runner error") },
		})
		_, err := tt.App.Provision(context.Background(), ProvisionFlags{ProvisionConfig: "parser/testdata/test_aksnodeconfig.json"})
		assert.Error(t, err)
	})
}

func TestApp_Provision_DryRun(t *testing.T) {
	tt := NewTestApp(t, TestAppConfig{})
	tt.App.cmdRunner = cmdRunner // Use real cmdRunner to test dry-run override
	exitCode := tt.App.Run(context.Background(), []string{"aks-node-controller", "provision", "--provision-config=parser/testdata/test_aksnodeconfig.json", "--dry-run"})
	assert.Equal(t, 0, exitCode)
	if reflect.ValueOf(tt.App.cmdRunner).Pointer() != reflect.ValueOf(cmdRunnerDryRun).Pointer() {
		t.Fatal("app.cmdRunner is expected to be cmdRunnerDryRun")
	}
}

func TestApp_ProvisionWait(t *testing.T) {
	testData := `{"ExitCode": "0", "Output": "hello world", "Error": ""}`

	t.Run("event path (file created after call)", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		tempDir := t.TempDir()
		p := ProvisionStatusFiles{
			ProvisionJSONFile:     filepath.Join(tempDir, "provision.json"),
			ProvisionCompleteFile: filepath.Join(tempDir, "provision.complete"),
		}

		go func() {
			time.Sleep(150 * time.Millisecond)
			_ = os.WriteFile(p.ProvisionJSONFile, []byte(testData), 0644)
			_, _ = os.Create(p.ProvisionCompleteFile)
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		data, err := tt.App.ProvisionWait(ctx, p)
		assert.NoError(t, err)
		assert.Equal(t, testData, data)
	})

	t.Run("fast path (file exists before call)", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		tempDir := t.TempDir()
		p := ProvisionStatusFiles{
			ProvisionJSONFile:     filepath.Join(tempDir, "provision.json"),
			ProvisionCompleteFile: filepath.Join(tempDir, "provision.complete"),
		}
		_ = os.WriteFile(p.ProvisionJSONFile, []byte(testData), 0644)
		_, _ = os.Create(p.ProvisionCompleteFile)

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		data, err := tt.App.ProvisionWait(ctx, p)
		assert.NoError(t, err)
		assert.Equal(t, testData, data)
	})

	t.Run("provision completion with failure ExitCode", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		tempDir := t.TempDir()
		p := ProvisionStatusFiles{
			ProvisionJSONFile:     filepath.Join(tempDir, "provision.json"),
			ProvisionCompleteFile: filepath.Join(tempDir, "provision.complete"),
		}
		failJSON := `{"ExitCode": "7", "Error": "boom", "Output": "trace"}`
		_ = os.WriteFile(p.ProvisionJSONFile, []byte(failJSON), 0644)
		_, _ = os.Create(p.ProvisionCompleteFile)

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		_, err := tt.App.ProvisionWait(ctx, p)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "provision failed")
	})

	t.Run("timeout waiting for completion", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		tempDir := t.TempDir()
		p := ProvisionStatusFiles{
			ProvisionJSONFile:     filepath.Join(tempDir, "provision.json"),
			ProvisionCompleteFile: filepath.Join(tempDir, "provision.complete"),
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		_, err := tt.App.ProvisionWait(ctx, p)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context deadline exceeded")
	})
}

func Test_readAndEvaluateProvision(t *testing.T) {
	writeTemp := func(t *testing.T, content string) string {
		t.Helper()
		f, err := os.CreateTemp(t.TempDir(), "provision_*.json")
		require.NoError(t, err)
		_, err = f.WriteString(content)
		require.NoError(t, err)
		f.Close()
		return f.Name()
	}

	t.Run("valid provision file", func(t *testing.T) {
		p := writeTemp(t, `{"ExitCode":"0","Output":"ok","Error":""}`)
		got, err := readAndEvaluateProvision(p)
		assert.NoError(t, err)
		assert.Contains(t, got, `"ExitCode":"0"`)
	})

	t.Run("missing provision file", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "does_not_exist.json")
		_, err := readAndEvaluateProvision(p)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no such file")
	})

	t.Run("invalid provision file (bad JSON)", func(t *testing.T) {
		p := writeTemp(t, `not-json`)
		_, err := readAndEvaluateProvision(p)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character")
	})

	t.Run("non-zero ExitCode returns error", func(t *testing.T) {
		p := writeTemp(t, `{"ExitCode":"7","Output":"boom","Error":"bad"}`)
		_, err := readAndEvaluateProvision(p)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "provision failed")
	})

	t.Run("invalid ExitCode returns error", func(t *testing.T) {
		p := writeTemp(t, `{"ExitCode":"unknown","Output":"boom","Error":"bad"}`)
		_, err := readAndEvaluateProvision(p)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid ExitCode")
	})

	t.Run("missing ExitCode returns error", func(t *testing.T) {
		p := writeTemp(t, `{"Output":"boom","Error":"bad"}`)
		_, err := readAndEvaluateProvision(p)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing ExitCode")
	})
}
