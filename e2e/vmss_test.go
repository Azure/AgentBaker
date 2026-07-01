package e2e

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
	"github.com/stretchr/testify/require"
)

// TestCSEExitCodeOutboundConnFail pins the exit code constant to the value emitted by
// ERR_OUTBOUND_CONN_FAIL in parts/linux/cloud-init/artifacts/cse_helpers.sh. If the
// product error code changes, this test forces the harness mitigation to be updated.
func TestCSEExitCodeOutboundConnFail(t *testing.T) {
	require.Equal(t, "50", cseExitCodeOutboundConnFail)
}

// TestParseLinuxCSEMessageOutboundExitCode verifies that parseLinuxCSEMessage extracts the
// outbound-connectivity exit code from a real CustomScript extension instance-view status.
// getLinuxCSEExitCode relies on this parsing to classify the retryable e2e flake, so a
// change to the message format must be reflected here.
func TestParseLinuxCSEMessageOutboundExitCode(t *testing.T) {
	tests := []struct {
		name         string
		code         string
		message      string
		wantExitCode string
		wantErr      bool
	}{
		{
			name:         "well-formed CSE json with outbound exit code",
			code:         "ProvisioningState/failed/0",
			message:      `Enable failed: [stdout] { "ExitCode": "50", "Output": "+ exit 50" } [stderr]`,
			wantExitCode: "50",
		},
		{
			name:         "unparsable body falls back to extension exit status",
			code:         "ProvisioningState/failed/0",
			message:      `Enable failed: failed to execute command: command terminated with exit status=50 [stdout]not-json[stderr]`,
			wantExitCode: "50",
		},
		{
			name:         "well-formed CSE json with non-outbound exit code",
			code:         "ProvisioningState/failed/0",
			message:      `Enable failed: [stdout] { "ExitCode": "51", "Output": "+ exit 51" } [stderr]`,
			wantExitCode: "51",
		},
		{
			// Real Test_Ubuntu2204_HTTPSProxy_PrivateDNS/default failure: the outer extension
			// wrapper and the CSE status both report 50.
			name: "real outbound flake, outer exit 50 and cse exit 50",
			code: "ProvisioningState/failed/0",
			message: "failed to execute command: command terminated with exit status=50\n[stdout]\n" +
				`{ "ExitCode": "50", "Output": "Processing manual pages under /usr/local/man...\n++ date\n+ echo 'man-db finished updates'\n+ exit 50", "Error": "", "ExecDuration": "155", "BootDatapoints": { "KubeletStartTime": "n/a" } }` +
				"\n\n[stderr]\ndate: invalid date 'n/a'\n",
			wantExitCode: "50",
		},
		{
			// Real Test_Ubuntu2204_HTTPSProxy_PrivateDNS/scriptless_nbc failure: the outer
			// extension wrapper reports exit status=1, but the CSE status reports 50. The
			// classifier must read the CSE ExitCode field, not the outer wrapper.
			name: "real outbound flake, outer exit 1 but cse exit 50",
			code: "ProvisioningState/failed/0",
			message: "failed to execute command: command terminated with exit status=1\n[stdout]\n" +
				`{ "ExitCode": "50", "Output": "man-db finished updates\n+ exit 50", "Error": "", "ExecDuration": "70", "BootDatapoints": { "KubeletStartTime": "n/a" } }` +
				"\n\n[stderr]\ndate\n",
			wantExitCode: "50",
		},
		{
			name:    "no parsable body",
			code:    "ProvisioningState/failed/0",
			message: `Enable failed with no parsable body`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := armcompute.InstanceViewStatus{
				Code:    to.Ptr(tt.code),
				Message: to.Ptr(tt.message),
			}
			cseStatus, err := parseLinuxCSEMessage(status)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, cseStatus)
			require.Equal(t, tt.wantExitCode, cseStatus.ExitCode)
		})
	}
}
