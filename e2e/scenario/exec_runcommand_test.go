package scenario

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v8"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/stretchr/testify/require"
)

func TestSSHDisabledAcrossRebootUsesARM(t *testing.T) {
	for _, test := range []struct {
		name                  string
		invalidBootID         bool
		unchangedBootID       bool
		failAfterReboot       bool
		kubeletNotReadyChecks int
		kubeletNeverReady     bool
		wantError             string
		wantRestarts          int
		wantSSHChecks         int
	}{
		{name: "same instance rebooted without SSH", wantRestarts: 1, wantSSHChecks: 2},
		{name: "invalid boot evidence prevents restart", invalidBootID: true, wantError: "invalid boot ID before", wantSSHChecks: 1},
		{name: "ARM success without a reboot is rejected", unchangedBootID: true, wantError: "wait for SSH-disabled VM reboot", wantRestarts: 1, wantSSHChecks: 1},
		{name: "guest validation fails after reboot", failAfterReboot: true, wantError: "RunCommand script failed", wantRestarts: 1, wantSSHChecks: 2},
		{name: "waits for kubelet to become active", kubeletNotReadyChecks: 2, wantRestarts: 1, wantSSHChecks: 2},
		{name: "kubelet readiness timeout blocks final SSH validation", kubeletNeverReady: true, wantError: "wait for kubelet to become active after SSH-disabled reboot", wantRestarts: 1, wantSSHChecks: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			restarts := 0
			kubeletChecks, sshChecks := 0, 0
			script := ""
			respond := vmssCreationTestPolicy(func(req *http.Request) *http.Response {
				require.Contains(t, req.URL.Path, "/virtualMachines/0/")
				switch req.Method {
				case http.MethodPost:
					require.True(t, strings.HasSuffix(req.URL.Path, "/restart"))
					restarts++
					return gcResponse(req, http.StatusOK, "")
				case http.MethodPut:
					var command armcompute.VirtualMachineRunCommand
					require.NoError(t, json.NewDecoder(req.Body).Decode(&command))
					script = *command.Properties.Source.Script
					return gcResponse(req, http.StatusOK, `{"properties":{"provisioningState":"Succeeded"}}`)
				case http.MethodGet:
					output := "SUCCESS: SSH units are disabled and stopped"
					state, exitCode := "Succeeded", 0
					switch script {
					case "cat /proc/sys/kernel/random/boot_id":
						output = "00000000-0000-4000-8000-000000000001"
						if test.invalidBootID {
							output = strings.Repeat("x", 36)
						} else if restarts > 0 && !test.unchangedBootID {
							output = "00000000-0000-4000-8000-000000000002"
						}
					case "systemctl is-active --quiet kubelet":
						kubeletChecks++
						if test.kubeletNeverReady || kubeletChecks <= test.kubeletNotReadyChecks {
							state, exitCode, output = "Failed", 3, "activating"
						}
					case validateSSHServiceDisabledScript:
						sshChecks++
						if restarts > 0 {
							require.Greater(t, kubeletChecks, test.kubeletNotReadyChecks)
						}
						if restarts > 0 && test.failAfterReboot {
							state, exitCode, output = "Failed", 1, "FAILED: sshd.service is active"
						}
					}
					return gcResponse(req, http.StatusOK, fmt.Sprintf(
						`{"properties":{"instanceView":{"executionState":%q,"exitCode":%d,"output":%q}}}`, state, exitCode, output))
				case http.MethodDelete:
					return gcResponse(req, http.StatusNoContent, "")
				default:
					t.Fatalf("unexpected VM request: %s %s", req.Method, req.URL)
					return nil
				}
			})
			options := &arm.ClientOptions{ClientOptions: policy.ClientOptions{PerCallPolicies: []policy.Policy{respond}}}
			runCommands, err := armcompute.NewVirtualMachineScaleSetVMRunCommandsClient("test", nil, options)
			require.NoError(t, err)
			vms, err := armcompute.NewVirtualMachineScaleSetVMsClient("test", nil, options)
			require.NoError(t, err)
			previousAzure := config.Azure
			config.Azure = &config.AzureClient{VMSSVMRunCommands: runCommands, VMSSVM: vms}
			t.Cleanup(func() { config.Azure = previousAzure })
			s := &Scenario{
				Config: Config{VHD: &config.Image{OS: config.OSUbuntu}, SkipSSHConnectivityValidation: true},
				Runtime: &ScenarioRuntime{
					VMSSName: "vmss",
					Cluster: &Cluster{Model: &armcontainerservice.ManagedCluster{
						Properties: &armcontainerservice.ManagedClusterProperties{NodeResourceGroup: to.Ptr("rg")},
					}},
					VM: &ScenarioVM{VM: &armcompute.VirtualMachineScaleSetVM{InstanceID: to.Ptr("0")}},
				},
			}
			synctest.Test(t, func(t *testing.T) {
				ctx := t.Context()
				if test.unchangedBootID {
					var cancel context.CancelFunc
					ctx, cancel = context.WithTimeout(ctx, 300*time.Millisecond)
					defer cancel()
				}
				err = ValidateSSHDisabledAcrossReboot(ctx, s)
				if test.wantError == "" {
					require.NoError(t, err)
				} else {
					require.ErrorContains(t, err, test.wantError)
				}
				if test.kubeletNeverReady {
					require.ErrorIs(t, err, context.DeadlineExceeded)
					require.Greater(t, kubeletChecks, 1)
				} else if !test.invalidBootID && !test.unchangedBootID {
					require.Equal(t, test.kubeletNotReadyChecks+1, kubeletChecks)
				}
			})
			require.Equal(t, test.wantRestarts, restarts)
			require.Equal(t, test.wantSSHChecks, sshChecks)
		})
	}
}

