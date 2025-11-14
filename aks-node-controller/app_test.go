package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// MockCmdRunner is a simple mock for cmdRunner.
type MockCmdRunner struct {
	RunFunc func(cmd *exec.Cmd) error
}

func (m *MockCmdRunner) Run(cmd *exec.Cmd) error {
	if m.RunFunc != nil {
		return m.RunFunc(cmd)
	}
	return nil
}

type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return "exit status " + strconv.Itoa(e.ExitCode())
}

func (e *ExitError) ExitCode() int {
	return e.Code
}

func TestApp_Run(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		setupMocks func(*MockCmdRunner)
		wantExit   int
	}{
		{
			name:     "missing command argument",
			args:     []string{"aks-node-controller"},
			wantExit: 1,
		},
		{
			name:     "unknown command",
			args:     []string{"aks-node-controller", "unknown"},
			wantExit: 1,
		},
		{
			name:     "provision command with missing flag",
			args:     []string{"provision"},
			wantExit: 1,
		},
		{
			name: "provision command with valid flag",
			args: []string{"aks-node-controller", "provision", "--provision-config=parser/testdata/test_aksnodeconfig.json"},
			setupMocks: func(mc *MockCmdRunner) {
				mc.RunFunc = func(cmd *exec.Cmd) error {
					return nil
				}
			},
			wantExit: 0,
		},
		{
			name: "provision command with command runner error",
			args: []string{"aks-node-controller", "provision", "--provision-config=parser/testdata/test_aksnodeconfig.json"},
			setupMocks: func(mc *MockCmdRunner) {
				mc.RunFunc = func(cmd *exec.Cmd) error {
					return &ExitError{Code: 666}
				}
			},
			wantExit: 666,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := new(MockCmdRunner)
			if tt.setupMocks != nil {
				tt.setupMocks(mc)
			}

			app := &App{
				cmdRunner: mc.Run,
			}

			exitCode := app.Run(context.Background(), tt.args)
			assert.Equal(t, tt.wantExit, exitCode)
		})
	}
}

func TestApp_Provision(t *testing.T) {
	tests := []struct {
		name       string
		flags      ProvisionFlags
		setupMocks func(*MockCmdRunner)
		wantErr    bool
	}{
		{
			name:    "valid provision config",
			flags:   ProvisionFlags{ProvisionConfig: "parser/testdata/test_aksnodeconfig.json"},
			wantErr: false,
		},
		{
			name:    "invalid provision config path",
			flags:   ProvisionFlags{ProvisionConfig: "invalid.json"},
			wantErr: true,
		},
		{
			name:  "command runner error",
			flags: ProvisionFlags{ProvisionConfig: "parser/testdata/test_aksnodeconfig.json"},
			setupMocks: func(mc *MockCmdRunner) {
				mc.RunFunc = func(cmd *exec.Cmd) error { return errors.New("command runner error") }
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := &MockCmdRunner{}
			if tt.setupMocks != nil {
				tt.setupMocks(mc)
			}
			app := &App{cmdRunner: mc.Run}
			err := app.Provision(context.Background(), tt.flags)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestApp_Provision_DryRun(t *testing.T) {
	app := &App{cmdRunner: cmdRunner}
	result := app.Run(context.Background(), []string{"aks-node-controller", "provision", "--provision-config=parser/testdata/test_aksnodeconfig.json", "--dry-run"})
	assert.Equal(t, 0, result)
	if reflect.ValueOf(app.cmdRunner).Pointer() != reflect.ValueOf(cmdRunnerDryRun).Pointer() {
		t.Fatal("app.cmdRunner is expected to be cmdRunnerDryRun")
	}
}

func TestApp_ProvisionWait(t *testing.T) {
	testData := `{"ExitCode": "0", "Output": "hello world", "Error": ""}`
	tests := []struct {
		name      string
		wantsErr  bool
		errString string
		setup     func(ProvisionStatusFiles)
	}{
		{
			name: "provision already complete",
			setup: func(p ProvisionStatusFiles) {
				go func() {
					time.Sleep(200 * time.Millisecond)
					_ = os.WriteFile(p.ProvisionJSONFile, []byte(testData), 0644)
					_, _ = os.Create(p.ProvisionCompleteFile)
				}()
			},
		},
		{
			name: "wait for provision completion",
			setup: func(p ProvisionStatusFiles) {
				_ = os.WriteFile(p.ProvisionJSONFile, []byte(testData), 0644)
				_, _ = os.Create(p.ProvisionCompleteFile)
			},
		},
		{
			name:      "provision completion with failure ExitCode",
			wantsErr:  true,
			errString: "provision failed",
			setup: func(p ProvisionStatusFiles) {
				failJSON := `{"ExitCode": "7", "Error": "boom", "Output": "trace"}`
				_ = os.WriteFile(p.ProvisionJSONFile, []byte(failJSON), 0644)
				_, _ = os.Create(p.ProvisionCompleteFile)
			},
		},
		{
			name:      "timeout waiting for completion",
			wantsErr:  true,
			errString: "context deadline exceeded waiting for provision complete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := &MockCmdRunner{}
			tempDir, err := os.MkdirTemp("", "provisiontest")
			assert.NoError(t, err)
			tempFile := filepath.Join(tempDir, "testfile.txt")
			completeFile := filepath.Join(tempDir, "provision.complete")
			defer os.RemoveAll(tempDir)

			p := ProvisionStatusFiles{ProvisionJSONFile: tempFile, ProvisionCompleteFile: completeFile}
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()
			app := &App{cmdRunner: mc.Run}
			if tt.setup != nil {
				tt.setup(p)
			}

			data, err := app.ProvisionWait(ctx, p)
			if tt.wantsErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errString)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testData, data)
			}
		})
	}
}
func TestApp_Run_Integration(t *testing.T) {
	t.Run("success case", func(t *testing.T) {
		mc := &MockCmdRunner{}
		app := &App{cmdRunner: mc.Run}
		// Use a valid provision config file from testdata
		exitCode := app.Run(context.Background(), []string{"aks-node-controller", "provision", "--provision-config=parser/testdata/test_aksnodeconfig.json"})
		assert.Equal(t, 0, exitCode)
	})

	t.Run("failure case - unknown command", func(t *testing.T) {
		mc := &MockCmdRunner{}
		app := &App{cmdRunner: mc.Run}
		exitCode := app.Run(context.Background(), []string{"aks-node-controller", "unknown"})
		assert.Equal(t, 1, exitCode)
	})

	t.Run("failure case - missing command argument", func(t *testing.T) {
		mc := &MockCmdRunner{}
		app := &App{cmdRunner: mc.Run}
		exitCode := app.Run(context.Background(), []string{"aks-node-controller"})
		assert.Equal(t, 1, exitCode)
	})

	t.Run("failure case - command runner returns ExitError", func(t *testing.T) {
		mc := &MockCmdRunner{
			RunFunc: func(cmd *exec.Cmd) error {
				return &ExitError{Code: 42}
			},
		}
		app := &App{cmdRunner: mc.Run}
		exitCode := app.Run(context.Background(), []string{"aks-node-controller", "provision", "--provision-config=parser/testdata/test_aksnodeconfig.json"})
		assert.Equal(t, 42, exitCode)
	})

	t.Run("failure case - command runner returns generic error", func(t *testing.T) {
		mc := &MockCmdRunner{
			RunFunc: func(cmd *exec.Cmd) error {
				return errors.New("generic error")
			},
		}
		app := &App{cmdRunner: mc.Run}
		exitCode := app.Run(context.Background(), []string{"aks-node-controller", "provision", "--provision-config=parser/testdata/test_aksnodeconfig.json"})
		assert.Equal(t, 1, exitCode)
	})
}

