package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
	"github.com/stretchr/testify/require"
)

func TestWriteScriptHotfixFixture(t *testing.T) {
	buildDir := t.TempDir()
	fixture := ScriptHotfixFixture{
		Source:      "e2e/provision_configs.sh",
		Destination: "/opt/azure/containers/provision_configs.sh",
		Mode:        "0744",
		Platforms:   []string{"ubuntu"},
		Payload:     []byte("#!/bin/bash\necho e2e\n"),
	}

	require.NoError(t, writeScriptHotfixFixture(buildDir, fixture))

	payload, err := os.ReadFile(filepath.Join(
		buildDir,
		"scripthotfix",
		"generated",
		"payloads",
		"e2e",
		"provision_configs.sh",
	))
	require.NoError(t, err)
	require.Equal(t, fixture.Payload, payload)

	manifestData, err := os.ReadFile(filepath.Join(
		buildDir,
		"scripthotfix",
		"generated",
		"manifest.json",
	))
	require.NoError(t, err)
	var manifest scriptHotfixFixtureManifest
	require.NoError(t, json.Unmarshal(manifestData, &manifest))
	require.Equal(t, 1, manifest.SchemaVersion)
	require.Len(t, manifest.Entries, 1)
	require.Equal(t, fixture.Source, manifest.Entries[0].Source)
	require.Equal(t, "payloads/e2e/provision_configs.sh", manifest.Entries[0].Payload)
	require.Equal(t, fixture.Destination, manifest.Entries[0].Destination)
	require.Equal(t, fixture.Mode, manifest.Entries[0].Mode)
	require.Equal(t, fixture.Platforms, manifest.Entries[0].Platforms)
}

func TestWriteScriptHotfixFixtureRejectsInvalidManifestData(t *testing.T) {
	valid := ScriptHotfixFixture{
		Source:      "e2e/provision_configs.sh",
		Destination: "/opt/azure/containers/provision_configs.sh",
		Mode:        "0744",
		Platforms:   []string{"ubuntu"},
		Payload:     []byte("#!/bin/bash\n"),
	}
	tests := []struct {
		name   string
		mutate func(*ScriptHotfixFixture)
	}{
		{
			name: "unsafe source",
			mutate: func(fixture *ScriptHotfixFixture) {
				fixture.Source = "../provision_configs.sh"
			},
		},
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
				fixture.Platforms = []string{"other"}
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
			require.Error(t, writeScriptHotfixFixture(t.TempDir(), fixture))
		})
	}
}

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
