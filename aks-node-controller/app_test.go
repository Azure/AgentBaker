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
			cmdRun:      runFunc,
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

	t.Run("--version flag returns success exit code", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		exitCode := tt.App.Run(context.Background(), []string{"aks-node-controller", "--version"})
		assert.Equal(t, 0, exitCode)
	})

	t.Run("version command returns success exit code", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		exitCode := tt.App.Run(context.Background(), []string{"aks-node-controller", "version"})
		assert.Equal(t, 0, exitCode)
	})

	t.Run("--help flag returns success exit code", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		exitCode := tt.App.Run(context.Background(), []string{"aks-node-controller", "--help"})
		assert.Equal(t, 0, exitCode)
	})

	t.Run("help command returns success exit code", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		exitCode := tt.App.Run(context.Background(), []string{"aks-node-controller", "help"})
		assert.Equal(t, 0, exitCode)
	})

	t.Run("download-hotfix rejects unexpected arguments", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		exitCode := tt.App.Run(context.Background(), []string{"aks-node-controller", "download-hotfix", "extra"})
		assert.Equal(t, 1, exitCode)
	})

	t.Run("download-hotfix returns success when VHD version already matches target", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		origVersion := Version
		Version = "202604.01.1"
		defer func() { Version = origVersion }()

		configPath := filepath.Join(t.TempDir(), "hotfix-config.json")
		require.NoError(t, os.WriteFile(configPath, []byte(`{"version": "202604.01.1"}`), 0o644))
		tt.App.hotfixVersionPath = configPath

		exitCode := tt.App.Run(context.Background(), []string{"aks-node-controller", "download-hotfix"})
		assert.Equal(t, 0, exitCode)
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

	t.Run("provision command with provision-config and nbc-cmd flag", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		params := []string{"aks-node-controller", "provision", "--nbc-cmd=parser/testdata/test_nbccmd.sh"}
		exitCode := tt.App.Run(context.Background(), params)
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

	t.Run("nbc cmd is executed by passing the script path to bash", func(t *testing.T) {
		scriptPath := filepath.Join(t.TempDir(), "test_nbccmd.sh")
		require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/bash\necho success running nbc_cmd.sh\n"), 0o600))

		var gotCmd *exec.Cmd
		tt := NewTestApp(t, TestAppConfig{
			RunFunc: func(cmd *exec.Cmd) error {
				gotCmd = cmd
				return cmdRunner(cmd)
			},
		})

		result, err := tt.App.Provision(context.Background(), ProvisionFlags{NBCCmd: scriptPath})
		require.NoError(t, err)
		require.NotNil(t, gotCmd)
		assert.Equal(t, "/bin/bash", gotCmd.Path)
		assert.Equal(t, []string{"/bin/bash", "--", scriptPath}, gotCmd.Args)
		assert.Equal(t, "0", result.ExitCode)
		assert.Contains(t, result.Output, "success running nbc_cmd.sh")
	})

	t.Run("nbc cmd is always passed as a script path, even when it starts with a dash", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Chdir(tempDir)
		scriptPath := "-test_nbccmd.sh"
		require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/bash\necho success running dashed nbc_cmd.sh\n"), 0o600))

		var gotCmd *exec.Cmd
		tt := NewTestApp(t, TestAppConfig{
			RunFunc: func(cmd *exec.Cmd) error {
				gotCmd = cmd
				return cmdRunner(cmd)
			},
		})

		result, err := tt.App.Provision(context.Background(), ProvisionFlags{NBCCmd: scriptPath})
		require.NoError(t, err)
		require.NotNil(t, gotCmd)
		assert.Equal(t, []string{"/bin/bash", "--", scriptPath}, gotCmd.Args)
		assert.Equal(t, "0", result.ExitCode)
		assert.Contains(t, result.Output, "success running dashed nbc_cmd.sh")
	})

	t.Run("nbc cmd file read errors are wrapped", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		scriptPath := filepath.Join(t.TempDir(), "missing.sh")

		result, err := tt.App.Provision(context.Background(), ProvisionFlags{NBCCmd: scriptPath})
		require.Error(t, err)
		assert.ErrorIs(t, err, os.ErrNotExist)
		assert.Equal(t, "240", result.ExitCode)
		assert.Contains(t, result.Error, "read NBC command file "+scriptPath)
	})
}

