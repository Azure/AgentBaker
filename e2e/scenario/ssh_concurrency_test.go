package scenario

import (
	"bufio"
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
	for _, test := range []struct {
		name      string
		command   string
		isWindows bool
		upload    bool
	}{
		{name: "command", command: "echo hello"},
		{name: "Linux script", command: "echo hello\necho world", upload: true},
		{name: "Windows script", command: "Write-Output hello", isWindows: true, upload: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			const operations = 24
			opened := make(chan struct{}, operations*2)
			executing := make(chan struct{}, operations)
			uploadGate := make(chan struct{})
			commandGate := make(chan struct{})
			releaseUploads := sync.OnceFunc(func() { close(uploadGate) })
			releaseCommands := sync.OnceFunc(func() { close(commandGate) })
			client := newSessionTestSSHClient(t, func(ch ssh.NewChannel) error {
				opened <- struct{}{}
				return serveSessionTestCommand(ch, func(command string, channel ssh.Channel) uint32 {
					if strings.HasPrefix(command, "scp -qt ") {
						<-uploadGate
						return receiveSessionTestScript(t, channel, test.command)
					}
					executing <- struct{}{}
					<-commandGate
					return 0
				})
			})
			t.Cleanup(releaseCommands)
			t.Cleanup(releaseUploads)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			results := make(chan error, operations)
			for range operations {
				go func() {
					result, err := runSSHCommand(ctx, client, test.command, test.isWindows)
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
			if test.upload {
				releaseUploads()
				for range 8 {
					requireSessionTestSignal(t, ctx, opened)
				}
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
			wantRemainingOpens := operations - 8
			if test.upload {
				wantRemainingOpens *= 2
			}
			assert.Len(t, opened, wantRemainingOpens)
		})
	}
}

func TestSSHQueuedCancellationAndIndependentConnection(t *testing.T) {
	opened := make(chan struct{}, 16)
	gate := make(chan struct{})
	release := sync.OnceFunc(func() { close(gate) })
	client := newSessionTestSSHClient(t, func(ch ssh.NewChannel) error {
		opened <- struct{}{}
		return serveSessionTestCommand(ch, func(_ string, _ ssh.Channel) uint32 {
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
	for range 8 {
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
		return serveSessionTestCommand(ch, func(_ string, _ ssh.Channel) uint32 { return 0 })
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
	assert.Len(t, opened, 1)
	assert.Empty(t, client.operations)
}

func TestSSHAlreadyCanceledDoesNotOpenSession(t *testing.T) {
	var opens atomic.Int32
	client := newSessionTestSSHClient(t, func(ch ssh.NewChannel) error {
		opens.Add(1)
		return serveSessionTestCommand(ch, func(_ string, _ ssh.Channel) uint32 { return 0 })
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
	assert.EqualValues(t, 1, opens.Load())
}

func TestSSHOperationFailuresReleaseCapacity(t *testing.T) {
	for _, failure := range []string{"open", "start", "upload", "exit"} {
		t.Run(failure, func(t *testing.T) {
			var opens atomic.Int32
			client := newSessionTestSSHClient(t, func(ch ssh.NewChannel) error {
				if opens.Add(1) <= 9 {
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
							_, err := io.WriteString(channel, "\x01transfer failed\n")
							assert.NoError(t, err)
						}
						return 17
					})
				}
				return serveSessionTestCommand(ch, func(_ string, _ ssh.Channel) uint32 { return 0 })
			})
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			command := "fail"
			if failure == "upload" {
				command += "\nscript"
			}
			for range 9 {
				result, err := runSSHCommand(ctx, client, command, false)
				if failure == "exit" {
					require.NoError(t, err)
					assert.Equal(t, "17", result.exitCode)
				} else {
					require.Error(t, err)
				}
				assert.Empty(t, client.operations)
			}
			result, err := runSSHCommand(ctx, client, "still healthy", false)
			require.NoError(t, err)
			assert.Equal(t, "0", result.exitCode)
			assert.EqualValues(t, 10, opens.Load())
			assert.Empty(t, client.operations)
		})
	}
}

func receiveSessionTestScript(t *testing.T, channel ssh.Channel, script string) uint32 {
	t.Helper()
	reader := bufio.NewReader(channel)
	header, err := reader.ReadString('\n')
	if !assert.NoError(t, err) {
		return 1
	}
	assert.True(t, strings.HasPrefix(header, fmt.Sprintf("C0755 %d ", len(script))))
	if _, err := channel.Write([]byte{0}); !assert.NoError(t, err) {
		return 1
	}
	content := make([]byte, len(script)+1)
	if _, err := io.ReadFull(reader, content); !assert.NoError(t, err) {
		return 1
	}
	assert.Equal(t, script+"\x00", string(content))
	_, err = channel.Write([]byte{0})
	assert.NoError(t, err)
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
