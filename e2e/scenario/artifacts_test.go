package scenario

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectCommandLogsConcurrency(t *testing.T) {
	original := config.Config.E2ELoggingDir
	config.Config.E2ELoggingDir = t.TempDir()
	t.Cleanup(func() { config.Config.E2ELoggingDir = original })
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	started := t.TempDir()
	commands := map[string]string{}
	for i := range 16 {
		commands[fmt.Sprintf("%d.log", i)] = fmt.Sprintf(
			`touch '%s/%d'; while [ "$(find '%s' -type f | wc -l)" -lt 16 ]; do sleep 0.01; done; printf %s`,
			started, i, started, strconv.Itoa(i))
	}
	calls := 0
	require.NoError(t, collectCommandLogs(ctx, "batch", commands, func(ctx context.Context, command string) (*podExecResult, error) {
		calls++
		require.NotContains(t, command, "\n")
		remoteCommand, err := copyScriptToRemoteIfRequired(ctx, nil, command, false)
		require.NoError(t, err)
		assert.Equal(t, command, remoteCommand)
		return runLocalLogCommand(ctx, command)
	}))
	assert.Equal(t, 1, calls)
	for i := range 16 {
		content, err := os.ReadFile(filepath.Join(artifactDir("batch"), fmt.Sprintf("%d.log", i)))
		require.NoError(t, err)
		assert.Equal(t, (&podExecResult{exitCode: "0", stdout: strconv.Itoa(i)}).String(), string(content))
	}
}

func runLocalLogCommand(ctx context.Context, command string) (*podExecResult, error) {
	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	cmd.Env = append(os.Environ(), "COPYFILE_DISABLE=1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.Output()
	code := "0"
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code = strconv.Itoa(exitErr.ExitCode())
	} else if err != nil {
		return nil, err
	}
	return &podExecResult{exitCode: code, stdout: string(stdout), stderr: stderr.String()}, nil
}

func TestCollectCommandLogsOutputAndRemoteCleanup(t *testing.T) {
	original := config.Config.E2ELoggingDir
	config.Config.E2ELoggingDir = t.TempDir()
	t.Cleanup(func() { config.Config.E2ELoggingDir = original })
	remoteTemp := t.TempDir()
	t.Setenv("TMPDIR", remoteTemp)
	require.NoError(t, collectCommandLogs(t.Context(), "batch", map[string]string{
		"output.log":  "printf \"it's quoted\\n\"; printf 'error\\n' >&2; exit 7",
		"missing.log": "printf 'optional file missing' >&2; exit 1",
		"binary.log":  "printf 'a\\0b\\n'\nprintf 'last\\n'",
	}, runLocalLogCommand))
	for file, want := range map[string]podExecResult{
		"output.log":  {exitCode: "7", stdout: "it's quoted\n", stderr: "error\n"},
		"missing.log": {exitCode: "1", stderr: "optional file missing"},
		"binary.log":  {exitCode: "0", stdout: "a\x00b\nlast\n"},
	} {
		content, err := os.ReadFile(filepath.Join(artifactDir("batch"), file))
		require.NoError(t, err)
		assert.Equal(t, want.String(), string(content))
	}
	leftovers, err := os.ReadDir(remoteTemp)
	require.NoError(t, err)
	assert.Empty(t, leftovers)
}

