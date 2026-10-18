package scenario

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func rejectAllChannels(ch ssh.NewChannel) error {
	return ch.Reject(ssh.Prohibited, "test")
}

type sshConnectivityStub struct {
	check func(context.Context, *Scenario) error
	dial  func(context.Context, *Scenario) (*SSHClient, error)
}

func (f sshConnectivityStub) wait(ctx context.Context, s *Scenario) error {
	return waitForSSHConnectivity(ctx, s, f.check, f.dial, 10*time.Millisecond)
}

var errUnexpectedDial = errors.New("unexpected reconnect")

func failOnDial(t *testing.T) func(context.Context, *Scenario) (*SSHClient, error) {
	return func(context.Context, *Scenario) (*SSHClient, error) {
		t.Error("unexpected reconnect")
		return nil, errUnexpectedDial
	}
}

func newRebootScenario(client *SSHClient, wait time.Duration) *Scenario {
	return &Scenario{
		Config:  Config{WaitForSSHAfterReboot: wait},
		Runtime: &ScenarioRuntime{VM: &ScenarioVM{PrivateIP: "10.0.0.4", SSHClient: client}},
	}
}

func TestValidateSSHConnectivityReconnectsAfterStaleConnection(t *testing.T) {
	staleClient := newSessionTestSSHClient(t, rejectAllChannels)
	freshClient := newSessionTestSSHClient(t, rejectAllChannels)
	s := newRebootScenario(staleClient, time.Second)

	var dials int
	stub := sshConnectivityStub{
		check: func(_ context.Context, s *Scenario) error {
			if s.Runtime.VM.SSHClient == staleClient {
				return fmt.Errorf("SSH connection to 10.0.0.4 failed: %w", context.DeadlineExceeded)
			}
			return nil
		},
		dial: func(context.Context, *Scenario) (*SSHClient, error) {
			dials++
			return freshClient, nil
		},
	}

	require.NoError(t, stub.wait(t.Context(), s))
	require.Equal(t, 1, dials)
	require.Same(t, freshClient, s.Runtime.VM.SSHClient)
	_, _, err := staleClient.SendRequest("ping", true, nil)
	require.Error(t, err, "stale client should be closed")
}

func TestValidateSSHConnectivityRetriesFailedReconnect(t *testing.T) {
	freshClient := newSessionTestSSHClient(t, rejectAllChannels)
	s := newRebootScenario(nil, time.Second)

	var dials int
	stub := sshConnectivityStub{
		check: func(context.Context, *Scenario) error { return nil },
		dial: func(context.Context, *Scenario) (*SSHClient, error) {
			dials++
			if dials < 3 {
				return nil, errors.New("bastion tunnel not ready")
			}
			return freshClient, nil
		},
	}

	require.NoError(t, stub.wait(t.Context(), s))
	require.Equal(t, 3, dials)
	require.Same(t, freshClient, s.Runtime.VM.SSHClient)
}

func TestValidateSSHConnectivityFailsFastOnNonRebootError(t *testing.T) {
	client := newSessionTestSSHClient(t, rejectAllChannels)
	s := newRebootScenario(client, time.Second)
	permissionErr := errors.New("Permission denied (publickey)")

	stub := sshConnectivityStub{
		check: func(context.Context, *Scenario) error { return permissionErr },
		dial:  failOnDial(t),
	}

	require.ErrorIs(t, stub.wait(t.Context(), s), permissionErr)
	require.Same(t, client, s.Runtime.VM.SSHClient)
}

func TestValidateSSHConnectivityWithoutRebootWaitDoesNotReconnect(t *testing.T) {
	client := newSessionTestSSHClient(t, rejectAllChannels)
	s := newRebootScenario(client, 0)

	stub := sshConnectivityStub{
		check: func(context.Context, *Scenario) error { return context.DeadlineExceeded },
		dial:  failOnDial(t),
	}

	require.ErrorIs(t, stub.wait(t.Context(), s), context.DeadlineExceeded)
	require.Same(t, client, s.Runtime.VM.SSHClient)
}

func TestClosedSSHClientErrorIsStale(t *testing.T) {
	client := newSessionTestSSHClient(t, rejectAllChannels)
	require.NoError(t, client.Close())

	s := newRebootScenario(client, 0)
	s.Config.VHD = &config.Image{OS: config.OSUbuntu}
	err := attemptSSHConnection(t.Context(), s)
	require.Error(t, err)
	require.True(t, errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed), "transport error was lost: %v", err)
	require.True(t, isStaleSSHConnectionError(err), "unexpected error type: %v", err)
}

func TestValidateSSHConnectivityKeepsClientWhenWaitExpires(t *testing.T) {
	client := newSessionTestSSHClient(t, rejectAllChannels)
	s := newRebootScenario(client, 50*time.Millisecond)

	stub := sshConnectivityStub{
		check: func(ctx context.Context, _ *Scenario) error {
			<-ctx.Done()
			return fmt.Errorf("SSH connection to 10.0.0.4 failed: %w", ctx.Err())
		},
		dial: failOnDial(t),
	}

	require.Error(t, stub.wait(t.Context(), s))
	require.Same(t, client, s.Runtime.VM.SSHClient)
}
