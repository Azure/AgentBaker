package scenario

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/logging"
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
	start := time.Now()
	attempts := 0
	for {
		if err := ctx.Err(); err != nil {
			if lastErr != nil {
				logging.Logf(ctx, "SSH session open stopped after %d attempts in %s: %v; last rejection: %v", attempts, time.Since(start).Round(time.Millisecond), err, lastErr)
			}
			return fmt.Errorf("opening SSH session: %w (last rejection: %v)", err, lastErr)
		}
		attempts++
		err := open()
		if !isSSHSessionCapacityError(err) {
			if lastErr != nil {
				if err == nil {
					logging.Logf(ctx, "SSH session open succeeded after %d attempts in %s", attempts, time.Since(start).Round(time.Millisecond))
				} else {
					logging.Logf(ctx, "SSH session open failed after %d attempts in %s: %v", attempts, time.Since(start).Round(time.Millisecond), err)
				}
			}
			return err
		}
		if lastErr == nil {
			logging.Logf(ctx, "SSH session open rejected; retrying with backoff: %v", err)
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
