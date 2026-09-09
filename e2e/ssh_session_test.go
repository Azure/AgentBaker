package e2e

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

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
		{"SCP resource shortage", errors.New("Error creating ssh session in copy to remote: ssh: rejected: resource shortage (full)"), true},
		{"SCP OpenSSH session limit", errors.New("Error creating ssh session in copy to remote: ssh: rejected: connect failed (open failed)"), true},
		{"SCP permission denied", errors.New("Error creating ssh session in copy to remote: ssh: rejected: administratively prohibited (open failed)"), false},
		{"SCP transport closed", errors.New("Error creating ssh session in copy to remote: EOF"), false},
		{"SCP transfer failure", errors.New("file transfer failed: ssh: rejected: resource shortage (full)"), false},
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
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		attempts := 0
		done := make(chan error, 1)
		go func() {
			done <- retrySSHSessionOpen(ctx, func() error {
				attempts++
				return &ssh.OpenChannelError{Reason: ssh.ResourceShortage, Message: "full"}
			})
		}()
		synctest.Wait()
		cancel()
		err := <-done
		require.ErrorIs(t, err, context.Canceled)
		assert.ErrorContains(t, err, "last rejection: ssh: rejected: resource shortage (full)")
		assert.Equal(t, 1, attempts)
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
			commands.Add(1)
			assert.Equal(t, "echo hello", command)
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
	assert.EqualValues(t, 2, opens.Load())
	assert.EqualValues(t, 1, commands.Load())
}

func TestSSHCommandFailureIsNotRetried(t *testing.T) {
	var opens atomic.Int32
	client := newSessionTestSSHClient(t, func(ch ssh.NewChannel) error {
		opens.Add(1)
		return serveSessionTestCommand(ch, func(_ string, channel ssh.Channel) uint32 {
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
	assert.EqualValues(t, 1, opens.Load())
}

func TestSCPCopyRetriesOnlyRejectedOpens(t *testing.T) {
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
					assert.True(t, strings.HasPrefix(command, "scp -qt "))
					reader := bufio.NewReader(channel)
					header, err := reader.ReadString('\n')
					if !assert.NoError(t, err) {
						return 1
					}
					assert.True(t, strings.HasPrefix(header, fmt.Sprintf("C0755 %d remote_script_", len(script))))
					if failTransfer {
						_, err := io.WriteString(channel, "\x01ssh: rejected: resource shortage (full)\n")
						assert.NoError(t, err)
						return 1
					}
					_, err = channel.Write([]byte{0})
					assert.NoError(t, err)
					content := make([]byte, len(script)+1)
					_, err = io.ReadFull(reader, content)
					assert.NoError(t, err)
					assert.Equal(t, script+"\x00", string(content))
					_, err = channel.Write([]byte{0})
					assert.NoError(t, err)
					return 0
				})
			})
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			remote, err := copyScriptToRemoteIfRequired(ctx, client, script, false)
			if failTransfer {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.True(t, strings.HasPrefix(remote, "/home/azureuser/remote_script_"))
			}
			assert.EqualValues(t, 2, opens.Load())
			assert.EqualValues(t, 1, transfers.Load())
		})
	}
}

func newSessionTestSSHClient(t *testing.T, handle func(ssh.NewChannel) error) *ssh.Client {
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
		assert.NoError(t, conn.SetDeadline(time.Now().Add(10*time.Second)))
		server, channels, requests, err := ssh.NewServerConn(conn, config)
		if !assert.NoError(t, err) {
			return
		}
		defer server.Close()
		go ssh.DiscardRequests(requests)
		for channel := range channels {
			assert.NoError(t, handle(channel))
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
	return client
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
