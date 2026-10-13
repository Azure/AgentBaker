package scenario

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/stretchr/testify/require"
)

func TestValidateSSHConnectivityReturnsNonRebootErrorUnchanged(t *testing.T) {
	s := &Scenario{
		Config: Config{
			VHD:                   &config.Image{OS: config.OSWindows},
			WaitForSSHAfterReboot: 5 * time.Minute,
		},
		Runtime: &ScenarioRuntime{VM: &ScenarioVM{PrivateIP: "10.0.0.4"}},
	}

	err := validateSSHConnectivity(context.Background(), s)

	require.Error(t, err)
	require.Contains(t, err.Error(), "SSH connection to 10.0.0.4 failed")
	require.NotContains(t, err.Error(), "for node to reboot")
}

func TestSSHConnectivityError(t *testing.T) {
	lastSSHError := errors.New("connection refused")
	nonRebootError := errors.New("SSH connection to 10.0.0.4 failed: context deadline exceeded")

	t.Run("poll timeout reports reboot wait", func(t *testing.T) {
		err := sshConnectivityError(nil, context.DeadlineExceeded, lastSSHError, 5*time.Minute)
		require.ErrorContains(t, err, "after waiting 5m0s for node to reboot")
		require.ErrorIs(t, err, lastSSHError)
	})

	t.Run("non-reboot error is returned unchanged", func(t *testing.T) {
		err := sshConnectivityError(nil, nonRebootError, nonRebootError, 10*time.Second)
		require.Same(t, nonRebootError, err)
	})

	for _, parentErr := range []error{context.Canceled, context.DeadlineExceeded} {
		t.Run("parent "+parentErr.Error()+" is returned unchanged", func(t *testing.T) {
			err := sshConnectivityError(parentErr, parentErr, lastSSHError, time.Minute)
			require.Equal(t, parentErr, err)
		})

		t.Run("parent "+parentErr.Error()+" wins over in-flight SSH error", func(t *testing.T) {
			err := sshConnectivityError(parentErr, nonRebootError, nonRebootError, time.Minute)
			require.Equal(t, parentErr, err)
		})
	}

	t.Run("success returns nil", func(t *testing.T) {
		require.NoError(t, sshConnectivityError(nil, nil, lastSSHError, time.Second))
	})
}