func Test_readAndEvaluateProvision(t *testing.T) {
	// valid provision file with exit code 0
	t.Run("valid provision file", func(t *testing.T) {
		f, err := os.CreateTemp(t.TempDir(), "provision_valid_*.json")
		if err != nil {
			t.Fatalf("temp file create: %v", err)
		}
		content := `{"ExitCode":"0","Output":"ok","Error":"","ExecDuration":"1"}`
		if _, err := f.WriteString(content); err != nil {
			t.Fatalf("write temp provision: %v", err)
		}
		f.Close()
		got, gotErr := readAndEvaluateProvision(f.Name())
		if gotErr != nil {
			t.Fatalf("unexpected error: %v", gotErr)
		}
		if got == "" || !strings.Contains(got, "\"ExitCode\":\"0\"") {
			t.Errorf("content mismatch; got=%s", got)
		}
	})

	// missing file
	t.Run("missing provision file", func(t *testing.T) {
		_, err := readAndEvaluateProvision(filepath.Join(t.TempDir(), "does_not_exist.json"))
		if err == nil {
			t.Fatalf("expected error for missing file")
		}
	})

	// invalid JSON
	t.Run("invalid provision file (bad JSON)", func(t *testing.T) {
		f, err := os.CreateTemp(t.TempDir(), "provision_invalid_*.json")
		if err != nil {
			t.Fatalf("temp file create: %v", err)
		}
		if _, err := f.WriteString("not-json"); err != nil {
			t.Fatalf("write: %v", err)
		}
		f.Close()
		_, gotErr := readAndEvaluateProvision(f.Name())
		if gotErr == nil {
			t.Fatalf("expected parse error, got nil")
		}
	})

	// valid provision file with non-zero exit code
	t.Run("non-zero ExitCode returns error", func(t *testing.T) {
		f, err := os.CreateTemp(t.TempDir(), "provision_fail_*.json")
		if err != nil {
			t.Fatalf("temp file create: %v", err)
		}
		if _, err := f.WriteString(`{"ExitCode":"7","Output":"boom","Error":"bad"}`); err != nil {
			t.Fatalf("write: %v", err)
		}
		f.Close()
		_, gotErr := readAndEvaluateProvision(f.Name())
		if gotErr == nil || !strings.Contains(gotErr.Error(), "provision failed") {
			t.Fatalf("expected failure error, got=%v", gotErr)
		}
	})
}
