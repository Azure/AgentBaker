package scenario

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestSSHCommandConcurrencyPerConnection(t *testing.T) {
	const operations = 24
	const script = "echo hello\necho world"
	opened := make(chan struct{}, operations*2)
	executing := make(chan struct{}, operations)
	uploadGate, commandGate := make(chan struct{}), make(chan struct{})
	releaseUploads := sync.OnceFunc(func() { close(uploadGate) })
	releaseCommands := sync.OnceFunc(func() { close(commandGate) })
	client := newSessionTestSSHClient(t, func(ch ssh.NewChannel) error {
		opened <- struct{}{}
		return serveSessionTestCommand(ch, func(command string, channel ssh.Channel) uint32 {
			if isSessionTestUpload(command) {
				<-uploadGate
				return receiveSessionTestScript(t, channel, script)
			}
			executing <- struct{}{}
			<-commandGate
			return 0
		})
	})
	t.Cleanup(releaseCommands)
	t.Cleanup(releaseUploads)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	results := make(chan error, operations)
	for range operations {
		go func() {
			result, err := runSSHCommand(ctx, client, script, false)
			if err == nil && result.exitCode != "0" {
				err = fmt.Errorf("exit code: %s", result.exitCode)
			}
			results <- err
		}()
	}
	for range 8 {
		requireSessionTestSignal(t, ctx, opened)
	}
	requireNoSessionTestSignal(t, opened)
	releaseUploads()
	for range 8 {
		requireSessionTestSignal(t, ctx, opened)
	}
	for range 8 {
		requireSessionTestSignal(t, ctx, executing)
	}
	requireNoSessionTestSignal(t, opened)
	releaseCommands()
	for range operations {
		select {
		case err := <-results:
			require.NoError(t, err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	assert.Empty(t, client.operations)
	assert.Len(t, opened, (operations-8)*2)
}

func TestSSHQueuedCancellationAndIndependentConnection(t *testing.T) {
	opened := make(chan struct{}, 16)
	gate := make(chan struct{})
	release := sync.OnceFunc(func() { close(gate) })
	client := newSessionTestSSHClient(t, func(ch ssh.NewChannel) error {
		opened <- struct{}{}
		return serveSessionTestCommand(ch, func(command string, channel ssh.Channel) uint32 {
			if isSessionTestUpload(command) {
				return discardSessionTestScript(t, channel)
			}
			<-gate
			return 0
		})
	})
	t.Cleanup(release)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	results := make(chan error, 8)
	for range 8 {
		go func() {
			_, err := runSSHCommand(ctx, client, "hold", false)
			results <- err
		}()
	}
	for range 16 {
		requireSessionTestSignal(t, ctx, opened)
	}

	queuedCtx, cancelQueued := context.WithCancel(ctx)
	defer cancelQueued()
	queuedResult := make(chan error, 1)
	go func() {
		_, err := runSSHCommand(queuedCtx, client, "echo queued\necho script", false)
		queuedResult <- err
	}()
	requireNoSessionTestSignal(t, opened)
	cancelQueued()
	select {
	case err := <-queuedResult:
		require.ErrorIs(t, err, context.Canceled)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	assert.Empty(t, opened)
	assert.Len(t, client.operations, 8)

	otherClient := newSessionTestSSHClient(t, func(ch ssh.NewChannel) error {
		return serveSessionTestCommand(ch, func(command string, channel ssh.Channel) uint32 {
			if isSessionTestUpload(command) {
				return receiveSessionTestScript(t, channel, "independent")
			}
			return 0
		})
	})
	result, err := runSSHCommand(ctx, otherClient, "independent", false)
	require.NoError(t, err)
	assert.Equal(t, "0", result.exitCode)

	release()
	for range 8 {
		select {
		case err := <-results:
			require.NoError(t, err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	result, err = runSSHCommand(ctx, client, "still healthy", false)
	require.NoError(t, err)
	assert.Equal(t, "0", result.exitCode)
	assert.Len(t, opened, 2)
	assert.Empty(t, client.operations)
}

func TestSSHAlreadyCanceledDoesNotOpenSession(t *testing.T) {
	var opens atomic.Int32
	client := newSessionTestSSHClient(t, func(ch ssh.NewChannel) error {
		opens.Add(1)
		return serveSessionTestCommand(ch, func(command string, channel ssh.Channel) uint32 {
			if isSessionTestUpload(command) {
				return receiveSessionTestScript(t, channel, "still healthy")
			}
			return 0
		})
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := runSSHCommand(ctx, client, "echo canceled\necho script", false)
	require.ErrorIs(t, err, context.Canceled)
	assert.Zero(t, opens.Load())
	assert.Empty(t, client.operations)
	result, err := runSSHCommand(context.Background(), client, "still healthy", false)
	require.NoError(t, err)
	assert.Equal(t, "0", result.exitCode)
	assert.EqualValues(t, 2, opens.Load())
}

func TestSSHOperationFailuresReleaseCapacity(t *testing.T) {
	for _, failure := range []string{"open", "start", "upload", "exit"} {
		t.Run(failure, func(t *testing.T) {
			var opens, executions atomic.Int32
			client := newSessionTestSSHClient(t, func(ch ssh.NewChannel) error {
				if opens.Add(1) <= 9 && failure != "exit" {
					switch failure {
					case "open":
						return ch.Reject(ssh.Prohibited, "denied")
					case "start":
						channel, requests, err := ch.Accept()
						if err != nil {
							return err
						}
						defer channel.Close()
						request := <-requests
						return request.Reply(false, nil)
					}
					return serveSessionTestCommand(ch, func(_ string, channel ssh.Channel) uint32 {
						if failure == "upload" {
							_, err := io.WriteString(channel.Stderr(), "transfer failed\n")
							assert.NoError(t, err)
						}
						return 17
					})
				}
				return serveSessionTestCommand(ch, func(command string, channel ssh.Channel) uint32 {
					if isSessionTestUpload(command) {
						return discardSessionTestScript(t, channel)
					}
					if executions.Add(1) <= 9 && failure == "exit" {
						return 17
					}
					return 0
				})
			})
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			command := "fail"
			for range 9 {
				result, err := runSSHCommand(ctx, client, command, false)
				if failure == "exit" {
					require.NoError(t, err)
					assert.Equal(t, "17", result.exitCode)
				} else {
					require.Error(t, err)
					if failure == "upload" {
						var exitErr *ssh.ExitError
						require.ErrorAs(t, err, &exitErr)
						assert.ErrorContains(t, err, "transfer failed")
						assert.Nil(t, result)
						assert.Zero(t, executions.Load(), "a failed upload must not execute the script")
					}
				}
				assert.Empty(t, client.operations)
			}
			result, err := runSSHCommand(ctx, client, "still healthy", false)
			require.NoError(t, err)
			assert.Equal(t, "0", result.exitCode)
			if failure == "exit" {
				assert.EqualValues(t, 20, opens.Load())
				assert.EqualValues(t, 10, executions.Load())
			} else {
				assert.EqualValues(t, 11, opens.Load())
			}
			assert.Empty(t, client.operations)
		})
	}
}

func isSessionTestUpload(command string) bool {
	return strings.HasPrefix(command, "umask 077;") ||
		strings.HasPrefix(command, `powershell -NoProfile -NonInteractive -Command "`)
}

func discardSessionTestScript(t *testing.T, channel ssh.Channel) uint32 {
	t.Helper()
	_, err := io.Copy(io.Discard, channel)
	if !assert.NoError(t, err) {
		return 1
	}
	return 0
}

func receiveSessionTestScript(t *testing.T, channel ssh.Channel, script string) uint32 {
	t.Helper()
	content, err := io.ReadAll(channel)
	if !assert.NoError(t, err) {
		return 1
	}
	assert.Equal(t, script, string(content))
	return 0
}

func requireSessionTestSignal(t *testing.T, ctx context.Context, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}

func requireNoSessionTestSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
		t.Fatal("unexpected SSH channel opened while all eight operations were busy")
	case <-time.After(100 * time.Millisecond):
	}
}
