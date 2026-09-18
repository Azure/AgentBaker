package scenario

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type vmssCreationTestPolicy func(*http.Request) *http.Response

type vmssCreationTestLogger struct {
	*testing.T
	messages []string
	sshAt    time.Time
}

func (l *vmssCreationTestLogger) Log(args ...any) {
	message := fmt.Sprint(args...)
	l.messages = append(l.messages, message)
	if strings.Contains(message, "SSH Instructions:") {
		l.sshAt = time.Now()
	}
	l.T.Log(args...)
}

func (l *vmssCreationTestLogger) Logf(format string, args ...any) {
	l.messages = append(l.messages, fmt.Sprintf(format, args...))
	l.T.Logf(format, args...)
}

func (f vmssCreationTestPolicy) Do(req *policy.Request) (*http.Response, error) {
	return f(req.Raw()), nil
}

type vmssCreationTestCase struct {
	name          string
	provisionCode string
	running       bool
	sshFails      bool
	skipSSH       bool
	viewFails     bool
	noVM          bool
	noNetwork     bool
	listFails     bool
	noNIC         bool
	vmAfterPoll   bool
	pendingPolls  int
	pollCount     int
	vmLookups     int
	wantErr       string
	wantSSH       bool
	polled        bool
	started       time.Time
	pollTimes     []time.Duration
	retryAfter    string
	readyAfter    time.Duration
	vmReadyAfter  time.Duration
	networkAt     time.Duration
}

