package scenario

import (
	"context"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v8"
	"github.com/stretchr/testify/require"
)

func TestUbuntuGPUA10Scenarios(t *testing.T) {
	for _, tc := range []struct {
		name string
		vhd  *config.Image
	}{
		{"Ubuntu2404_GPUA10", config.VHDUbuntu2404Gen2Containerd},
		{"Ubuntu2604_GPUA10", config.VHDUbuntu2604MinimalGen2Containerd},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var found *Scenario
			for _, s := range List() {
				if s.Name == tc.name {
					found = s
					break
				}
			}
			require.NotNil(t, found, "GPU scenario must be registered")
			require.Same(t, tc.vhd, found.VHD)
			require.Contains(t, found.Description, tc.vhd.Name)
			require.True(t, found.EffectiveTags().GPU)
			require.Equal(t, string(config.OSUbuntu), found.EffectiveTags().OS)
			require.NotNil(t, found.Cluster)
			require.NotNil(t, found.Validator)
			require.Empty(t, found.SkipReason)
			require.False(t, found.SkipOnCapacityError)

			nbc := &datamodel.NodeBootstrappingConfiguration{
				AgentPoolProfile:              &datamodel.AgentPoolProfile{},
				EnableGPUDevicePluginIfNeeded: true,
			}
			found.BootstrapConfigMutator(nil, nbc)
			require.Equal(t, "Standard_NV6ads_A10_v5", nbc.AgentPoolProfile.VMSize)
			require.True(t, nbc.ConfigGPUDriverIfNeeded)
			require.True(t, nbc.EnableNvidia)
			require.False(t, nbc.EnableGPUDevicePluginIfNeeded)

			vmss := &armcompute.VirtualMachineScaleSet{SKU: &armcompute.SKU{}}
			found.VMConfigMutator(vmss)
			require.Equal(t, nbc.AgentPoolProfile.VMSize, *vmss.SKU.Name)
		})
	}
}

func TestUbuntuGRIDScenarioUsesSuppliedCluster(t *testing.T) {
	want := &Cluster{}
	request := ClusterRequest{Location: "westus2", K8sSystemPoolSKU: "Standard_D2s_v3"}
	called := false
	cluster := func(ctx context.Context, got ClusterRequest) (*Cluster, error) {
		called = true
		require.Equal(t, t.Context(), ctx)
		require.Equal(t, request, got)
		return want, nil
	}
	s := ubuntuGRIDScenario("Ubuntu2604_GPUA10", "Standard_NV6ads_A10_v5",
		config.VHDUbuntu2604MinimalGen2Containerd, cluster)

	got, err := s.Cluster(t.Context(), request)
	require.NoError(t, err)
	require.True(t, called)
	require.Same(t, want, got)
}
