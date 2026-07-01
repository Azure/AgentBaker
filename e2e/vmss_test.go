package e2e

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCSEExitCodeOutboundConnFail pins the exit code constant to the value emitted by
// ERR_OUTBOUND_CONN_FAIL in parts/linux/cloud-init/artifacts/cse_helpers.sh. If the
// product error code changes, this test forces the harness mitigation to be updated.
func TestCSEExitCodeOutboundConnFail(t *testing.T) {
	require.Equal(t, "50", cseExitCodeOutboundConnFail)
}

// TestIsTransientOutboundCSEFailure verifies the bounded-retry classifier only matches a
// VMExtensionProvisioningError whose embedded CSE status reports ERR_OUTBOUND_CONN_FAIL,
// and ignores other failures so genuine regressions still surface.
func TestIsTransientOutboundCSEFailure(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error is not a transient outbound failure",
			err:  nil,
			want: false,
		},
		{
			// Real-world payload shape from Test_Ubuntu2204_HTTPSProxy_PrivateDNS.
			name: "outbound exit 50 embedded in extension provisioning error",
			err:  errors.New(`VMExtensionProvisioningError: [stdout] { "ExitCode": "50", "Output": "+ exit 50" } [stderr]`),
			want: true,
		},
		{
			name: "different CSE exit code is not retried",
			err:  errors.New(`VMExtensionProvisioningError: [stdout] { "ExitCode": "51" } [stderr]`),
			want: false,
		},
		{
			name: "unrelated azure error is not retried",
			err:  errors.New("AllocationFailed: not enough capacity"),
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isTransientOutboundCSEFailure(tt.err))
		})
	}
}
