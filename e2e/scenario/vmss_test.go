package scenario

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestVMSSProvisioningErrorClassification(t *testing.T) {
	oldSkip := config.Config.SkipTestsWithSKUCapacityIssue
	t.Cleanup(func() { config.Config.SkipTestsWithSKUCapacityIssue = oldSkip })
	for _, tc := range []struct {
		code      string
		status    int
		message   string
		wantRetry bool
		wantSkip  bool
	}{
		{code: "AllocationFailed", status: 200, wantRetry: true},
		{code: "GalleryImageNotFound", status: 404, wantRetry: true},
		{code: "SkuNotAvailable", status: 409, wantSkip: true},
		{code: "OperationNotAllowed", status: 409, message: "exceeding approved quota", wantSkip: true},
		{code: "OperationNotAllowed", status: 409, message: "another operation is pending"},
		{code: "VMExtensionProvisioningError", status: 200},
		{code: "AllocationFailed", status: 500},
		{code: "GalleryImageNotFound", status: 500},
		{code: "SkuNotAvailable", status: 500},
	} {
		t.Run(fmt.Sprintf("%s/%d/%s", tc.code, tc.status, tc.message), func(t *testing.T) {
			armErr := &azcore.ResponseError{
				StatusCode: tc.status,
				ErrorCode:  tc.code,
				RawResponse: &http.Response{
					StatusCode: tc.status,
					Header:     http.Header{"Content-Type": {"application/json"}},
					Body:       io.NopCloser(strings.NewReader(fmt.Sprintf(`{"error":{"code":%q,"message":%q}}`, tc.code, tc.message))),
				},
			}
			for _, sshErr := range []error{nil, context.DeadlineExceeded} {
				err := fmt.Errorf("create VMSS: %w", joinProvisioningErrors(armErr, sshErr))
				var responseErr *azcore.ResponseError
				require.ErrorAs(t, err, &responseErr)
				require.Same(t, armErr, responseErr)
				require.Equal(t, tc.wantRetry, isRetryableVMSSCreationError(err))
				for _, skipEnabled := range []bool{false, true} {
					config.Config.SkipTestsWithSKUCapacityIssue = skipEnabled
					skipErr := skipIfSKUNotAvailableErr(err)
					if tc.wantSkip && skipEnabled {
						var skipped *skipError
						require.ErrorAs(t, skipErr, &skipped)
					} else {
						require.NoError(t, skipErr)
					}
				}
			}
		})
	}
	require.False(t, isRetryableVMSSCreationError(nil))
	require.False(t, isRetryableVMSSCreationError(joinProvisioningErrors(nil, context.DeadlineExceeded)))
}

func TestJoinProvisioningErrors(t *testing.T) {
	allocationErr := &azcore.ResponseError{StatusCode: 200, ErrorCode: "AllocationFailed"}
	cseErr := &azcore.ResponseError{StatusCode: 200, ErrorCode: "VMExtensionProvisioningError"}
	for _, tc := range []struct {
		name         string
		provisionErr error
		sshErr       error
	}{
		{name: "success"},
		{name: "SSH failure", sshErr: context.DeadlineExceeded},
		{name: "CSE failure with SSH success", provisionErr: cseErr},
		{name: "CSE and SSH failure", provisionErr: cseErr, sshErr: context.DeadlineExceeded},
		{name: "allocation and SSH failure", provisionErr: allocationErr, sshErr: context.DeadlineExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := joinProvisioningErrors(tc.provisionErr, tc.sshErr)
			if tc.provisionErr == nil && tc.sshErr == nil {
				require.NoError(t, err)
				return
			}
			if tc.provisionErr != nil {
				var responseErr *azcore.ResponseError
				require.ErrorAs(t, err, &responseErr)
				require.Same(t, tc.provisionErr, responseErr)
			}
			if tc.sshErr == nil {
				require.Same(t, tc.provisionErr, err)
			} else {
				require.ErrorIs(t, err, tc.sshErr)
				require.ErrorContains(t, err, "failed to start bastion tunnel:")
			}
			if tc.provisionErr != nil && tc.sshErr != nil {
				require.EqualError(t, err, tc.provisionErr.Error()+"\nfailed to start bastion tunnel: "+tc.sshErr.Error())
			}
		})
	}
}