func TestCollectCommandLogsTimeoutPreservesPartialOutput(t *testing.T) {
	original := config.Config.E2ELoggingDir
	config.Config.E2ELoggingDir = t.TempDir()
	t.Cleanup(func() { config.Config.E2ELoggingDir = original })
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	err := collectCommandLogs(ctx, "batch", map[string]string{
		"fast.log": "printf complete",
		"slow.log": "printf partial; sleep 10",
	}, runLocalLogCommand)
	require.ErrorContains(t, err, "slow.log")
	assert.NotContains(t, err.Error(), "fast.log")
	content, err := os.ReadFile(filepath.Join(artifactDir("batch"), "slow.log"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "partial")
	assert.Contains(t, string(content), "log collection failed:")
	assert.Contains(t, string(content), "timed out")
	content, err = os.ReadFile(filepath.Join(artifactDir("batch"), "fast.log"))
	require.NoError(t, err)
	assert.Equal(t, (&podExecResult{exitCode: "0", stdout: "complete"}).String(), string(content))
}

func TestCollectCommandLogsErrorArtifacts(t *testing.T) {
	original := config.Config.E2ELoggingDir
	config.Config.E2ELoggingDir = t.TempDir()
	t.Cleanup(func() { config.Config.E2ELoggingDir = original })
	detail := "SSH transport failed with detailed diagnostics"
	for _, name := range []string{"error", "panic", "nil result"} {
		t.Run(name, func(t *testing.T) {
			err := collectCommandLogs(t.Context(), name, map[string]string{"one.log": "one", "two.log": "two"},
				func(context.Context, string) (*podExecResult, error) {
					switch name {
					case "panic":
						panic(detail)
					case "nil result":
						return nil, nil
					default:
						return nil, errors.New(detail)
					}
				})
			require.ErrorContains(t, err, "one.log")
			assert.ErrorContains(t, err, "two.log")
			assert.NotContains(t, err.Error(), detail)
			for _, file := range []string{"one.log", "two.log"} {
				content, err := os.ReadFile(filepath.Join(artifactDir(name), file))
				require.NoError(t, err)
				assert.Contains(t, string(content), "log collection failed:")
				if name != "nil result" {
					assert.Contains(t, string(content), detail)
				}
			}
		})
	}
}

func TestReadLogArchiveRejectsUnexpectedEntries(t *testing.T) {
	for _, name := range []string{"../escape", "0.stdout/extra", "1.stdout", "00.stdout"} {
		t.Run(name, func(t *testing.T) {
			var buffer bytes.Buffer
			compressed := gzip.NewWriter(&buffer)
			archive := tar.NewWriter(compressed)
			require.NoError(t, archive.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: 0}))
			require.NoError(t, archive.Close())
			require.NoError(t, compressed.Close())
			_, err := readLogArchive(buffer.String(), 1)
			require.ErrorContains(t, err, "unexpected diagnostic archive entry")
		})
	}
}

func TestCollectCommandLogsArchiveFailuresPreserveOutput(t *testing.T) {
	original := config.Config.E2ELoggingDir
	config.Config.E2ELoggingDir = t.TempDir()
	t.Cleanup(func() { config.Config.E2ELoggingDir = original })
	for _, name := range []string{"missing exit", "truncated", "bad checksum", "transport error"} {
		t.Run(name, func(t *testing.T) {
			var buffer bytes.Buffer
			compressed := gzip.NewWriter(&buffer)
			archive := tar.NewWriter(compressed)
			for _, entry := range []struct{ name, content string }{
				{"0.stdout", "partial"}, {"0.stderr", "details"}, {"0.exit", "0"},
			} {
				if name == "missing exit" && entry.name == "0.exit" {
					continue
				}
				require.NoError(t, archive.WriteHeader(&tar.Header{Name: entry.name, Mode: 0600, Size: int64(len(entry.content))}))
				_, err := archive.Write([]byte(entry.content))
				require.NoError(t, err)
			}
			require.NoError(t, archive.Close())
			require.NoError(t, compressed.Close())
			data := buffer.Bytes()
			switch name {
			case "truncated":
				data = data[:len(data)-8]
			case "bad checksum":
				data[len(data)-8] ^= 0xff
			}
			err := collectCommandLogs(t.Context(), name, map[string]string{"test.log": "command"},
				func(context.Context, string) (*podExecResult, error) {
					var err error
					if name == "transport error" {
						err = errors.New("SSH connection lost")
					}
					return &podExecResult{exitCode: "0", stdout: string(data)}, err
				})
			require.ErrorContains(t, err, "could not collect test.log")
			content, err := os.ReadFile(filepath.Join(artifactDir(name), "test.log"))
			require.NoError(t, err)
			assert.Contains(t, string(content), "log collection failed:")
			assert.Contains(t, string(content), "partial")
			assert.Contains(t, string(content), "details")
		})
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

func TestGenerateVMSSNameLinuxStartsWithDate(t *testing.T) {
	before := time.Now().Format(time.DateOnly)
	name := generateVMSSNameLinux("scenario")
	after := time.Now().Format(time.DateOnly)

	require.Regexp(t, `^\d{4}-\d{2}-\d{2}-[a-z0-9]{4}-scenario$`, name)
	assert.Contains(t, []string{before, after}, name[:10])
}

func TestGenerateVMSSNameLinuxHasValidEnding(t *testing.T) {
	for _, artifactName := range []string{
		"Ubuntu2204_A10_UpstreamDevicePlugin/attempt-1",
		strings.Repeat("a", 40) + "-suffix",
		strings.Repeat("a", 40) + ".suffix",
		"scenario-",
		"scenario.",
	} {
		t.Run(artifactName, func(t *testing.T) {
			name := generateVMSSNameLinux(artifactName)
			require.LessOrEqual(t, len(name), 57)
			require.Regexp(t, `^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$`, name)
		})
	}
}