func TestCreateVMSSPollingKeepsEarlySSHInstructions(t *testing.T) {
	previousInterval := config.Config.DefaultPollInterval
	previousSubscription := config.Config.SubscriptionID
	previousKeyFile := config.VMSSHPrivateKeyFileName
	config.Config.DefaultPollInterval = 15 * time.Second
	config.Config.SubscriptionID = "00000000-0000-0000-0000-000000000001"
	config.VMSSHPrivateKeyFileName = "/tmp/e2e keys/id_rsa"
	t.Cleanup(func() {
		config.Config.DefaultPollInterval = previousInterval
		config.Config.SubscriptionID = previousSubscription
		config.VMSSHPrivateKeyFileName = previousKeyFile
	})
	var sshCommand string
	for _, tc := range []struct {
		name         string
		retryAfter   string
		readyAfter   time.Duration
		vmReadyAfter time.Duration
		wantPolls    []time.Duration
		wantDuration time.Duration
	}{
		{"initial Retry-After", "30", 25 * time.Second, 15 * time.Second, []time.Duration{30 * time.Second}, 30 * time.Second},
		{"subsequent Retry-After", "30", 65 * time.Second, 15 * time.Second, []time.Duration{30 * time.Second, 60 * time.Second, 90 * time.Second}, 90 * time.Second},
		{"configured fallback", "", 25 * time.Second, 15 * time.Second, []time.Duration{0, 15 * time.Second, 30 * time.Second}, 30 * time.Second},
		{"VM appears after completion", "30", 25 * time.Second, 45 * time.Second, []time.Duration{30 * time.Second}, 45 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				tt := vmssCreationTestCase{
					running: true, started: time.Now(), retryAfter: tc.retryAfter,
					readyAfter: tc.readyAfter, vmReadyAfter: tc.vmReadyAfter,
				}
				oldAzure := config.Azure
				config.Azure = tt.client(t)
				t.Cleanup(func() { config.Azure = oldAzure })
				s := &Scenario{
					Config: Config{SkipSSHConnectivityValidation: true},
					Runtime: &ScenarioRuntime{
						VMSSName: "vmss",
						Cluster: &Cluster{Model: &armcontainerservice.ManagedCluster{
							Location: to.Ptr("southeastasia"),
							Properties: &armcontainerservice.ManagedClusterProperties{
								NodeResourceGroup: to.Ptr("rg"),
							},
						}},
					},
				}
				logger := &vmssCreationTestLogger{T: t}
				ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
				defer cancel()
				_, err := createVMSS(logging.WithLogger(ctx, logger), s, "rg", armcompute.VirtualMachineScaleSet{}, nil)
				require.NoError(t, err)
				assert.Equal(t, tc.wantPolls, tt.pollTimes)
				assert.Equal(t, tc.wantDuration, time.Since(tt.started))
				assert.Equal(t, tc.wantDuration, tt.networkAt)
				assert.Equal(t, time.Duration(0), logger.sshAt.Sub(tt.started))
				for _, message := range logger.messages {
					if strings.HasPrefix(message, "SSH Instructions:") {
						_, sshCommand, _ = strings.Cut(message, "========================\n")
					}
				}
			})
		})
	}
	require.NotEmpty(t, sshCommand)
	for _, lookupFails := range []bool{false, true} {
		t.Run(fmt.Sprintf("SSH command/lookupFails=%t", lookupFails), func(t *testing.T) {
			script := fmt.Sprintf(`
az() {
    printf '%%s\n' "$@" >&2
    if [ "$1 $2" = "vmss list-instances" ]; then
        if [ %t = true ]; then
            return 7
        fi
        printf '%%s\n' '/subscriptions/test/resourceGroups/rg/providers/Microsoft.Compute/virtualMachineScaleSets/vmss/virtualMachines/7'
    fi
}
`, lookupFails) + sshCommand
			output, err := exec.CommandContext(t.Context(), "bash", "-c", script).CombinedOutput()
			want := []string{
				"vmss", "list-instances", "--subscription", config.Config.SubscriptionID,
				"--resource-group", "rg", "--name", "vmss", "--query", "[0].id", "--output", "tsv",
			}
			if lookupFails {
				var exitErr *exec.ExitError
				require.ErrorAs(t, err, &exitErr)
				assert.Equal(t, 7, exitErr.ExitCode())
			} else {
				require.NoError(t, err, "%s", output)
				want = append(want,
					"network", "bastion", "ssh", "--subscription", config.Config.SubscriptionID,
					"--target-resource-id", "/subscriptions/test/resourceGroups/rg/providers/Microsoft.Compute/virtualMachineScaleSets/vmss/virtualMachines/7",
					"--name", SharedBastionName, "--resource-group", config.ResourceGroupName("southeastasia"),
					"--auth-type", "ssh-key", "--username", "azureuser", "--ssh-key", config.VMSSHPrivateKeyFileName)
			}
			assert.Equal(t, want, strings.Split(strings.TrimSuffix(string(output), "\n"), "\n"))
		})
	}
}