func TestWriteScriptHotfixFixture(t *testing.T) {
	buildDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(buildDir, "generated"), 0o755))
	fixture := ScriptHotfixFixture{
		Platform:    "ubuntu",
		Destination: "/opt/azure/containers/provision_configs.sh",
		Mode:        "0744",
		Payload:     []byte("#!/bin/bash\necho e2e\n"),
	}

	require.NoError(t, writeScriptHotfixFixture(buildDir, fixture))
	entries, err := os.ReadDir(filepath.Join(buildDir, "generated"))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "rendered_nodecustomdata_ubuntu.yml", entries[0].Name())

	renderedData, err := os.ReadFile(filepath.Join(
		buildDir,
		"generated",
		"rendered_nodecustomdata_ubuntu.yml",
	))
	require.NoError(t, err)
	var rendered scriptHotfixFixtureNodeCustomData
	require.NoError(t, yaml.Unmarshal(renderedData, &rendered))
	require.Len(t, rendered.WriteFiles, 1)
	require.Equal(t, fixture.Destination, rendered.WriteFiles[0].Path)
	require.Equal(t, fixture.Mode, rendered.WriteFiles[0].Permissions)
	require.Equal(t, "base64", rendered.WriteFiles[0].Encoding)
	payload, err := base64.StdEncoding.DecodeString(rendered.WriteFiles[0].Content)
	require.NoError(t, err)
	require.Equal(t, fixture.Payload, payload)
}

func TestWriteScriptHotfixFixtureRejectsInvalidData(t *testing.T) {
	valid := ScriptHotfixFixture{
		Platform:    "ubuntu",
		Destination: "/opt/azure/containers/provision_configs.sh",
		Mode:        "0744",
		Payload:     []byte("#!/bin/bash\n"),
	}
	tests := []struct {
		name   string
		mutate func(*ScriptHotfixFixture)
	}{
		{
			name: "relative destination",
			mutate: func(fixture *ScriptHotfixFixture) {
				fixture.Destination = "opt/provision_configs.sh"
			},
		},
		{
			name: "invalid mode",
			mutate: func(fixture *ScriptHotfixFixture) {
				fixture.Mode = "0999"
			},
		},
		{
			name: "unsupported platform",
			mutate: func(fixture *ScriptHotfixFixture) {
				fixture.Platform = "other"
			},
		},
		{
			name: "empty payload",
			mutate: func(fixture *ScriptHotfixFixture) {
				fixture.Payload = nil
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := valid
			test.mutate(&fixture)
			buildDir := t.TempDir()
			require.NoError(t, os.MkdirAll(filepath.Join(buildDir, "generated"), 0o755))
			require.Error(t, writeScriptHotfixFixture(buildDir, fixture))
		})
	}
}

// TestCSEExitCodeOutboundConnFail pins the exit code constant to the value emitted by
// ERR_OUTBOUND_CONN_FAIL in parts/linux/cloud-init/artifacts/cse_helpers.sh. If the
// product error code changes, this test forces the harness mitigation to be updated.
func TestGetBaseVMSSModelUsesScenarioVMSize(t *testing.T) {
	s := &Scenario{
		Runtime: &ScenarioRuntime{VMSize: "Standard_D2ds_v5"},
	}
	assert.Equal(t, s.Runtime.VMSize, scenarioVMSize(s))
}

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
			// Real Ubuntu2204_HTTPSProxy_PrivateDNS/default failure: the outer extension
			// wrapper and the CSE status both report 50.
			name: "real outbound flake, outer exit 50 and cse exit 50",
			code: "ProvisioningState/failed/0",
			message: "failed to execute command: command terminated with exit status=50\n[stdout]\n" +
				`{ "ExitCode": "50", "Output": "Processing manual pages under /usr/local/man...\n++ date\n+ echo 'man-db finished updates'\n+ exit 50", "Error": "", "ExecDuration": "155", "BootDatapoints": { "KubeletStartTime": "n/a" } }` +
				"\n\n[stderr]\ndate: invalid date 'n/a'\n",
			wantExitCode: "50",
		},
		{
			// Real Ubuntu2204_HTTPSProxy_PrivateDNS/scriptless_nbc failure: the outer
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
