package scenario

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestSSHSessionCapacityErrors(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want bool
	}{
		{"success", nil, false},
		{"resource shortage", &ssh.OpenChannelError{Reason: ssh.ResourceShortage, Message: "full"}, true},
		{"OpenSSH session limit", &ssh.OpenChannelError{Reason: ssh.ConnectionFailed, Message: "open failed"}, true},
		{"wrapped rejection", fmt.Errorf("open: %w", &ssh.OpenChannelError{Reason: ssh.ResourceShortage}), true},
		{"permission denied", &ssh.OpenChannelError{Reason: ssh.Prohibited, Message: "open failed"}, false},
		{"unsupported channel", &ssh.OpenChannelError{Reason: ssh.UnknownChannelType}, false},
		{"connection refused", &ssh.OpenChannelError{Reason: ssh.ConnectionFailed, Message: "connection refused"}, false},
		{"transport closed", io.EOF, false},
		{"authentication", errors.New("ssh: handshake failed: unable to authenticate"), false},
		{"untyped rejection", errors.New("ssh: rejected: resource shortage (full)"), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, isSSHSessionCapacityError(test.err))
		})
	}
}

func TestRetrySSHSessionOpen(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		attempts := 0
		err := retrySSHSessionOpen(context.Background(), func() error {
			attempts++
			if attempts < 4 {
				return &ssh.OpenChannelError{Reason: ssh.ResourceShortage}
			}
			return nil
		})
		require.NoError(t, err)
		assert.Equal(t, 4, attempts)
	})
}

func TestRetrySSHSessionOpenLogsOutcome(t *testing.T) {
	for _, test := range []struct {
		name    string
		err     error
		outcome string
	}{
		{"success", nil, "succeeded"},
		{"failure", io.EOF, "failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				logger := &executionLogger{}
				attempts := 0
				err := retrySSHSessionOpen(logging.WithLogger(t.Context(), logger), func() error {
					attempts++
					if attempts < 4 {
						return &ssh.OpenChannelError{Reason: ssh.ResourceShortage}
					}
					return test.err
				})
				require.ErrorIs(t, err, test.err)
				require.Len(t, logger.logs, 2)
				assert.Contains(t, logger.logs[0], "retrying with backoff")
				assert.Contains(t, logger.logs[1], "SSH session open "+test.outcome+" after 4 attempts in ")
			})
		})
	}
}

func TestRetrySSHSessionOpenReturnsOtherErrors(t *testing.T) {
	for _, failure := range []error{
		io.EOF,
		&ssh.OpenChannelError{Reason: ssh.Prohibited},
		errors.New("ssh: handshake failed: unable to authenticate"),
	} {
		attempts := 0
		err := retrySSHSessionOpen(context.Background(), func() error {
			attempts++
			return failure
		})
		assert.Same(t, failure, err)
		assert.Equal(t, 1, attempts)
	}
}

func TestRetrySSHSessionOpenCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		logger := &executionLogger{}
		ctx, cancel := context.WithCancel(logging.WithLogger(t.Context(), logger))
		defer cancel()
		attempts := 0
		rejection := &ssh.OpenChannelError{Reason: ssh.ResourceShortage, Message: "full"}
		done := make(chan error, 1)
		go func() {
			done <- retrySSHSessionOpen(ctx, func() error {
				attempts++
				return rejection
			})
		}()
		synctest.Wait()
		cancel()
		err := <-done
		require.ErrorIs(t, err, context.Canceled)
		assert.ErrorContains(t, err, "last rejection: "+rejection.Error())
		assert.Equal(t, 1, attempts)
		require.Len(t, logger.logs, 2)
		assert.Contains(t, logger.logs[1], "SSH session open stopped after 1 attempts in ")
		assert.Contains(t, logger.logs[1], "context canceled")
	})
}

func TestRetrySSHSessionOpenAlreadyCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	err := retrySSHSessionOpen(ctx, func() error {
		called = true
		return nil
	})
	require.ErrorIs(t, err, context.Canceled)
	assert.False(t, called)
}

