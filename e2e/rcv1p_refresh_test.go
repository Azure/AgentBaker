package e2e

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestRCV1PRefreshWorkloadIsolation(t *testing.T) {
	s := &Scenario{Runtime: &ScenarioRuntime{VM: &ScenarioVM{KubeName: "scenario-node"}}}
	pod := rcv1pWorkloadPod(s)
	require.Empty(t, pod.Name)
	require.NotEmpty(t, pod.GenerateName)
	require.Empty(t, pod.Spec.NodeName, "use the scheduler, not a direct binding")
	require.Equal(t, map[string]string{"kubernetes.io/hostname": "scenario-node"}, pod.Spec.NodeSelector)
	require.False(t, pod.Spec.HostNetwork)
	require.Equal(t, corev1.PullAlways, pod.Spec.Containers[0].ImagePullPolicy)
	require.NotNil(t, pod.Spec.Containers[0].ReadinessProbe.HTTPGet)
}

func TestRCV1PRefreshScenarioSelection(t *testing.T) {
	generalPipeline, err := os.ReadFile("../.pipelines/e2e.yaml")
	require.NoError(t, err)
	require.Contains(t, string(generalPipeline), `TAGS_TO_SKIP: "os=windows,gpu=true,rcv1pcertmode=true"`)
	positives := map[string]bool{
		"RCV1P_Ubuntu2204": false, "RCV1P_Ubuntu2404": false,
		"RCV1P_Ubuntu2604Minimal": false, "RCV1P_AzureLinuxV3": false, "RCV1P_ACL": false,
	}
	synthetic := 0
	for _, s := range registeredScenarios() {
		_, positive := positives[s.Name]
		isSynthetic := strings.HasPrefix(s.Name, "RCV1P_ContainerdSyntheticCARotation/")
		require.False(t, strings.HasPrefix(s.Name, "ContainerdCARotation/"), "old untagged entry must not survive")
		if !positive && !isSynthetic {
			continue
		}
		require.True(t, s.Tags.RCV1PCertMode, s.Name)
		require.NotNil(t, s.SkipIf, s.Name)
		require.Equal(t, reflect.ValueOf(skipIfRCV1PNotConfigured).Pointer(), reflect.ValueOf(s.SkipIf).Pointer())
		vmss := &armcompute.VirtualMachineScaleSet{Properties: &armcompute.VirtualMachineScaleSetProperties{}}
		s.VMConfigMutator(vmss)
		require.Equal(t, "true", *vmss.Tags[rcv1pOptInTag], s.Name)
		if positive {
			positives[s.Name] = true
			require.Equal(t, reflect.ValueOf(ValidateRCV1PRefreshHealth).Pointer(), reflect.ValueOf(s.Validator).Pointer())
		} else {
			synthetic++
		}
		reason, err := filterReason(s.Name, s, tagFilter{run: "rcv1pcertmode=true"})
		require.NoError(t, err)
		require.Empty(t, reason, s.Name)
		reason, err = filterReason(s.Name, s, tagFilter{skip: "rcv1pcertmode=true"})
		require.NoError(t, err)
		require.NotEmpty(t, reason, s.Name)
		reason, err = filterReason(s.Name, s, tagFilter{run: "rcv1pcertmode=false"})
		require.NoError(t, err)
		require.NotEmpty(t, reason, s.Name)
		// No filter selects all registered cases; tags alone are not a guard.
		reason, err = filterReason(s.Name, s, tagFilter{})
		require.NoError(t, err)
		require.Empty(t, reason)
	}
	for name, found := range positives {
		require.True(t, found, name)
	}
	require.Equal(t, 3, synthetic)
}