func TestWindowsGuestValidationWithoutSSH(t *testing.T) {
	state, exitCode, commands, deletes := "Succeeded", 0, 0, 0
	respond := vmssCreationTestPolicy(func(req *http.Request) *http.Response {
		switch req.Method {
		case http.MethodPut:
			commands++
			return gcResponse(req, http.StatusOK, `{"properties":{"provisioningState":"Succeeded"}}`)
		case http.MethodGet:
			require.Equal(t, "instanceView", req.URL.Query().Get("$expand"))
			return gcResponse(req, http.StatusOK, fmt.Sprintf(
				`{"properties":{"instanceView":{"executionState":%q,"exitCode":%d}}}`, state, exitCode))
		case http.MethodDelete:
			deletes++
			return gcResponse(req, http.StatusNoContent, "")
		default:
			t.Fatalf("unexpected RunCommand request: %s %s", req.Method, req.URL)
			return nil
		}
	})
	client, err := armcompute.NewVirtualMachineScaleSetVMRunCommandsClient("test", nil, &arm.ClientOptions{
		ClientOptions: policy.ClientOptions{PerCallPolicies: []policy.Policy{respond}},
	})
	require.NoError(t, err)
	previousAzure := config.Azure
	config.Azure = &config.AzureClient{VMSSVMRunCommands: client}
	t.Cleanup(func() { config.Azure = previousAzure })
	s := &Scenario{
		Config: Config{VHD: &config.Image{OS: config.OSWindows}, SkipDefaultValidation: true, SkipSSHConnectivityValidation: true},
		Runtime: &ScenarioRuntime{
			NBC:      &datamodel.NodeBootstrappingConfiguration{PreProvisionOnly: true},
			VMSSName: "vmss",
			Cluster: &Cluster{Model: &armcontainerservice.ManagedCluster{
				Properties: &armcontainerservice.ManagedClusterProperties{NodeResourceGroup: to.Ptr("rg")},
			}},
			VM: &ScenarioVM{VM: &armcompute.VirtualMachineScaleSetVM{InstanceID: to.Ptr("0")}},
		},
	}
	for _, tt := range []struct {
		name, state, wantErr string
		exitCode             int
		preProvision         bool
		validator            func(context.Context, *Scenario, string) error
	}{
		{"bake marker present", "Succeeded", "", 0, true, ValidateFileExists},
		{"provision marker present", "Succeeded", "", 0, false, ValidateFileExists},
		{"marker absent", "Failed", "", 1, true, ValidateFileDoesNotExist},
		{"validation fails", "Failed", "expected file", 1, true, ValidateFileExists},
		{"timeout is not absence", "TimedOut", "RunCommand script failed", 1, true, ValidateFileDoesNotExist},
	} {
		t.Run(tt.name, func(t *testing.T) {
			state, exitCode = tt.state, tt.exitCode
			s.Runtime.NBC.PreProvisionOnly = tt.preProvision
			s.Validator = func(ctx context.Context, s *Scenario) error {
				marker := `C:\AzureData\provision.complete`
				if tt.preProvision {
					marker = `C:\AzureData\base_prep.complete`
				}
				return tt.validator(ctx, s, marker)
			}
			before := commands
			err := validateVM(t.Context(), s)
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
			require.Equal(t, before+1, commands, "guest validation must execute without SSH")
			require.Equal(t, commands, deletes)
		})
	}
	// Sysprep and other existing RunCommand callers still reject non-zero exits.
	state, exitCode = "Failed", 1
	_, err = RunCommand(t.Context(), s, "exit 1")
	require.ErrorContains(t, err, "RunCommand script failed")
	for _, tt := range []struct {
		os                    config.OS
		preProvision, skipSSH bool
	}{
		{config.OSWindows, true, false},
		{config.OSWindows, false, false},
		{config.OSUbuntu, true, false},
		{config.OSUbuntu, true, true},
	} {
		s.VHD.OS = tt.os
		s.Runtime.NBC.PreProvisionOnly = tt.preProvision
		s.SkipSSHConnectivityValidation = tt.skipSSH
		require.ErrorContains(t, validateVM(t.Context(), s), "ssh client is nil")
		_, err = execScriptOnVMForScenario(t.Context(), s, "echo test")
		require.ErrorContains(t, err, "ssh client is nil")
	}
	require.Equal(t, 6, commands)
	require.Equal(t, commands, deletes)
}