func TestSSHCommandRetriesRejectedOpensButRunsOnce(t *testing.T) {
	var opens, commands atomic.Int32
	client := newSessionTestSSHClient(t, func(ch ssh.NewChannel) error {
		if opens.Add(1) == 1 {
			return ch.Reject(ssh.ConnectionFailed, "open failed")
		}
		return serveSessionTestCommand(ch, func(command string, channel ssh.Channel) uint32 {
			if isSessionTestUpload(command) {
				return receiveSessionTestScript(t, channel, "echo hello")
			}
			commands.Add(1)
			assert.True(t, strings.HasPrefix(command, "bash /home/azureuser/remote_script_"))
			_, err := io.WriteString(channel, "hello\n")
			assert.NoError(t, err)
			return 0
		})
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := runSSHCommand(ctx, client, "echo hello", false)
	require.NoError(t, err)
	assert.Equal(t, "hello\n", result.stdout)
	assert.Equal(t, "0", result.exitCode)
	assert.EqualValues(t, 3, opens.Load())
	assert.EqualValues(t, 1, commands.Load())
}

func TestSSHCommandFailureIsNotRetried(t *testing.T) {
	var opens atomic.Int32
	client := newSessionTestSSHClient(t, func(ch ssh.NewChannel) error {
		opens.Add(1)
		return serveSessionTestCommand(ch, func(command string, channel ssh.Channel) uint32 {
			if isSessionTestUpload(command) {
				return receiveSessionTestScript(t, channel, "false")
			}
			_, err := io.WriteString(channel.Stderr(), "ssh: rejected: resource shortage (full)")
			assert.NoError(t, err)
			return 17
		})
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := runSSHCommand(ctx, client, "false", false)
	require.NoError(t, err)
	assert.Equal(t, "17", result.exitCode)
	assert.EqualValues(t, 2, opens.Load())
}

func TestSSHCommandMissingExitStatusIsNotSuccess(t *testing.T) {
	var opens atomic.Int32
	client := newSessionTestSSHClient(t, func(ch ssh.NewChannel) error {
		if opens.Add(1) == 1 {
			return serveSessionTestCommand(ch, func(_ string, channel ssh.Channel) uint32 {
				return receiveSessionTestScript(t, channel, "false")
			})
		}
		channel, requests, err := ch.Accept()
		if err != nil {
			return err
		}
		defer channel.Close()
		for request := range requests {
			if err := request.Reply(request.Type == "exec", nil); err != nil {
				return err
			}
			if request.Type == "exec" {
				return nil
			}
		}
		return nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := runSSHCommand(ctx, client, "false", false)
	var missingStatus *ssh.ExitMissingError
	require.ErrorAs(t, err, &missingStatus)
	assert.Nil(t, result)
	assert.EqualValues(t, 2, opens.Load())
}

func TestSSHCommandCancellationUnblocksSetupAndRun(t *testing.T) {
	const script = "echo hello\necho world"
	for _, stage := range []string{"channel-open", "exec-request", "upload", "command"} {
		t.Run(stage, func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			stop := sync.OnceFunc(func() { close(release) })
			var opens atomic.Int32
			client := newSessionTestSSHClient(t, func(ch ssh.NewChannel) error {
				if opens.Add(1) == 1 && stage == "command" {
					return serveSessionTestCommand(ch, func(_ string, channel ssh.Channel) uint32 {
						return receiveSessionTestScript(t, channel, script)
					})
				}
				if stage == "channel-open" {
					close(entered)
					<-release
					return nil
				}
				channel, requests, err := ch.Accept()
				if err != nil {
					return err
				}
				defer channel.Close()
				for request := range requests {
					if request.Type == "exec" {
						if stage == "command" || stage == "upload" {
							if err := request.Reply(true, nil); err != nil {
								return err
							}
						}
						close(entered)
						<-release
						return nil
					}
					if err := request.Reply(false, nil); err != nil {
						return err
					}
				}
				return nil
			})
			t.Cleanup(stop)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				_, err := runSSHCommand(ctx, client, script, false)
				done <- err
			}()
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("SSH operation did not reach " + stage)
			}
			cancel()
			select {
			case err := <-done:
				require.ErrorIs(t, err, context.Canceled)
				assert.Empty(t, client.operations)
			case <-time.After(time.Second):
				_ = client.Close()
				stop()
				<-done
				t.Fatal("SSH operation ignored cancellation at " + stage)
			}
		})
	}
}

func TestScriptUploadSetupHonorsDeadline(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	client := newSessionTestSSHClient(t, func(ch ssh.NewChannel) error {
		close(entered)
		<-release
		return nil
	})
	t.Cleanup(func() { close(release) })
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := uploadSSHScript(ctx, client.Client, "Write-Output hello", true)
		done <- err
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("upload did not attempt to open a channel")
	}
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.DeadlineExceeded)
	case <-time.After(time.Second):
		_ = client.Close()
		t.Fatal("upload setup ignored the copy deadline")
	}
}

