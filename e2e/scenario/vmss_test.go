package scenario

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
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
}

func (l *vmssCreationTestLogger) Logf(format string, args ...any) {
	l.messages = append(l.messages, fmt.Sprintf(format, args...))
	l.T.Logf(format, args...)
}

func (f vmssCreationTestPolicy) Do(req *policy.Request) (*http.Response, error) {
	return f(req.Raw()), nil
}

type vmssCreationTestCase struct {
	name             string
	provisionCode    string
	running          bool
	sshFails         bool
	skipSSH          bool
	viewFails        bool
	noVM             bool
	noNetwork        bool
	listFails        bool
	noNIC            bool
	vmAfterPoll      bool
	ipeMode          bool
	preexisting      bool
	beginFails       bool
	receiptReadFails bool
	foreignReceipt   bool
	created          bool
	creationToken    string
	deleted          bool
	deletePolled     bool
	pendingPolls     int
	pollCount        int
	wantErr          string
	wantSSH          bool
	polled           bool
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
		{name: "ACL transition provisioning fails without instance", provisionCode: "VMExtensionProvisioningError", noVM: true, ipeMode: true},
		{name: "ACL transition VM has no NIC", noNIC: true, wantErr: "no network interfaces found", ipeMode: true},
		{name: "creation succeeded but SSH failed", running: true, sshFails: true, wantSSH: true, ipeMode: true},
		{name: "creation and SSH succeeded", running: true, wantSSH: true},
		{name: "network available before creation completes", running: true, pendingPolls: 2, wantSSH: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.ipeMode {
				t.Setenv(aclIPEModeEnv, "off")
				t.Setenv(aclIPETransitionEnv, "off-to-audit")
				previousSubscription, previousKeep := config.Config.SubscriptionID, config.Config.KeepVMSS
				config.Config.SubscriptionID, config.Config.KeepVMSS = "test", false
				t.Cleanup(func() {
					config.Config.SubscriptionID, config.Config.KeepVMSS = previousSubscription, previousKeep
				})
			}
			synctest.Test(t, func(t *testing.T) {
				previousAzure := config.Azure
				config.Azure = tt.client(t)
				t.Cleanup(func() { config.Azure = previousAzure })
				s := &Scenario{
					// Avoid unrelated Linux boot-log collection while exercising the real cleanup callback.
					Config: Config{SkipSSHConnectivityValidation: tt.skipSSH,
						VHD: &config.Image{OS: config.OSWindows}},
					cleanup: &scenarioCleanup{},
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
				if tt.ipeMode {
					s.Name = "ACL"
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
				model := armcompute.VirtualMachineScaleSet{}
				if tt.ipeMode {
					model.Tags = map[string]*string{"owner": to.Ptr("e2e")}
				}
				vm, err := createVMSS(logging.WithLogger(ctx, logger), s, "rg", model, dialSSH)
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
				if tt.ipeMode {
					require.NotNil(t, vm.ipeCreationReceipt, "SSH failure must preserve verified ARM creation ownership")
					require.NotNil(t, vm.VMSS)
					require.Equal(t, *vm.VMSS.ID, vm.ipeCreationReceipt.resourceID)
					require.Equal(t, tt.creationToken, vm.ipeCreationReceipt.token)
					s.Runtime.VM = vm
					registerVMSSCleanup(s, vm)
					require.Equal(t, aclIPETransitionCleanTime, s.cleanup.timeout)
					require.NoError(t, s.cleanup.runCleanups(logging.WithLogger(ctx, logger)))
					require.True(t, tt.deleted, "owned teardown must dispatch ARM DELETE even when SSH failed")
					require.True(t, tt.deletePolled, "owned teardown must await ARM DELETE")
				}
			})
		})
	}
}

