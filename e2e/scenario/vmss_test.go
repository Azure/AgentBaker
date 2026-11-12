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
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v8"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type vmssCreationTestPolicy func(*http.Request) *http.Response

func TestCustomDataWithScriptBundle(t *testing.T) {
	const preload = "nohup /bin/bash /opt/azure/containers/provision_preload.sh >/dev/null 2>&1 &"
	original := "#!/bin/bash\nset -euo pipefail\n" + preload + "\n" + hotfixMarker + "\nstart-anc\n"
	input := base64.StdEncoding.EncodeToString([]byte(original))
	encoded, err := CustomDataWithScriptBundle(input, "https://example.invalid/anc?sig=test")
	require.NoError(t, err)
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	require.NoError(t, err)
	actual := string(decoded)
	require.NotContains(t, actual, hotfixMarker)
	require.Equal(t, 1, strings.Count(actual, "curl -fSL"))
	require.Equal(t, 1, strings.Count(actual, "unpack-scripts"))
	require.Contains(t, actual, "&& /opt/azure/containers/aks-node-controller-hotfix unpack-scripts")
	require.Contains(t, actual, "unpack-scripts || exit 1")
	require.Less(t, strings.Index(actual, "unpack-scripts"), strings.Index(actual, "start-anc"))
	require.Less(t, strings.Index(actual, "unpack-scripts"), strings.Index(actual, preload))

	encoded, err = CustomDataWithNBCCmdHack(input, "https://example.invalid/anc")
	require.NoError(t, err)
	decoded, err = base64.StdEncoding.DecodeString(encoded)
	require.NoError(t, err)
	require.NotContains(t, string(decoded), "unpack-scripts", "hotfix fixtures must let the launcher apply the embedded tree")
	_, err = CustomDataWithScriptBundle("not base64", "unused")
	require.ErrorContains(t, err, "decode custom data")
	_, err = CustomDataWithScriptBundle(base64.StdEncoding.EncodeToString([]byte("no marker")), "unused")
	require.ErrorContains(t, err, "not found")
	_, err = CustomDataWithScriptBundle(base64.StdEncoding.EncodeToString([]byte(hotfixMarker)), "unused")
	require.ErrorContains(t, err, "preload command not found")
}

func TestScriptBundleFailureStopsProvisioning(t *testing.T) {
	bash, err := exec.LookPath("bash")
	require.NoError(t, err)
	input := base64.StdEncoding.EncodeToString([]byte("#!/bin/bash\nset -euo pipefail\nnohup /bin/bash /opt/azure/containers/provision_preload.sh &\n" + hotfixMarker + "\nstart-anc\n"))
	encoded, err := CustomDataWithScriptBundle(input, "https://example.invalid/anc")
	require.NoError(t, err)
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	require.NoError(t, err)
	script := strings.ReplaceAll(string(decoded), "/opt/azure/containers/aks-node-controller-hotfix", "test-anc")
	for _, failure := range []string{"download", "unpack"} {
		t.Run(failure, func(t *testing.T) {
			downloadCode, unpackCode := 0, 0
			if failure == "download" {
				downloadCode = 23
			} else {
				unpackCode = 1
			}
			mocks := fmt.Sprintf("curl() { return %d; }; chmod() { :; }; test-anc() { return %d; }; start-anc() { echo started; };\n", downloadCode, unpackCode)
			output, err := exec.Command(bash, "-c", mocks+script).CombinedOutput()
			require.Error(t, err)
			require.NotContains(t, string(output), "started")
		})
	}
}

type vmssCreationTestLogger struct {
	*testing.T
	messages []string
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
	wantErr       string
	wantSSH       bool
	polled        bool
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
		{name: "deadline while creation is pending without VM", noVM: true, pendingPolls: 100, wantErr: "timeout waiting for VMSS VM"},
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
				vm, err := createVMSS(logging.WithLogger(ctx, logger), s, "rg", armcompute.VirtualMachineScaleSet{}, dialSSH)
				require.NotNil(t, vm)
				assert.Equal(t, tt.wantSSH, sshCalled)
				if tt.provisionCode != "" {
					var responseErr *azcore.ResponseError
					require.ErrorAs(t, err, &responseErr, "provisioning error must survive discovery or SSH failure")
					assert.Equal(t, tt.provisionCode, responseErr.ErrorCode)
					assert.Equal(t, tt.provisionCode == "AllocationFailed", isRetryableVMSSCreationError(fmt.Errorf("create VMSS: %w", err)))
					assert.NotContains(t, strings.Join(logger.messages, "\n"), "after creation")
				} else if tt.wantErr != "" {
					require.ErrorContains(t, err, tt.wantErr)
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
		// capacityScoped skips only when the scenario opts in via SkipOnCapacityError
		// (AllocationFailed). SkuNotAvailable/quota skip for any scenario.
		capacityScoped bool
	}{
		{code: "AllocationFailed", status: 200, wantRetry: true, wantSkip: true, capacityScoped: true},
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
					for _, scenarioSkipsOnCapacity := range []bool{false, true} {
						config.Config.SkipTestsWithSKUCapacityIssue = skipEnabled
						// AllocationFailed only skips when the scenario opts in (capacityScoped);
						// SkuNotAvailable/quota skip for any scenario. Both require the global flag.
						wantSkip := tc.wantSkip && skipEnabled && (!tc.capacityScoped || scenarioSkipsOnCapacity)
						skipErr := skipIfSKUNotAvailableErr(err, scenarioSkipsOnCapacity)
						if wantSkip {
							var skipped *skipError
							require.ErrorAs(t, skipErr, &skipped)
						} else {
							require.NoError(t, skipErr)
						}
					}
				}
			}
		})
	}
	for _, err := range []error{nil, fmt.Errorf("failed to start bastion tunnel: %w", context.DeadlineExceeded)} {
		require.False(t, isRetryableVMSSCreationError(err))
		for _, enabled := range []bool{false, true} {
			for _, scenarioSkipsOnCapacity := range []bool{false, true} {
				config.Config.SkipTestsWithSKUCapacityIssue = enabled
				require.NoError(t, skipIfSKUNotAvailableErr(err, scenarioSkipsOnCapacity))
			}
		}
	}
}

