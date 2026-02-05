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
			_, err := app.Provision(context.Background(), tt.flags)
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
			name: "event path (file created after call)",
			setup: func(provisionStatusFiles ProvisionStatusFiles) {
				// This goroutine simulates an external process writing the files after a short delay.
				// It's running asynchronously from the main test flow.
				go func() {
					time.Sleep(150 * time.Millisecond)
					_ = os.WriteFile(provisionStatusFiles.ProvisionJSONFile, []byte(testData), 0644)
					_, _ = os.Create(provisionStatusFiles.ProvisionCompleteFile)
				}()
			},
		},
		{
			name: "fast path (file exists before call)",
			setup: func(provisionStatusFiles ProvisionStatusFiles) {
				_ = os.WriteFile(provisionStatusFiles.ProvisionJSONFile, []byte(testData), 0644)
				_, _ = os.Create(provisionStatusFiles.ProvisionCompleteFile) // pre-create to trigger immediate return
			},
		},
		{
			name:      "provision completion with failure ExitCode",
			wantsErr:  true,
			errString: "provision failed",
			setup: func(provisionStatusFiles ProvisionStatusFiles) {
				failJSON := `{"ExitCode": "7", "Error": "boom", "Output": "trace"}`
				_ = os.WriteFile(provisionStatusFiles.ProvisionJSONFile, []byte(failJSON), 0644)
				_, _ = os.Create(provisionStatusFiles.ProvisionCompleteFile)
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
	type testCase struct {
		name           string
		fileContent    string // raw content to place in file (empty => file absent)
		createFile     bool   // whether to create the file
		expectErrSub   string // substring that should appear in error; empty means success expected
		expectContains string // substring that must appear in successful content
	}

	tests := []testCase{
		{
			name:           "valid provision file",
			createFile:     true,
			fileContent:    `{"ExitCode":"0","Output":"ok","Error":"","ExecDuration":"1"}`,
			expectContains: `"ExitCode":"0"`,
		},
		{
			name:         "missing provision file",
			createFile:   false,
			expectErrSub: "no such file",
		},
		{
			name:         "invalid provision file (bad JSON)",
			createFile:   true,
			fileContent:  `not-json`,
			expectErrSub: "invalid character",
		},
		{
			name:         "non-zero ExitCode returns error",
			createFile:   true,
			fileContent:  `{"ExitCode":"7","Output":"boom","Error":"bad"}`,
			expectErrSub: "provision failed",
		},
		{
			name:         "invalid ExitCode returns error",
			createFile:   true,
			fileContent:  `{"ExitCode":"unknown","Output":"boom","Error":"bad"}`,
			expectErrSub: "invalid ExitCode",
		},
		{
			name:         "missing ExitCode returns error",
			createFile:   true,
			fileContent:  `{"Output":"boom","Error":"bad"}`,
			expectErrSub: "missing ExitCode",
		},
	}

	writeTemp := func(t *testing.T, content string) string {
		t.Helper()
		f, err := os.CreateTemp(t.TempDir(), "provision_*.json")
		assert.NoError(t, err)
		_, errWS := f.WriteString(content)
		assert.NoError(t, errWS)
		f.Close()
		return f.Name()
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "does_not_exist.json")
			if tc.createFile {
				p = writeTemp(t, tc.fileContent)
			}
			got, err := readAndEvaluateProvision(p)
			if tc.expectErrSub != "" { // expected error
				assert.Error(t, err, "expected an error")
				if err != nil { // avoid panic if err is nil
					assert.Contains(t, err.Error(), tc.expectErrSub, "error should contain substring")
				}
			} else { // success
				assert.NoError(t, err, "unexpected error")
				if tc.expectContains != "" {
					assert.Contains(t, got, tc.expectContains, "content should contain substring")
				}
			}
		})
	}
}