func TestCreateVMSSProvisioningErrorPrecedence(t *testing.T) {
	for _, tt := range []vmssCreationTestCase{
		{name: "allocation failed", provisionCode: "AllocationFailed", sshFails: true},
		{name: "allocation failed with running guest and SSH failure", provisionCode: "AllocationFailed", running: true, sshFails: true, wantSSH: true},
		{name: "allocation failed without VM", provisionCode: "AllocationFailed", noVM: true},
		{name: "allocation failed without network profile", provisionCode: "AllocationFailed", noNetwork: true},
		{name: "allocation failed while instance listing fails", provisionCode: "AllocationFailed", listFails: true},
		{name: "allocation failed without NIC", provisionCode: "AllocationFailed", noNIC: true},
		{name: "allocation fails after pending without VM", provisionCode: "AllocationFailed", noVM: true, pendingPolls: 2},
		{name: "VM appears while creation is pending", running: true, vmAfterPoll: true, pendingPolls: 2, wantSSH: true},
		{name: "creation completes before VM appears", running: true, vmAfterPoll: true, wantSSH: true},
		{name: "deadline while creation is pending without VM", noVM: true, pendingPolls: 100, wantErr: "context deadline exceeded"},
		{name: "creation succeeded without VM", noVM: true, wantErr: "timeout waiting for VMSS VM"},
		{name: "successful creation without NIC", noNIC: true, wantErr: "no network interfaces found"},
		{name: "wrapped allocation failed", provisionCode: "ResourceOperationFailure", sshFails: true},
		{name: "OS provisioning failed without running guest", provisionCode: "OSProvisioningTimedOut", sshFails: true},
		{name: "CSE failed with running guest and SSH failure", provisionCode: "VMExtensionProvisioningError", running: true, sshFails: true, wantSSH: true},
		{name: "CSE failed with guest diagnostics available", provisionCode: "VMExtensionProvisioningError", running: true, wantSSH: true},
		{name: "CSE failed with SSH validation disabled", provisionCode: "VMExtensionProvisioningError", running: true, skipSSH: true},
		{name: "instance view unavailable", provisionCode: "AllocationFailed", viewFails: true, sshFails: true},
		{name: "creation succeeded but SSH failed", running: true, sshFails: true, wantSSH: true},
		{name: "creation and SSH succeeded", running: true, wantSSH: true},
		{name: "network available before creation completes", running: true, pendingPolls: 2, wantSSH: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				previousAzure := config.Azure
				config.Azure = tt.client(t)
				t.Cleanup(func() { config.Azure = previousAzure })
				s := &Scenario{
					Config: Config{SkipSSHConnectivityValidation: tt.skipSSH},
					Runtime: &ScenarioRuntime{
						VMSSName: "vmss",
						Cluster: &Cluster{Model: &armcontainerservice.ManagedCluster{
							Location: to.Ptr("southeastasia"),
							Properties: &armcontainerservice.ManagedClusterProperties{
								NodeResourceGroup: to.Ptr("rg"),
							},
						}},
					},
				}
				sshErr := errors.New("SSH handshake failed")
				sshClient := &SSHClient{}
				sshCalled := false
				logger := &vmssCreationTestLogger{T: t}
				dialSSH := func(_ context.Context, _ *Bastion, ip string, _ []byte) (*SSHClient, error) {
					require.True(t, tt.polled)
					require.Equal(t, "10.0.0.4", ip)
					if tt.provisionCode != "" {
						require.Contains(t, strings.Join(logger.messages, "\n"), "VMSS vmss provisioning failed:")
					}
					sshCalled = true
					if tt.sshFails {
						return nil, sshErr
					}
					return sshClient, nil
				}
				ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
				defer cancel()
				started := time.Now()
				vm, err := createVMSS(logging.WithLogger(ctx, logger), s, "rg", armcompute.VirtualMachineScaleSet{}, dialSSH)
				require.NotNil(t, vm)
				assert.Equal(t, tt.wantSSH, sshCalled)
				if tt.provisionCode != "" {
					var responseErr *azcore.ResponseError
					require.ErrorAs(t, err, &responseErr, "provisioning error must survive discovery or SSH failure")
					assert.Equal(t, tt.provisionCode, responseErr.ErrorCode)
					assert.Equal(t, tt.provisionCode == "AllocationFailed", isRetryableVMSSCreationError(fmt.Errorf("create VMSS: %w", err)))
					assert.NotContains(t, strings.Join(logger.messages, "\n"), "after creation")
					assert.Less(t, time.Since(started), time.Minute, "failed provisioning must not wait for the discovery deadline")
					assert.Equal(t, 1, tt.vmLookups, "failed provisioning gets one lookup for diagnostics")
				} else if tt.wantErr != "" {
					require.ErrorContains(t, err, tt.wantErr)
					if tt.wantErr == context.DeadlineExceeded.Error() {
						assert.ErrorIs(t, err, context.DeadlineExceeded)
					}
				} else if !tt.sshFails {
					require.NoError(t, err)
					require.NotNil(t, vm.VMSS)
				}
				require.True(t, tt.polled, "must exercise the actual SDK provisioning result")
				if tt.wantSSH && tt.sshFails {
					assert.ErrorIs(t, err, sshErr)
					assert.ErrorContains(t, err, "failed to start bastion tunnel:")
					if tt.provisionCode == "" {
						assert.False(t, isRetryableVMSSCreationError(err))
					}
				}
				if tt.wantSSH && !tt.sshFails {
					assert.Same(t, sshClient, vm.SSHClient, "retain the connection for guest diagnostics")
				}
			})
		})
	}
}

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
		{code: "OperationNotAllowed", status: 500, message: "exceeding approved quota"},
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
				var err error = armErr
				if sshErr != nil {
					err = errors.Join(err, fmt.Errorf("failed to start bastion tunnel: %w", sshErr))
				}
				err = fmt.Errorf("create VMSS: %w", err)
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
	for _, err := range []error{nil, fmt.Errorf("failed to start bastion tunnel: %w", context.DeadlineExceeded)} {
		require.False(t, isRetryableVMSSCreationError(err))
		for _, enabled := range []bool{false, true} {
			config.Config.SkipTestsWithSKUCapacityIssue = enabled
			require.NoError(t, skipIfSKUNotAvailableErr(err))
		}
	}
}

