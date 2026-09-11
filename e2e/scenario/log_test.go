package scenario

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestCollectCommandLogsConcurrency(t *testing.T) {
	original := config.Config.E2ELoggingDir
	config.Config.E2ELoggingDir = t.TempDir()
	t.Cleanup(func() { config.Config.E2ELoggingDir = original })
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	commands := map[string]string{}
	for i := range 2 * logCollectionConcurrency {
		commands[fmt.Sprintf("%d.log", i)] = fmt.Sprint(i)
	}
	started := make(chan struct{}, len(commands))
	release := make(chan struct{})
	var active atomic.Int32
	done := make(chan error, 1)
	go func() {
		done <- collectCommandLogs(ctx, "batch", commands, func(ctx context.Context, command string) (*podExecResult, error) {
			if active.Add(1) > logCollectionConcurrency {
				t.Error("SSH concurrency limit exceeded")
			}
			defer active.Add(-1)
			started <- struct{}{}
			select {
			case <-release:
				return &podExecResult{exitCode: "0", stdout: command}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		})
	}()
	for range logCollectionConcurrency {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("log collection did not start concurrently")
		}
	}
	select {
	case err := <-done:
		t.Fatalf("collection returned before commands finished: %v", err)
	default:
	}
	close(release)
	require.NoError(t, <-done)
	for file, command := range commands {
		content, err := os.ReadFile(filepath.Join(artifactDir("batch"), file))
		require.NoError(t, err)
		assert.Equal(t, (&podExecResult{exitCode: "0", stdout: command}).String(), string(content))
	}
}

func TestCollectCommandLogsErrorArtifacts(t *testing.T) {
	original := config.Config.E2ELoggingDir
	config.Config.E2ELoggingDir = t.TempDir()
	t.Cleanup(func() { config.Config.E2ELoggingDir = original })
	detail := "SSH transport failed with detailed diagnostics"
	err := collectCommandLogs(t.Context(), "batch", map[string]string{
		"error.log": "error", "panic.log": "panic", "missing.log": "missing",
	}, func(_ context.Context, command string) (*podExecResult, error) {
		switch command {
		case "error":
			return nil, errors.New(detail)
		case "panic":
			panic(detail)
		default:
			return &podExecResult{exitCode: "1", stderr: "optional file missing"}, nil
		}
	})
	require.ErrorContains(t, err, "error.log")
	assert.ErrorContains(t, err, "panic.log")
	assert.NotContains(t, err.Error(), detail)
	assert.NotContains(t, err.Error(), "missing.log")
	for file, expected := range map[string]string{
		"error.log": detail, "panic.log": detail, "missing.log": "optional file missing",
	} {
		content, err := os.ReadFile(filepath.Join(artifactDir("batch"), file))
		require.NoError(t, err)
		assert.Contains(t, string(content), expected)
	}
}

func TestCollectCommandLogsCanceledContextAndWriteFailure(t *testing.T) {
	original := config.Config.E2ELoggingDir
	config.Config.E2ELoggingDir = t.TempDir()
	t.Cleanup(func() { config.Config.E2ELoggingDir = original })
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	exec := func(context.Context, string) (*podExecResult, error) {
		t.Error("command started after cancellation")
		return nil, errors.New("unexpected execution")
	}
	commands := map[string]string{"error.log": "command"}
	require.ErrorContains(t, collectCommandLogs(ctx, "batch", commands, exec), "could not collect error.log")
	content, err := os.ReadFile(filepath.Join(artifactDir("batch"), "error.log"))
	require.NoError(t, err)
	assert.Contains(t, string(content), context.Canceled.Error())

	require.NoError(t, os.WriteFile(filepath.Join(config.Config.E2ELoggingDir, "blocked"), nil, 0600))
	require.ErrorContains(t, collectCommandLogs(ctx, "blocked", commands, exec), "write log error.log")
}

func TestCollectCommandLogsRetriesRejectedSessions(t *testing.T) {
	original := config.Config.E2ELoggingDir
	config.Config.E2ELoggingDir = t.TempDir()
	t.Cleanup(func() { config.Config.E2ELoggingDir = original })
	for _, test := range []struct {
		name     string
		failures int
		reason   ssh.RejectionReason
		wantRuns int
		wantErr  bool
		cancel   bool
	}{
		{"recovers", 1, ssh.ConnectionFailed, 2, false, false},
		{"exhausted", 10, ssh.ConnectionFailed, 5, true, false},
		{"denied", 1, ssh.Prohibited, 1, true, false},
		{"canceled", 1, ssh.ConnectionFailed, 1, true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			runs := 0
			err := collectCommandLogs(ctx, test.name, map[string]string{"test.log": "command"},
				func(context.Context, string) (*podExecResult, error) {
					runs++
					if test.cancel {
						cancel()
					}
					if runs <= test.failures {
						return nil, &ssh.OpenChannelError{Reason: test.reason, Message: "open failed"}
					}
					return &podExecResult{exitCode: "0", stdout: "collected"}, nil
				})
			assert.Equal(t, test.wantRuns, runs)
			content, readErr := os.ReadFile(filepath.Join(artifactDir(test.name), "test.log"))
			require.NoError(t, readErr)
			if test.wantErr {
				require.Error(t, err)
				assert.Contains(t, string(content), "open failed")
			} else {
				require.NoError(t, err)
				assert.Contains(t, string(content), "collected")
			}
		})
	}
}

func TestArtifactHelpersUseTheGivenArtifactName(t *testing.T) {
	loggingDir := t.TempDir()
	original := config.Config.E2ELoggingDir
	config.Config.E2ELoggingDir = loggingDir
	t.Cleanup(func() { config.Config.E2ELoggingDir = original })

	const artifactName = "TestScenario/vhd-provision"

	assert.Equal(t, filepath.Join(loggingDir, artifactName), artifactDir(artifactName))

	require.NoError(t, writeToFile(artifactName, "single.log", "single"))
	require.NoError(t, dumpFileMapToDir(artifactName, map[string]string{"/var/log/nested/mapped.log": "mapped"}))

	for name, want := range map[string]string{"single.log": "single", "mapped.log": "mapped"} {
		got, err := os.ReadFile(filepath.Join(artifactDir(artifactName), name))
		require.NoError(t, err, "read %s", name)
		assert.Equal(t, want, string(got), name)
	}
}

func TestGenerateVMSSNameLinuxUsesTheGivenArtifactName(t *testing.T) {
	name := generateVMSSNameLinux("TestScenario_Ubuntu2204/vhd_caching")

	assert.LessOrEqual(t, len(name), 57, "name %q exceeds the VMSS limit", name)
	assert.NotContains(t, name, "_")
	assert.NotContains(t, name, "/")
	assert.NotContains(t, name, "Test")
	assert.Equal(t, strings.ToLower(name), name, "name is not lowercase")
	assert.Contains(t, name, "scenarioubuntu2204", "name does not carry the test name")
}
