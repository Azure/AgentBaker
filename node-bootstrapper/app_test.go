package main

import (
	"context"
	"errors"
	"os/exec"
	"strconv"
	"testing"

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
			args:     []string{"node-bootstrapper"},
			wantExit: 1,
		},
		{
			name:     "unknown command",
			args:     []string{"node-bootstrapper", "unknown"},
			wantExit: 1,
		},
		{
			name:     "provision command with missing flag",
			args:     []string{"provision"},
			wantExit: 1,
		},
		{
			name: "provision command with valid flag",
			args: []string{"node-bootstrapper", "provision", "--provision-config=parser/testdata/test_nbc.json"},
			setupMocks: func(mc *MockCmdRunner) {
				mc.RunFunc = func(cmd *exec.Cmd) error {
					return nil
				}
			},
			wantExit: 0,
		},
		{
			name: "provision command with command runner error",
			args: []string{"node-bootstrapper", "provision", "--provision-config=parser/testdata/test_nbc.json"},
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
			flags:   ProvisionFlags{ProvisionConfig: "parser/testdata/test_nbc.json"},
			wantErr: false,
		},
		{
			name:    "invalid provision config path",
			flags:   ProvisionFlags{ProvisionConfig: "invalid.json"},
			wantErr: true,
		},
		{
			name:  "command runner error",
			flags: ProvisionFlags{ProvisionConfig: "parser/testdata/test_nbc.json"},
			setupMocks: func(mc *MockCmdRunner) {
				mc.RunFunc = func(cmd *exec.Cmd) error {
					return errors.New("command runner error")
				}
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

			app := &App{
				cmdRunner: mc.Run,
			}

			err := app.Provision(context.Background(), tt.flags)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
