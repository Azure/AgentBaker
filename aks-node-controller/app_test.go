package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Azure/agentbaker/aks-node-controller/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// logRecord holds a captured slog record for test assertions.
type logRecord struct {
	Level   slog.Level
	Message string
	Attrs   map[string]string
}

// logCapturer is a slog.Handler that captures log records for test verification.
type logCapturer struct {
	mu      sync.Mutex
	records []logRecord
}

func (c *logCapturer) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (c *logCapturer) Handle(_ context.Context, r slog.Record) error {
	rec := logRecord{
		Level:   r.Level,
		Message: r.Message,
		Attrs:   make(map[string]string),
	}
	r.Attrs(func(a slog.Attr) bool {
		rec.Attrs[a.Key] = a.Value.String()
		return true
	})
	c.mu.Lock()
	c.records = append(c.records, rec)
	c.mu.Unlock()
	return nil
}

func (c *logCapturer) WithAttrs(_ []slog.Attr) slog.Handler { return c }
func (c *logCapturer) WithGroup(_ string) slog.Handler      { return c }

func (c *logCapturer) getRecords() []logRecord {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]logRecord, len(c.records))
	copy(out, c.records)
	return out
}

// installLogCapturer replaces the default slog logger with a capturing handler
// and returns the capturer. It restores the original logger when the test ends.
func installLogCapturer(t *testing.T) *logCapturer {
	t.Helper()
	logCap := &logCapturer{}
	orig := slog.Default()
	slog.SetDefault(slog.New(logCap))
	t.Cleanup(func() { slog.SetDefault(orig) })
	return logCap
}

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

	t.Run("compareEnvs failure does not block nbc-cmd provisioning", func(t *testing.T) {
		// Use an invalid provision-config path so compareEnvs will fail internally.
		// Provisioning via nbc-cmd should still succeed.
		scriptPath := filepath.Join(t.TempDir(), "test_nbccmd.sh")
		require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/bash\necho provisioned\n"), 0o600))

		tt := NewTestApp(t, TestAppConfig{
			RunFunc: cmdRunner,
		})
		result, err := tt.App.Provision(context.Background(), ProvisionFlags{
			ProvisionConfig: "/nonexistent/invalid_config.json",
			NBCCmd:          scriptPath,
		})
		require.NoError(t, err)
		assert.Equal(t, "0", result.ExitCode)
		assert.Contains(t, result.Output, "provisioned")
	})

	t.Run("compareEnvs panic does not block nbc-cmd provisioning", func(t *testing.T) {
		// Use a nil eventLogger so that compareEnvs panics with a nil pointer
		// dereference when it calls eventLogger.LogEvent after parsing succeeds.
		// The defer/recover inside compareEnvs must catch this and allow
		// provisioning to proceed normally.
		scriptPath := filepath.Join(t.TempDir(), "test_nbccmd.sh")
		require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/bash\necho provisioned after panic\n"), 0o600))

		app := &App{
			cmdRun:      cmdRunner,
			eventLogger: nil, // nil to trigger panic inside compareEnvs
		}
		result, err := app.Provision(context.Background(), ProvisionFlags{
			ProvisionConfig: "parser/testdata/test_aksnodeconfig.json",
			NBCCmd:          scriptPath,
		})
		require.NoError(t, err)
		assert.Equal(t, "0", result.ExitCode)
		assert.Contains(t, result.Output, "provisioned after panic")
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

func TestCompareEnvs(t *testing.T) {
	// Build a cmd from the test provision config to discover which CSE env vars it produces.
	// We filter out OS env vars (same as compareEnvs does) to get the "expected" env set.
	buildConfigEnv := func(t *testing.T) map[string]string {
		t.Helper()
		cmd, err := buildCmdFromProvisionConfig(context.Background(), "parser/testdata/test_aksnodeconfig.json")
		require.NoError(t, err)
		osEnv := envSliceToMap(os.Environ())
		allEnv := envSliceToMap(cmd.Env)
		configEnv := make(map[string]string, len(allEnv))
		for k, v := range allEnv {
			if osVal, inOS := osEnv[k]; !inOS || osVal != v {
				configEnv[k] = v
			}
		}
		return configEnv
	}

	// writeNBCCmd writes an NBC command script to a temp file and returns its path.
	writeNBCCmd := func(t *testing.T, content string) string {
		t.Helper()
		p := filepath.Join(t.TempDir(), "nbc_cmd.sh")
		require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
		return p
	}

	t.Run("matching env vars produce no diff logs", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		logCap := installLogCapturer(t)
		configEnv := buildConfigEnv(t)

		// Build an NBC cmd that has the same env vars as the config.
		var parts []string
		for k, v := range configEnv {
			if v == "" {
				parts = append(parts, k+"=")
			} else {
				parts = append(parts, k+"=\""+v+"\"")
			}
		}
		nbcContent := strings.Join(parts, " ")
		nbcPath := writeNBCCmd(t, nbcContent)

		compareEnvs(context.Background(), ProvisionFlags{
			ProvisionConfig: "parser/testdata/test_aksnodeconfig.json",
			NBCCmd:          nbcPath,
		}, tt.eventLogger)

		records := logCap.getRecords()
		var foundNoOp bool
		for _, r := range records {
			if strings.Contains(r.Message, "env compare: no differences found") {
				foundNoOp = true
			}
			assert.NotContains(t, r.Message, "env var differences", "expected no differences logged")
		}
		assert.True(t, foundNoOp, "expected 'no differences' log message")

		// Verify guest agent event was emitted with match message.
		events := tt.eventLogger.Events()
		require.NotEmpty(t, events)
		var found bool
		for _, e := range events {
			if strings.Contains(e.TaskName, "CompareEnvs") {
				assert.Contains(t, e.Message, "env vars match")
				found = true
			}
		}
		assert.True(t, found, "expected CompareEnvs guest agent event")
	})

	t.Run("var only in provision-config is logged", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		logCap := installLogCapturer(t)
		configEnv := buildConfigEnv(t)

		// Build NBC cmd with all config vars except ADMINUSER.
		var parts []string
		for k, v := range configEnv {
			if k == "ADMINUSER" {
				continue
			}
			if v == "" {
				parts = append(parts, k+"=")
			} else {
				parts = append(parts, k+"=\""+v+"\"")
			}
		}
		nbcContent := strings.Join(parts, " ")
		nbcPath := writeNBCCmd(t, nbcContent)

		compareEnvs(context.Background(), ProvisionFlags{
			ProvisionConfig: "parser/testdata/test_aksnodeconfig.json",
			NBCCmd:          nbcPath,
		}, tt.eventLogger)

		records := logCap.getRecords()
		var foundDiff bool
		for _, r := range records {
			if strings.Contains(r.Message, "only-in-pc: ADMINUSER") {
				foundDiff = true
			}
		}
		assert.True(t, foundDiff, "expected summary log containing 'only-in-pc: ADMINUSER'")

		// Verify guest agent event contains the diff.
		events := tt.eventLogger.Events()
		var found bool
		for _, e := range events {
			if strings.Contains(e.TaskName, "CompareEnvs") {
				assert.Contains(t, e.Message, "only-in-pc: ADMINUSER")
				found = true
			}
		}
		assert.True(t, found, "expected CompareEnvs guest agent event")
	})

	t.Run("var only in nbc-cmd is logged", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		logCap := installLogCapturer(t)
		configEnv := buildConfigEnv(t)

		// Build NBC cmd with all config vars plus an extra var.
		var parts []string
		for k, v := range configEnv {
			if v == "" {
				parts = append(parts, k+"=")
			} else {
				parts = append(parts, k+"=\""+v+"\"")
			}
		}
		parts = append(parts, `EXTRA_NBC_ONLY="extra_value"`)
		nbcContent := strings.Join(parts, " ")
		nbcPath := writeNBCCmd(t, nbcContent)

		compareEnvs(context.Background(), ProvisionFlags{
			ProvisionConfig: "parser/testdata/test_aksnodeconfig.json",
			NBCCmd:          nbcPath,
		}, tt.eventLogger)

		records := logCap.getRecords()
		var foundDiff bool
		for _, r := range records {
			if strings.Contains(r.Message, "only-in-nbc: EXTRA_NBC_ONLY") {
				foundDiff = true
			}
		}
		assert.True(t, foundDiff, "expected summary log containing 'only-in-nbc: EXTRA_NBC_ONLY'")

		// Verify guest agent event contains the diff.
		events := tt.eventLogger.Events()
		var found bool
		for _, e := range events {
			if strings.Contains(e.TaskName, "CompareEnvs") {
				assert.Contains(t, e.Message, "only-in-nbc: EXTRA_NBC_ONLY")
				found = true
			}
		}
		assert.True(t, found, "expected CompareEnvs guest agent event")
	})

	t.Run("differing values are logged", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		logCap := installLogCapturer(t)
		configEnv := buildConfigEnv(t)

		// Build NBC cmd with all config vars but change VM_TYPE value.
		var parts []string
		for k, v := range configEnv {
			if k == "VM_TYPE" {
				parts = append(parts, k+"=standard")
				continue
			}
			if v == "" {
				parts = append(parts, k+"=")
			} else {
				parts = append(parts, k+"=\""+v+"\"")
			}
		}
		nbcContent := strings.Join(parts, " ")
		nbcPath := writeNBCCmd(t, nbcContent)

		compareEnvs(context.Background(), ProvisionFlags{
			ProvisionConfig: "parser/testdata/test_aksnodeconfig.json",
			NBCCmd:          nbcPath,
		}, tt.eventLogger)

		records := logCap.getRecords()
		var foundDiff bool
		for _, r := range records {
			if strings.Contains(r.Message, "differs: VM_TYPE") {
				foundDiff = true
			}
		}
		assert.True(t, foundDiff, "expected summary log containing 'differs: VM_TYPE'")

		// Verify guest agent event contains the diff.
		events := tt.eventLogger.Events()
		var found bool
		for _, e := range events {
			if strings.Contains(e.TaskName, "CompareEnvs") {
				assert.Contains(t, e.Message, "differs: VM_TYPE")
				found = true
			}
		}
		assert.True(t, found, "expected CompareEnvs guest agent event")
	})

	t.Run("multiple differences are all logged", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		logCap := installLogCapturer(t)
		configEnv := buildConfigEnv(t)

		// NBC cmd: remove ADMINUSER (only in config), add EXTRA_VAR (only in NBC), change VM_TYPE (differs).
		var parts []string
		for k, v := range configEnv {
			if k == "ADMINUSER" {
				continue
			}
			if k == "VM_TYPE" {
				parts = append(parts, k+"=changed")
				continue
			}
			if v == "" {
				parts = append(parts, k+"=")
			} else {
				parts = append(parts, k+"=\""+v+"\"")
			}
		}
		parts = append(parts, `EXTRA_VAR="new"`)
		nbcContent := strings.Join(parts, " ")
		nbcPath := writeNBCCmd(t, nbcContent)

		compareEnvs(context.Background(), ProvisionFlags{
			ProvisionConfig: "parser/testdata/test_aksnodeconfig.json",
			NBCCmd:          nbcPath,
		}, tt.eventLogger)

		records := logCap.getRecords()
		var foundSummary bool
		for _, r := range records {
			if strings.Contains(r.Message, "env var differences (3)") {
				foundSummary = true
				assert.Contains(t, r.Message, "only-in-pc: ADMINUSER")
				assert.Contains(t, r.Message, "only-in-nbc: EXTRA_VAR")
				assert.Contains(t, r.Message, "differs: VM_TYPE")
			}
		}
		assert.True(t, foundSummary, "expected summary log with all 3 differences")

		// Verify guest agent event has all 3 diffs.
		events := tt.eventLogger.Events()
		var found bool
		for _, e := range events {
			if strings.Contains(e.TaskName, "CompareEnvs") {
				assert.Contains(t, e.Message, "env var differences (3)")
				assert.Contains(t, e.Message, "only-in-pc: ADMINUSER")
				assert.Contains(t, e.Message, "only-in-nbc: EXTRA_VAR")
				assert.Contains(t, e.Message, "differs: VM_TYPE")
				found = true
			}
		}
		assert.True(t, found, "expected CompareEnvs guest agent event")
	})
}