func TestApp_Provision_DryRun(t *testing.T) {
	tt := NewTestApp(t, TestAppConfig{})
	tt.App.cmdRun = cmdRunner // Use real cmdRunner to test dry-run override
	exitCode := tt.App.Run(context.Background(), []string{"aks-node-controller", "provision", "--provision-config=parser/testdata/test_aksnodeconfig.json", "--dry-run"})
	assert.Equal(t, 0, exitCode)
	if reflect.ValueOf(tt.App.cmdRun).Pointer() != reflect.ValueOf(cmdRunnerDryRun).Pointer() {
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

func TestParseEnvVarsFromNBCCmdContent(t *testing.T) {
	t.Run("simple unquoted vars", func(t *testing.T) {
		content := `ADMINUSER=azureuser KUBERNETES_VERSION=1.33.7 MOBY_VERSION=`
		got := parseEnvVarsFromNBCCmdContent(content)
		assert.Equal(t, "azureuser", got["ADMINUSER"])
		assert.Equal(t, "1.33.7", got["KUBERNETES_VERSION"])
		assert.Equal(t, "", got["MOBY_VERSION"])
	})

	t.Run("quoted values", func(t *testing.T) {
		content := `ENABLE_MANAGED_GPU="false" DISABLE_SSH="false"`
		got := parseEnvVarsFromNBCCmdContent(content)
		assert.Equal(t, "false", got["ENABLE_MANAGED_GPU"])
		assert.Equal(t, "false", got["DISABLE_SSH"])
	})

	t.Run("url values", func(t *testing.T) {
		content := `KUBE_BINARY_URL=https://packages.aks.azure.com/kubernetes/v1.33.7/binaries/kubernetes-node-linux-amd64.tar.gz`
		got := parseEnvVarsFromNBCCmdContent(content)
		assert.Equal(t, "https://packages.aks.azure.com/kubernetes/v1.33.7/binaries/kubernetes-node-linux-amd64.tar.gz", got["KUBE_BINARY_URL"])
	})

	t.Run("skips shell commands", func(t *testing.T) {
		content := `echo hello; ADMINUSER=azureuser /usr/bin/nohup /bin/bash -c "script"`
		got := parseEnvVarsFromNBCCmdContent(content)
		assert.Equal(t, "azureuser", got["ADMINUSER"])
		assert.NotContains(t, got, "echo")
	})

	t.Run("handles missing space between assignments", func(t *testing.T) {
		content := `KUBELET_CONFIG_FILE_ENABLED="true"PRE_PROVISION_ONLY="false"`
		got := parseEnvVarsFromNBCCmdContent(content)
		assert.Equal(t, "true", got["KUBELET_CONFIG_FILE_ENABLED"])
		assert.Equal(t, "false", got["PRE_PROVISION_ONLY"])
	})

	t.Run("semicolons as delimiters", func(t *testing.T) {
		content := `PROVISION_OUTPUT="/var/log/azure/cluster-provision-cse-output.log"; echo foo; ADMINUSER=azureuser`
		got := parseEnvVarsFromNBCCmdContent(content)
		assert.Equal(t, "/var/log/azure/cluster-provision-cse-output.log", got["PROVISION_OUTPUT"])
		assert.Equal(t, "azureuser", got["ADMINUSER"])
	})

	t.Run("real nbc command snippet", func(t *testing.T) {
		content := `PROVISION_OUTPUT="/var/log/azure/cluster-provision-cse-output.log"; echo $(date),$(hostname) > ${PROVISION_OUTPUT}; ADMINUSER=azureuser MOBY_VERSION= TENANT_ID=72f988bf-86f1-41af-91ab-2d7cd011db47 KUBERNETES_VERSION=1.33.7 KUBE_BINARY_URL=https://packages.aks.azure.com/kubernetes/v1.33.7/binaries/kubernetes-node-linux-amd64.tar.gz VM_TYPE=vmss NETWORK_PLUGIN=kubenet ENABLE_MANAGED_GPU="false" GPU_NEEDS_FABRIC_MANAGER="false" CSE_TIMEOUT="900" /usr/bin/nohup /bin/bash -c "/bin/bash /opt/azure/containers/provision_start.sh"`
		got := parseEnvVarsFromNBCCmdContent(content)
		assert.Equal(t, "/var/log/azure/cluster-provision-cse-output.log", got["PROVISION_OUTPUT"])
		assert.Equal(t, "azureuser", got["ADMINUSER"])
		assert.Equal(t, "", got["MOBY_VERSION"])
		assert.Equal(t, "72f988bf-86f1-41af-91ab-2d7cd011db47", got["TENANT_ID"])
		assert.Equal(t, "1.33.7", got["KUBERNETES_VERSION"])
		assert.Equal(t, "vmss", got["VM_TYPE"])
		assert.Equal(t, "kubenet", got["NETWORK_PLUGIN"])
		assert.Equal(t, "false", got["ENABLE_MANAGED_GPU"])
		assert.Equal(t, "false", got["GPU_NEEDS_FABRIC_MANAGER"])
		assert.Equal(t, "900", got["CSE_TIMEOUT"])
	})
}