func (tt *vmssCreationTestCase) client(t *testing.T) *config.AzureClient {
	t.Helper()
	respond := vmssCreationTestPolicy(func(req *http.Request) *http.Response { return tt.respond(t, req) })
	options := &arm.ClientOptions{ClientOptions: policy.ClientOptions{
		PerCallPolicies: []policy.Policy{respond},
	}}
	client := &config.AzureClient{}
	var err error
	client.VMSS, err = armcompute.NewVirtualMachineScaleSetsClient("test", nil, options)
	require.NoError(t, err)
	client.VMSSVM, err = armcompute.NewVirtualMachineScaleSetVMsClient("test", nil, options)
	require.NoError(t, err)
	client.NetworkInterfaces, err = armnetwork.NewInterfacesClient("test", nil, options)
	require.NoError(t, err)
	return client
}

func (tt *vmssCreationTestCase) respond(t *testing.T, req *http.Request) *http.Response {
	t.Helper()
	const vmssPath = "/subscriptions/test/resourceGroups/rg/providers/Microsoft.Compute/virtualMachineScaleSets/vmss"
	const vmPath = vmssPath + "/virtualMachines/0"
	vmBody := func(running bool) string {
		powerState := "deallocated"
		if running {
			powerState = "running"
		}
		return fmt.Sprintf(`{"id":%q,"instanceId":"0","properties":{"networkProfile":{},
			"instanceView":{"statuses":[{"code":"PowerState/%s"}]}}}`, vmPath, powerState)
	}
	status := http.StatusOK
	header := http.Header{"Content-Type": {"application/json"}}
	var body string
	switch {
	case req.Method == http.MethodPut && req.URL.Path == vmssPath:
		status = http.StatusCreated
		header.Set("Azure-AsyncOperation", "https://management.azure.com/operations/create")
		if tt.retryAfter != "" {
			header.Set("Retry-After", tt.retryAfter)
		}
		body = `{"properties":{"provisioningState":"Creating"}}`
	case req.Method == http.MethodGet && req.URL.Path == "/operations/create":
		tt.polled = true
		tt.pollCount++
		if !tt.started.IsZero() {
			tt.pollTimes = append(tt.pollTimes, time.Since(tt.started))
		}
		if tt.retryAfter != "" {
			header.Set("Retry-After", tt.retryAfter)
		}
		body = `{"status":"Succeeded"}`
		if tt.provisionCode != "" {
			details := ""
			if tt.provisionCode == "ResourceOperationFailure" {
				details = `,"details":[{"code":"AllocationFailed","message":"insufficient capacity"}]`
			}
			body = fmt.Sprintf(`{"status":"Failed","error":{"code":%q,"message":"provisioning failed",
				"target":"vmss"%s}}`, tt.provisionCode, details)
		}
		if tt.pollCount <= tt.pendingPolls || (tt.readyAfter > 0 && time.Since(tt.started) < tt.readyAfter) {
			body = `{"status":"InProgress"}`
		}
	case req.Method == http.MethodGet && req.URL.Path == vmssPath:
		body = fmt.Sprintf(`{"id":%q,"properties":{"provisioningState":"Succeeded"}}`, vmssPath)
	case req.Method == http.MethodGet && req.URL.Path == vmssPath+"/virtualMachines":
		tt.vmLookups++
		body = `{"value":[` + vmBody(true) + `]}`
		if tt.noVM || (tt.vmAfterPoll && !tt.polled) || (tt.vmReadyAfter > 0 && time.Since(tt.started) < tt.vmReadyAfter) {
			body = `{"value":[]}`
		}
		if tt.noNetwork {
			body = fmt.Sprintf(`{"value":[{"id":%q,"instanceId":"0","properties":{}}]}`, vmPath)
		}
		if tt.listFails {
			status = http.StatusNotFound
			body = `{"error":{"code":"ResourceNotFound","message":"VMSS not found"}}`
		}
	case req.Method == http.MethodGet && strings.EqualFold(req.URL.Path, vmPath+"/networkInterfaces"):
		if !tt.started.IsZero() {
			tt.networkAt = time.Since(tt.started)
		}
		body = `{"value":[{"properties":{"ipConfigurations":[{"properties":{"privateIPAddress":"10.0.0.4"}}]}}]}`
		if tt.noNIC {
			body = `{"value":[]}`
		}
	case req.Method == http.MethodGet && req.URL.Path == vmPath:
		require.True(t, tt.polled)
		require.Equal(t, "instanceView", req.URL.Query().Get("$expand"))
		body = vmBody(tt.running)
		if tt.viewFails {
			status = http.StatusNotFound
			body = `{"error":{"code":"ResourceNotFound","message":"VM not found"}}`
		}
	default:
		t.Fatalf("unexpected Azure request: %s %s", req.Method, req.URL)
	}
	return &http.Response{
		StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader(body)), Request: req,
	}
}

