package scenario

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/toolkit"
	"golang.org/x/crypto/ssh"
)

// Leave headroom below OpenSSH's default MaxSessions of 10 per connection.
// Capacity retries remain necessary: this client-side limit does not eliminate rejections.
const maxConcurrentSSHOperations = 8

type SSHClient struct {
	*ssh.Client
	operations chan struct{}
}

func newSSHClient(client *ssh.Client) *SSHClient {
	return &SSHClient{
		Client:     client,
		operations: make(chan struct{}, maxConcurrentSSHOperations),
	}
}

func retrySSHSessionOpen(ctx context.Context, open func() error) error {
	delay := 100 * time.Millisecond
	var lastErr error
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("opening SSH session: %w (last rejection: %v)", err, lastErr)
		}
		err := open()
		if !isSSHSessionCapacityError(err) {
			return err
		}
		if lastErr == nil {
			toolkit.Logf(ctx, "SSH session open rejected; retrying: %v", err)
		}
		lastErr = err
		timer := time.NewTimer(delay + time.Duration(rand.IntN(100))*time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
		delay = min(2*delay, time.Second)
	}
}

func isSSHSessionCapacityError(err error) bool {
	if err == nil {
		return false
	}
	var rejection *ssh.OpenChannelError
	if errors.As(err, &rejection) {
		return rejection.Reason == ssh.ResourceShortage ||
			(rejection.Reason == ssh.ConnectionFailed && rejection.Message == "open failed")
	}

	message, ok := strings.CutPrefix(err.Error(), "Error creating ssh session in copy to remote: ")
	if !ok {
		return false
	}
	return message == "ssh: rejected: connect failed (open failed)" ||
		(strings.HasPrefix(message, "ssh: rejected: resource shortage (") && strings.HasSuffix(message, ")"))
}
