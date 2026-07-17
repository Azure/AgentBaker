package e2e

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNBCToAKSNodeConfigV1CopiesGPUConfig(t *testing.T) {
	nbc := baseTemplateLinux(t, "westus2", "1.34.8", "amd64")
	nbc.EnableNvidia = true
	nbc.ConfigGPUDriverIfNeeded = true
	nbc.EnableGPUDevicePluginIfNeeded = true
	nbc.GPUInstanceProfile = "MIG7g"
	nbc.ManagedGPUExperienceAFECEnabled = true
	nbc.EnableManagedGPU = true
	nbc.MigStrategy = "Mixed"
	nbc.MIGProfiles = []string{"MIG3g", "MIG2g", "MIG1g", "MIG1g"}

	config := nbcToAKSNodeConfigV1(nbc)

	require.NotNil(t, config.GpuConfig)
	require.True(t, config.GpuConfig.GetEnableNvidia())
	require.True(t, config.GpuConfig.GetConfigGpuDriver())
	require.True(t, config.GpuConfig.GetGpuDevicePlugin())
	require.Equal(t, "MIG7g", config.GpuConfig.GetGpuInstanceProfile())
	require.True(t, config.GpuConfig.GetManagedGpuExperienceAfecEnabled())
	require.True(t, config.GpuConfig.GetEnableManagedGpu())
	require.Equal(t, "Mixed", config.GpuConfig.GetMigStrategy())
	require.Equal(t, []string{"MIG3g", "MIG2g", "MIG1g", "MIG1g"}, config.GpuConfig.GetMigProfiles())

	nbc.MIGProfiles[0] = "MIG7g"
	require.Equal(t, "MIG3g", config.GpuConfig.GetMigProfiles()[0], "MIG profiles should not alias the NBC slice")
}
