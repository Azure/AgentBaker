package scenario

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestScriptUploadBytesAndUniqueFiles(t *testing.T) {
	for _, windows := range []bool{false, true} {
		t.Run(fmt.Sprintf("windows=%t", windows), func(t *testing.T) {
			var powershell string
			if windows {
				var err error
				powershell, err = exec.LookPath("pwsh")
				if err != nil {
					t.Skip("pwsh is required to execute the PowerShell upload wrapper")
				}
			}
			dir := t.TempDir()
			client := newSessionTestSSHClient(t, func(ch ssh.NewChannel) error {
				return serveSessionTestCommand(ch, func(command string, channel ssh.Channel) uint32 {
					var cmd *exec.Cmd
					if windows {
						const prefix = `powershell -NoProfile -NonInteractive -Command "`
						if strings.HasPrefix(command, prefix) {
							script := strings.TrimSuffix(strings.TrimPrefix(command, prefix), `"`)
							script = strings.ReplaceAll(script, "c:/", filepath.ToSlash(dir)+"/")
							cmd = exec.CommandContext(t.Context(), powershell, "-NoProfile", "-NonInteractive", "-Command", script)
						} else {
							path := strings.TrimPrefix(command, "powershell -NoProfile -NonInteractive -File ")
							path = strings.ReplaceAll(path, "c:/", filepath.ToSlash(dir)+"/")
							cmd = exec.CommandContext(t.Context(), powershell, "-NoProfile", "-NonInteractive", "-File", path)
						}
					} else {
						command = strings.ReplaceAll(command, "/home/azureuser/", dir+"/")
						cmd = exec.CommandContext(t.Context(), "sh", "-c", command)
					}
					cmd.Stdin, cmd.Stdout, cmd.Stderr = channel, channel, channel.Stderr()
					if err := cmd.Run(); err != nil {
						var exitErr *exec.ExitError
						if errors.As(err, &exitErr) {
							return uint32(exitErr.ExitCode())
						}
						assert.NoError(t, err)
						return 1
					}
					return 0
				})
			})
			// Include shell metacharacters, CRLF, UTF-8, and a payload larger than an SSH packet.
			script := "\xef\xbb\xbf# caf\xc3\xa9\r\n$HOME ` \" ' ; & | \x00\n" + strings.Repeat("# payload\n", 8192)
			paths := make(map[string]bool)
			for range 2 {
				remote, err := uploadSSHScript(t.Context(), client.Client, script, windows)
				require.NoError(t, err)
				name := filepath.Base(remote)
				require.False(t, paths[name], "script path reused")
				paths[name] = true
				content, err := os.ReadFile(filepath.Join(dir, name))
				require.NoError(t, err)
				assert.Equal(t, script, string(content))
				if !windows {
					info, err := os.Stat(filepath.Join(dir, name))
					require.NoError(t, err)
					assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
				}
			}
			for _, test := range []struct {
				name, linuxScript, windowsScript, stdout, stderr, exitCode string
			}{
				{name: "empty", exitCode: "0"},
				{
					name: "single line", linuxScript: "echo SSH_CONNECTION_OK",
					windowsScript: "echo SSH_CONNECTION_OK", stdout: "SSH_CONNECTION_OK", exitCode: "0",
				},
				{
					name:        "separate stdin and exit status",
					linuxScript: "if read -r input; then exit 99; fi\nprintf 'stdout\\n'; printf 'stderr\\n' >&2; exit 17",
					windowsScript: "if ([Console]::In.ReadToEnd().Length -ne 0) { exit 99 }\r\n" +
						"[Console]::Out.WriteLine('stdout'); [Console]::Error.WriteLine('stderr'); exit 17",
					stdout: "stdout", stderr: "stderr", exitCode: "17",
				},
			} {
				t.Run(test.name, func(t *testing.T) {
					script := test.linuxScript
					if windows {
						script = test.windowsScript
					}
					result, err := runSSHCommand(t.Context(), client, script, windows)
					require.NoError(t, err)
					assert.Equal(t, test.exitCode, result.exitCode)
					assert.Equal(t, test.stdout, strings.TrimSpace(result.stdout))
					assert.Equal(t, test.stderr, strings.TrimSpace(result.stderr))
				})
			}
		})
	}
}

func TestScriptUploadDeadlineClosesBlockedTransfer(t *testing.T) {
	release := make(chan struct{})
	client := newSessionTestSSHClient(t, func(ch ssh.NewChannel) error {
		channel, requests, err := ch.Accept()
		if err != nil {
			return err
		}
		defer channel.Close()
		request := <-requests
		if err := request.Reply(true, nil); err != nil {
			return err
		}
		<-release
		return nil
	})
	t.Cleanup(func() { close(release) })
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	_, err := uploadSSHScript(ctx, client.Client, strings.Repeat("x\n", 2*1024*1024), true)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}
