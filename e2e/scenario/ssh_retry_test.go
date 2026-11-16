package scenario

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"testing/synctest"

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
