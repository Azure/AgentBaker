package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCloudFileWrites(t *testing.T) {
	t.Run("create and replace", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "certificates", "root.crt")
		require.NoError(t, writeCloudFile(path, []byte("original")))
		require.NoError(t, writeCloudFile(path, []byte("replacement")))
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, "replacement", string(data))
	})
	t.Run("reject symlinks outside the destination directory", func(t *testing.T) {
		directory := t.TempDir()
		outside := filepath.Join(t.TempDir(), "outside.crt")
		require.NoError(t, os.WriteFile(outside, []byte("original"), 0o600))
		path := filepath.Join(directory, "root.crt")
		require.NoError(t, os.Symlink(outside, path))
		require.Error(t, writeCloudFile(path, []byte("replacement")))
		data, err := os.ReadFile(outside)
		require.NoError(t, err)
		require.Equal(t, "original", string(data))
	})
}

func TestCloudPlatform(t *testing.T) {
	for _, tc := range []struct {
		name, release, want string
	}{
		{"Ubuntu", cloudTestUbuntu, osReleaseIDUbuntu},
		{"Mariner", "ID=mariner\nNAME=CBL-Mariner\nVERSION_ID=2.0\n", "mariner"},
		{"AzureLinux", "ID=azurelinux\nNAME=\"Microsoft Azure Linux\"\nVERSION_ID=3.0\n", osReleaseIDAzureLinux},
		{"OSGuard", "ID=azurelinux\nVARIANT_ID=osguard\nVERSION_ID=3.0\n", osReleaseIDAzureLinux},
		{"Flatcar", "ID=flatcar\n", osReleaseIDFlatcar},
		{"ACL", "ID=azurecontainerlinux\n", osReleaseIDAzureContainerLinux},
		{"ACL variant", "ID=azurelinux\nVARIANT_ID=azurecontainerlinux\n", osReleaseIDAzureContainerLinux},
		{"Mixed-case ACL variant", "ID=azurelinux\nVARIANT_ID=AzureContainerLinux\n", osReleaseIDAzureContainerLinux},
		{"unsupported", "ID=debian\n", ""},
		{"missing ID", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestCloudInitializer(t, tc.release)
			platform, err := c.platform()
			if tc.want == "" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.want, platform.ID)
			}
		})
	}
}

func TestCloudCertificateResponseLimits(t *testing.T) {
	_, err := readCloudResponse(&http.Response{
		Body: io.NopCloser(strings.NewReader(strings.Repeat("x", cloudResponseLimit+1))),
	})
	require.ErrorContains(t, err, "size limit")
}

func TestCloudCommandSuccess(t *testing.T) {
	c := &cloudInitializer{}
	output, err := c.command(t.Context(), []byte("input"), os.Args[0],
		"-test.run=^TestCloudCommandHelperProcess$", "--", "echo", "")
	require.NoError(t, err)
	require.Equal(t, "C:input", string(output))
}

func TestCloudCommandAlreadyCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	called := false
	c := &cloudInitializer{run: func(cmd *exec.Cmd) error {
		called = true
		return cmd.Run()
	}}
	output, err := c.command(ctx, nil, os.Args[0],
		"-test.run=^TestCloudCommandHelperProcess$", "--", "echo", "")
	require.ErrorIs(t, err, context.Canceled)
	require.False(t, called, "an already-canceled context must not invoke the command runner")
	require.Nil(t, output)
}

func TestCloudCommandCancellation(t *testing.T) {
	for _, tc := range []struct {
		name         string
		mode         string
		returnWithin time.Duration
	}{
		{"kills descendants with inherited output", "group", 2 * time.Second},
		{"bounds output draining after a descendant leaves the group", "detached", 7 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			readyPath := filepath.Join(t.TempDir(), "child.pid")
			ctx, cancel := context.WithCancel(t.Context())
			started := make(chan *os.Process, 1)
			finished := make(chan struct{})
			c := &cloudInitializer{run: func(cmd *exec.Cmd) error {
				if err := cmd.Start(); err != nil {
					return err
				}
				started <- cmd.Process
				return cmd.Wait()
			}}
			var parent, child *os.Process
			t.Cleanup(func() {
				cancel()
				for _, process := range []*os.Process{child, parent} {
					if process == nil {
						continue
					}
					if err := process.Kill(); err != nil &&
						!errors.Is(err, os.ErrProcessDone) && !errors.Is(err, syscall.ESRCH) {
						t.Errorf("clean up process %d: %v", process.Pid, err)
					}
				}
				if child != nil {
					defer child.Release()
				}
				select {
				case <-finished:
				case <-time.After(5 * time.Second):
					t.Error("command did not finish after subprocess cleanup")
				}
			})
			var output []byte
			var commandErr error
			go func() {
				defer close(finished)
				output, commandErr = c.command(ctx, nil, os.Args[0],
					"-test.run=^TestCloudCommandHelperProcess$", "--", tc.mode, readyPath)
			}()
			select {
			case parent = <-started:
			case <-finished:
				t.Fatalf("command exited before starting its child: %v", commandErr)
			case <-time.After(5 * time.Second):
				t.Fatal("command did not start")
			}
			var childPID int
			require.Eventually(t, func() bool {
				data, err := os.ReadFile(readyPath)
				if err != nil {
					return false
				}
				childPID, err = strconv.Atoi(strings.TrimSpace(string(data)))
				return err == nil && childPID > 0
			}, 5*time.Second, 10*time.Millisecond, "command did not start its output-holding child")
			var err error
			child, err = os.FindProcess(childPID)
			require.NoError(t, err)

			cancel()
			select {
			case <-finished:
				require.Error(t, commandErr)
				require.Contains(t, string(output), "stdout-marker")
				require.ErrorContains(t, commandErr, "stderr-marker")
			case <-time.After(tc.returnWithin):
				t.Fatalf("command did not return within %s of cancellation", tc.returnWithin)
			}
		})
	}
}

func TestCloudCommandHelperProcess(t *testing.T) {
	if len(os.Args) != 5 || os.Args[2] != "--" {
		return
	}
	switch os.Args[3] {
	case "echo":
		input, err := io.ReadAll(os.Stdin)
		require.NoError(t, err)
		_, err = fmt.Fprintf(os.Stdout, "%s:%s", os.Getenv("LC_ALL"), input)
		require.NoError(t, err)
	case "hold":
		time.Sleep(time.Minute)
	case "group", "detached":
		child := exec.Command(os.Args[0], os.Args[1], "--", "hold", os.Args[4])
		child.Stdout, child.Stderr = os.Stdout, os.Stderr
		if os.Args[3] == "detached" {
			child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		}
		require.NoError(t, child.Start())
		defer child.Process.Kill()
		fmt.Fprintln(os.Stdout, "stdout-marker")
		fmt.Fprintln(os.Stderr, "stderr-marker")
		require.NoError(t, os.WriteFile(os.Args[4], []byte(strconv.Itoa(child.Process.Pid)), 0o600))
		require.NoError(t, child.Wait())
	default:
		t.Fatalf("unknown command helper mode %q", os.Args[3])
	}
	os.Exit(0)
}