func TestRunCommandCleanup(t *testing.T) {
	for _, tt := range []struct {
		name                string
		cancelCaller        bool
		commandFails        bool
		scriptFails         bool
		rawCommand          bool
		deleteStartFails    bool
		deletePollFails     bool
		deleteNeverFinishes bool
		wantErrors          []string
	}{
		{name: "waits for asynchronous deletion"},
		{name: "cleans up after caller cancellation", cancelCaller: true},
		{
			name: "delete initiation failure", deleteStartFails: true,
			wantErrors: []string{"failed to delete RunCommand", "CleanupFailed"},
		},
		{
			name: "delete polling failure", deletePollFails: true,
			wantErrors: []string{"failed to delete RunCommand", "CleanupFailed"},
		},
		{
			name: "preserves command and cleanup failures", commandFails: true, deletePollFails: true,
			wantErrors: []string{"failed to get RunCommand instance view", "InstanceViewFailed", "CleanupFailed"},
		},
		{
			name: "preserves script and cleanup failures", scriptFails: true, deletePollFails: true,
			wantErrors: []string{"CleanupFailed", "RunCommand script failed", "executionState=Failed", "exitCode=42", `stdout="done"`, `stderr="script stderr"`},
		},
		{name: "raw helper allows expected nonzero exit", scriptFails: true, rawCommand: true},
		{
			name: "delete polling timeout", deleteNeverFinishes: true,
			wantErrors: []string{"failed to delete RunCommand", "context deadline exceeded"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				deletePolls := 0
				respond := vmssCreationTestPolicy(func(req *http.Request) *http.Response {
					if req.URL.Path == "/operations/run-command-delete" {
						if tt.cancelCaller {
							require.NoError(t, req.Context().Err(), "cleanup must not use the canceled caller context")
						}
						deletePolls++
						if tt.deletePollFails {
							return gcResponse(req, http.StatusOK, `{"status":"Failed","error":{"code":"CleanupFailed","message":"delete failed"}}`)
						}
						if deletePolls == 1 || tt.deleteNeverFinishes {
							return gcResponse(req, http.StatusOK, `{"status":"InProgress"}`)
						}
						return gcResponse(req, http.StatusOK, `{"status":"Succeeded"}`)
					}
					switch req.Method {
					case http.MethodPut:
						return gcResponse(req, http.StatusOK, `{"properties":{"provisioningState":"Succeeded"}}`)
					case http.MethodGet:
						if tt.commandFails {
							return gcResponse(req, http.StatusBadRequest, `{"error":{"code":"InstanceViewFailed","message":"cannot read result"}}`)
						}
						if tt.cancelCaller {
							cancel()
						}
						state, exitCode, stderr := "Succeeded", 0, ""
						if tt.scriptFails {
							state, exitCode, stderr = "Failed", 42, "script stderr"
						}
						return gcResponse(req, http.StatusOK, fmt.Sprintf(
							`{"properties":{"instanceView":{"executionState":%q,"exitCode":%d,"output":"done","error":%q}}}`, state, exitCode, stderr))
					case http.MethodDelete:
						require.NoError(t, req.Context().Err())
						if tt.deleteStartFails {
							return gcResponse(req, http.StatusBadRequest, `{"error":{"code":"CleanupFailed","message":"cannot start delete"}}`)
						}
						resp := gcResponse(req, http.StatusAccepted, "")
						resp.Header.Set("Azure-AsyncOperation", "https://management.azure.com/operations/run-command-delete")
						return resp
					default:
						t.Fatalf("unexpected RunCommand request: %s %s", req.Method, req.URL)
						return nil
					}
				})
				client, err := armcompute.NewVirtualMachineScaleSetVMRunCommandsClient("test", nil, &arm.ClientOptions{
					ClientOptions: policy.ClientOptions{PerCallPolicies: []policy.Policy{respond}},
				})
				require.NoError(t, err)
				previousAzure := config.Azure
				config.Azure = &config.AzureClient{VMSSVMRunCommands: client}
				t.Cleanup(func() { config.Azure = previousAzure })
				s := &Scenario{
					Runtime: &ScenarioRuntime{
						VMSSName: "vmss",
						Cluster: &Cluster{Model: &armcontainerservice.ManagedCluster{
							Properties: &armcontainerservice.ManagedClusterProperties{NodeResourceGroup: to.Ptr("rg")},
						}},
						VM: &ScenarioVM{VM: &armcompute.VirtualMachineScaleSetVM{InstanceID: to.Ptr("0")}},
					},
				}
				command := RunCommand
				if tt.rawCommand {
					command = runCommand
				}
				view, err := command(ctx, s, "echo done")
				if len(tt.wantErrors) == 0 {
					require.NoError(t, err)
					require.Equal(t, to.Ptr("done"), view.Output)
					if tt.rawCommand {
						require.Equal(t, to.Ptr(int32(42)), view.ExitCode)
					}
					require.Equal(t, 2, deletePolls, "RunCommand must wait until deletion succeeds")
				} else {
					for _, want := range tt.wantErrors {
						require.ErrorContains(t, err, want)
					}
					if !tt.scriptFails {
						require.NotContains(t, err.Error(), "RunCommand script failed")
					}
					if !tt.deleteStartFails {
						require.Positive(t, deletePolls)
					}
				}
			})
		})
	}
}