func TestWriteScriptHotfixFixture(t *testing.T) {
	buildDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(buildDir, "generated"), 0o755))
	fixture := ScriptHotfixFixture{
		Platform: "ubuntu",
		Files: []ScriptHotfixFile{{
			Destination: "/opt/azure/containers/provision_configs.sh",
			Mode:        "0744",
			Payload:     []byte("#!/bin/bash\necho e2e\n"),
		}},
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
	require.Equal(t, fixture.Files[0].Destination, rendered.WriteFiles[0].Path)
	require.Equal(t, fixture.Files[0].Mode, rendered.WriteFiles[0].Permissions)
	require.Equal(t, "base64", rendered.WriteFiles[0].Encoding)
	payload, err := base64.StdEncoding.DecodeString(rendered.WriteFiles[0].Content)
	require.NoError(t, err)
	require.Equal(t, fixture.Files[0].Payload, payload)
}

func TestWriteScriptHotfixFixtureRejectsInvalidData(t *testing.T) {
	validFixture := func() ScriptHotfixFixture {
		return ScriptHotfixFixture{
			Platform: "ubuntu",
			Files: []ScriptHotfixFile{{
				Destination: "/opt/azure/containers/provision_configs.sh",
				Mode:        "0744",
				Payload:     []byte("#!/bin/bash\n"),
			}},
		}
	}
	tests := []struct {
		name   string
		mutate func(*ScriptHotfixFixture)
	}{
		{
			name: "relative destination",
			mutate: func(fixture *ScriptHotfixFixture) {
				fixture.Files[0].Destination = "opt/provision_configs.sh"
			},
		},
		{
			name: "invalid mode",
			mutate: func(fixture *ScriptHotfixFixture) {
				fixture.Files[0].Mode = "0999"
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
				fixture.Files[0].Payload = nil
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := validFixture()
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
		Runtime: &ScenarioRuntime{VMSize: config.DEFAULT_VMSKU},
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