func TestACLIPECreationOwnershipOnAmbiguousFailures(t *testing.T) {
	for _, tc := range []struct {
		name, wantError                                                      string
		preexisting, beginFails, receiptReadFails, foreignReceipt, wantOwned bool
	}{
		{name: "preexisting VMSS", preexisting: true, wantError: "already exists"},
		{name: "ambiguous create recovered", beginFails: true, wantOwned: true, wantError: "ambiguous create response"},
		{name: "ambiguous create cannot read", beginFails: true, receiptReadFails: true, wantError: "cannot verify ACL IPE VMSS creation ownership"},
		{name: "ambiguous create foreign marker", beginFails: true, foreignReceipt: true, wantError: "refuse ownership of ambiguous ACL IPE VMSS"},
		{name: "successful create cannot read", receiptReadFails: true, wantError: "cannot verify ACL IPE VMSS creation ownership"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(aclIPEModeEnv, "off")
			t.Setenv(aclIPETransitionEnv, "off-to-audit")
			oldAzure, oldSub, oldKeep := config.Azure, config.Config.SubscriptionID, config.Config.KeepVMSS
			t.Cleanup(func() {
				config.Azure, config.Config.SubscriptionID, config.Config.KeepVMSS = oldAzure, oldSub, oldKeep
			})
			config.Config.SubscriptionID, config.Config.KeepVMSS = "test", false
			tt := &vmssCreationTestCase{
				ipeMode: true, preexisting: tc.preexisting, beginFails: tc.beginFails,
				receiptReadFails: tc.receiptReadFails, foreignReceipt: tc.foreignReceipt,
			}
			config.Azure = tt.client(t)
			s := &Scenario{
				Name: "ACL", Config: Config{VHD: &config.Image{OS: config.OSWindows}}, cleanup: &scenarioCleanup{},
				Runtime: &ScenarioRuntime{VMSSName: "vmss", Cluster: &Cluster{
					Model: &armcontainerservice.ManagedCluster{
						Location:   to.Ptr("southeastasia"),
						Properties: &armcontainerservice.ManagedClusterProperties{NodeResourceGroup: to.Ptr("rg")},
					},
				}},
			}
			model := armcompute.VirtualMachineScaleSet{Tags: map[string]*string{"owner": to.Ptr("e2e")}}
			vm, err := createVMSS(t.Context(), s, "rg", model,
				func(context.Context, *Bastion, string, []byte) (*SSHClient, error) {
					t.Fatal("must not dial SSH after failed VMSS creation")
					return nil, nil
				})
			require.ErrorContains(t, err, tc.wantError)
			require.NotNil(t, vm)
			require.Equal(t, tc.wantOwned, vm.ipeCreationReceipt != nil)
			s.Runtime.VM = vm
			registerVMSSCleanup(s, vm)
			cleanupErr := s.cleanup.runCleanups(t.Context())
			if tc.wantOwned {
				require.NoError(t, cleanupErr)
				require.True(t, tt.deleted, "verified creation receipt must dispatch awaited owned deletion")
				require.True(t, tt.deletePolled)
			} else {
				require.False(t, tt.deleted, "unknown or foreign VMSS must not be deleted")
				if tt.created {
					require.ErrorContains(t, cleanupErr, "ownership unverified")
				} else {
					require.NoError(t, cleanupErr)
				}
			}
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
		if tt.ipeMode {
			require.Equal(t, "*", req.Header.Get("If-None-Match"), "create-only write must not overwrite a competing VMSS")
			var submitted armcompute.VirtualMachineScaleSet
			require.NoError(t, json.NewDecoder(req.Body).Decode(&submitted))
			require.NotNil(t, submitted.Tags[aclIPECreationTag])
			tt.creationToken = *submitted.Tags[aclIPECreationTag]
			tt.created = true
		}
		status = http.StatusCreated
		header.Set("Azure-AsyncOperation", "https://management.azure.com/operations/create")
		body = `{"properties":{"provisioningState":"Creating"}}`
		if tt.beginFails {
			status = http.StatusConflict
			body = `{"error":{"code":"Conflict","message":"ambiguous create response"}}`
		}
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
	case req.Method == http.MethodGet && req.URL.Path == "/operations/delete":
		tt.deletePolled = true
		body = `{"status":"Succeeded"}`
	case req.Method == http.MethodGet && req.URL.Path == vmssPath:
		body = fmt.Sprintf(`{"id":%q,"properties":{"provisioningState":"Succeeded"}}`, vmssPath)
		if tt.ipeMode {
			switch {
			case (!tt.created && !tt.preexisting) || tt.deleted:
				status = http.StatusNotFound
				body = `{"error":{"code":"ResourceNotFound"}}`
			case tt.receiptReadFails:
				status = http.StatusForbidden
				body = `{"error":{"code":"AuthorizationFailed"}}`
			default:
				_, owned := aclIPEFixture()
				owned.ID, owned.Name = to.Ptr(vmssPath), to.Ptr("vmss")
				owned.Tags[aclIPECreationTag] = to.Ptr(tt.creationToken)
				if tt.foreignReceipt {
					owned.Tags[aclIPECreationTag] = to.Ptr("someone-else")
				}
				body = aclIPEModelJSON(t, owned)
			}
		}
	case req.Method == http.MethodDelete && req.URL.Path == vmssPath:
		require.True(t, tt.ipeMode, "legacy deletion must not be used in the transition test")
		tt.deleted = true
		status = http.StatusAccepted
		header.Set("Azure-AsyncOperation", "https://management.azure.com/operations/delete")
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
