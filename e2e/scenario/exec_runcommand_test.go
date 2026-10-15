package scenario

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v8"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/stretchr/testify/require"
)

func TestWindowsPreProvisionGuestValidation(t *testing.T) {
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
		Config: Config{VHD: &config.Image{OS: config.OSWindows}, SkipDefaultValidation: true},
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
		validator            func(context.Context, *Scenario, string) error
	}{
		{"marker present", "Succeeded", "", 0, ValidateFileExists},
		{"marker absent", "Failed", "", 1, ValidateFileDoesNotExist},
		{"validation fails", "Failed", "expected file", 1, ValidateFileExists},
		{"timeout is not absence", "TimedOut", "RunCommand script failed", 1, ValidateFileDoesNotExist},
	} {
		t.Run(tt.name, func(t *testing.T) {
			state, exitCode = tt.state, tt.exitCode
			s.Validator = func(ctx context.Context, s *Scenario) error {
				return tt.validator(ctx, s, `C:\AzureData\base_prep.complete`)
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
	for _, os := range []config.OS{config.OSWindows, config.OSUbuntu} {
		s.VHD.OS = os
		s.Runtime.NBC.PreProvisionOnly = os == config.OSUbuntu
		require.ErrorContains(t, validateVM(t.Context(), s), "ssh client is nil")
		_, err = execScriptOnVMForScenario(t.Context(), s, "echo test")
		require.ErrorContains(t, err, "ssh client is nil")
	}
	require.Equal(t, 5, commands)
	require.Equal(t, commands, deletes)
}