func TestRCV1PRefreshFeatureGuard(t *testing.T) {
	old := CachedPlatformSettingsOverrideFeatureFlag
	defer func() { CachedPlatformSettingsOverrideFeatureFlag = old }()
	for _, test := range []struct {
		name       string
		registered bool
		err        error
		skipped    bool
	}{
		{"registered", true, nil, false},
		{"not registered", false, nil, true},
		{"auth failure", false, errors.New("unauthorized"), true},
	} {
		t.Run(test.name, func(t *testing.T) {
			CachedPlatformSettingsOverrideFeatureFlag = func(_ context.Context, sub string) (bool, error) {
				require.Equal(t, config.Config.SubscriptionID, sub)
				return test.registered, test.err
			}
			require.Equal(t, test.skipped, skipIfRCV1PNotConfigured(context.Background()) != "")
		})
	}
}

func TestRCV1PRefreshStagesFailClosed(t *testing.T) {
	failure := errors.New("refresh acquisition failed")
	for _, failAt := range []int{-1, 0, 1, 2, 3} {
		var got []string
		stages := []refreshHealthStage{}
		for i, name := range []string{"provenance", "baseline", "refresh", "health"} {
			stages = append(stages, refreshHealthStage{name, func(context.Context) error {
				got = append(got, name)
				if i == failAt {
					return failure
				}
				return nil
			}})
		}
		err := runRefreshHealthStages(context.Background(), stages)
		if failAt == -1 {
			require.NoError(t, err)
			require.Equal(t, []string{"provenance", "baseline", "refresh", "health"}, got)
		} else {
			require.ErrorIs(t, err, failure)
			require.Contains(t, err.Error(), stages[failAt].name)
			require.Len(t, got, failAt+1)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, runRefreshHealthStages(ctx, []refreshHealthStage{{"canceled", func(context.Context) error {
		t.Fatal("must not run a canceled stage")
		return nil
	}}}), context.Canceled)
}

func TestRCV1PRefreshCommand(t *testing.T) {
	cron := `0 19 * * * "/opt/azure/containers/init-aks-cloud.sh" ca-refresh "westus3"`
	service := "ExecStart=/opt/azure/containers/init-aks-cloud.sh ca-refresh westus3"
	for _, test := range []struct {
		name, schedule, location string
		systemd, valid           bool
	}{
		{"cron", cron, "westus3", false, true},
		{"systemd", "[Service]\n" + service, "westus3", true, true},
		{"wrong location", cron, "eastus2", false, false},
		{"missing location", cron, "", false, false},
		{"duplicate", cron + "\n" + cron, "westus3", false, false},
		{"other script", strings.ReplaceAll(cron, installedRCV1PScript, "/tmp/fixture"), "westus3", false, false},
		{"shell suffix", cron + "; true", "westus3", false, false},
		{"comment only", "#" + cron, "westus3", false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			command, err := rcv1pRefreshCommand(test.schedule, test.location, test.systemd)
			if test.valid {
				require.NoError(t, err)
				require.Contains(t, command, "timeout 300")
			} else {
				require.Error(t, err)
				require.Empty(t, command)
			}
		})
	}
}

func TestRCV1PRefreshOutputRequiresFreshAcquisition(t *testing.T) {
	output := `Using custom cloud certificate endpoint mode: rcv1p
IsOptedInForRootCerts=true
Retrieving certificate operations for type: operationrequestsroot
Successfully saved certificate: root.crt
Retrieving certificate operations for type: operationrequestsintermediate
Successfully saved certificate: intermediate.crt
Trust store contents after cert copy: /usr/local/share/ca-certificates`
	require.NoError(t, validateRCV1PRefreshOutput(output))
	for _, marker := range []string{"rcv1p", "IsOptedInForRootCerts=true", "operationrequestsroot", "operationrequestsintermediate", "Successfully saved certificate:", "Trust store contents after cert copy:"} {
		require.Error(t, validateRCV1PRefreshOutput(strings.ReplaceAll(output, marker, "")), marker)
	}
	for _, marker := range []string{"Warning: No response received", "ERROR: install failed", "Skipping custom cloud", "No certificate filenames"} {
		require.Error(t, validateRCV1PRefreshOutput(output+"\n"+marker), marker)
	}
}