// TestShouldSurfaceCapacityError: the retry-loop guard fires only when the scenario opts in,
// the global flag is on, and the error is AllocationFailed.
func TestShouldSurfaceCapacityError(t *testing.T) {
	oldSkip := config.Config.SkipTestsWithSKUCapacityIssue
	t.Cleanup(func() { config.Config.SkipTestsWithSKUCapacityIssue = oldSkip })

	allocErr := fmt.Errorf("failed to wait for VMSS VM: %w", &azcore.ResponseError{StatusCode: 200, ErrorCode: "AllocationFailed"})
	quotaErr := &azcore.ResponseError{StatusCode: 409, ErrorCode: "SkuNotAvailable"}

	for _, tc := range []struct {
		name       string
		optIn      bool
		globalFlag bool
		err        error
		want       bool
	}{
		{name: "opted-in, flag on, AllocationFailed", optIn: true, globalFlag: true, err: allocErr, want: true},
		{name: "opted-in, flag on, non-allocation error", optIn: true, globalFlag: true, err: quotaErr, want: false},
		{name: "opted-in, flag off, AllocationFailed", optIn: true, globalFlag: false, err: allocErr, want: false},
		{name: "not opted-in, flag on, AllocationFailed", optIn: false, globalFlag: true, err: allocErr, want: false},
		{name: "opted-in, flag on, nil error", optIn: true, globalFlag: true, err: nil, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config.Config.SkipTestsWithSKUCapacityIssue = tc.globalFlag
			s := &Scenario{Config: Config{SkipOnCapacityError: tc.optIn}}
			require.Equal(t, tc.want, shouldSurfaceCapacityError(s, tc.err))
		})
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
		body = `{"properties":{"provisioningState":"Creating"}}`
	case req.Method == http.MethodGet && req.URL.Path == "/operations/create":
		tt.polled = true
		tt.pollCount++
		body = `{"status":"Succeeded"}`
		if tt.provisionCode != "" {
			details := ""
			if tt.provisionCode == "ResourceOperationFailure" {
				details = `,"details":[{"code":"AllocationFailed","message":"insufficient capacity"}]`
			}
			body = fmt.Sprintf(`{"status":"Failed","error":{"code":%q,"message":"provisioning failed",
				"target":"vmss"%s}}`, tt.provisionCode, details)
		}
		if tt.pollCount <= tt.pendingPolls {
			body = `{"status":"InProgress"}`
		}
	case req.Method == http.MethodGet && req.URL.Path == vmssPath:
		body = fmt.Sprintf(`{"id":%q,"properties":{"provisioningState":"Succeeded"}}`, vmssPath)
	case req.Method == http.MethodGet && req.URL.Path == vmssPath+"/virtualMachines":
		body = `{"value":[` + vmBody(true) + `]}`
		if tt.noVM || (tt.vmAfterPoll && !tt.polled) {
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
		if !tt.vmAfterPoll {
			require.False(t, tt.polled, "discover available networking before waiting for creation")
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
	root := t.TempDir()
	target := filepath.Join(root, "parts/root/ubuntu/opt/azure/containers/provision_configs.sh")
	require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o755))
	require.NoError(t, os.WriteFile(target+".link", []byte("shared/provision_configs.sh\n"), 0o644))
	fixture := ScriptHotfixFixture{Platform: "ubuntu", Files: []ScriptHotfixFile{{Destination: "/opt/azure/containers/provision_configs.sh", Payload: []byte("#!/bin/bash\necho e2e\n")}}}
	require.NoError(t, writeScriptHotfixFixture(root, fixture))
	require.NoFileExists(t, target+".link")
	data, err := os.ReadFile(filepath.Join(root, "parts/root/ubuntu/opt/azure/containers/provision_configs.sh"))
	require.NoError(t, err)
	require.Equal(t, fixture.Files[0].Payload, data)
}

func TestWriteScriptHotfixFixtureRejectsInvalidData(t *testing.T) {
	validFixture := func() ScriptHotfixFixture {
		return ScriptHotfixFixture{
			Platform: "ubuntu",
			Files: []ScriptHotfixFile{{
				Destination: "/opt/azure/containers/provision_configs.sh",
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
			require.Error(t, writeScriptHotfixFixture(buildDir, fixture))
		})
	}
}

// TestCSEExitCodeOutboundConnFail pins the exit code constant to the value emitted by
// ERR_OUTBOUND_CONN_FAIL in parts/root/common/opt/azure/containers/provision_source.sh. If the
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