func TestScriptUploadRetriesOnlyRejectedOpens(t *testing.T) {
	for _, failTransfer := range []bool{false, true} {
		t.Run(fmt.Sprintf("fail-transfer=%t", failTransfer), func(t *testing.T) {
			var opens, transfers atomic.Int32
			script := "set -e\necho hello\n"
			client := newSessionTestSSHClient(t, func(ch ssh.NewChannel) error {
				if opens.Add(1) == 1 {
					return ch.Reject(ssh.ResourceShortage, "full")
				}
				return serveSessionTestCommand(ch, func(command string, channel ssh.Channel) uint32 {
					transfers.Add(1)
					assert.True(t, strings.HasPrefix(command, "umask 077; set -C; cat > "))
					if failTransfer {
						_, err := io.WriteString(channel.Stderr(), "ssh: rejected: resource shortage (full)\n")
						assert.NoError(t, err)
						return 1
					}
					return receiveSessionTestScript(t, channel, script)
				})
			})
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			remote, err := uploadSSHScript(ctx, client.Client, script, false)
			if failTransfer {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.True(t, strings.HasPrefix(remote, "bash /home/azureuser/remote_script_"))
			}
			assert.EqualValues(t, 2, opens.Load())
			assert.EqualValues(t, 1, transfers.Load())
		})
	}
}

func newSessionTestSSHClient(t *testing.T, handle func(ssh.NewChannel) error) *SSHClient {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(privateKey)
	require.NoError(t, err)
	config := &ssh.ServerConfig{NoClientAuth: true}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if !assert.NoError(t, err) {
			return
		}
		defer conn.Close()
		if deadline, ok := t.Deadline(); ok {
			assert.NoError(t, conn.SetDeadline(deadline))
		}
		server, channels, requests, err := ssh.NewServerConn(conn, config)
		if !assert.NoError(t, err) {
			return
		}
		defer server.Close()
		go ssh.DiscardRequests(requests)
		var handlers sync.WaitGroup
		defer handlers.Wait()
		for channel := range channels {
			handlers.Go(func() {
				assert.NoError(t, handle(channel))
			})
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		<-done
	})
	client, err := ssh.Dial("tcp", listener.Addr().String(), &ssh.ClientConfig{
		User:            "test",
		HostKeyCallback: ssh.FixedHostKey(signer.PublicKey()),
		Timeout:         5 * time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	return newSSHClient(client)
}

func serveSessionTestCommand(ch ssh.NewChannel, run func(string, ssh.Channel) uint32) error {
	channel, requests, err := ch.Accept()
	if err != nil {
		return err
	}
	defer channel.Close()
	for request := range requests {
		if request.Type != "exec" {
			if err := request.Reply(false, nil); err != nil {
				return err
			}
			continue
		}
		var payload struct{ Command string }
		if err := ssh.Unmarshal(request.Payload, &payload); err != nil {
			return err
		}
		if err := request.Reply(true, nil); err != nil {
			return err
		}
		status := run(payload.Command, channel)
		_, err := channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{status}))
		return err
	}
	return io.EOF
}
