package e2e

import (
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/require"
)

func TestNBCToAKSNodeConfigV1CopiesGPUConfig(t *testing.T) {
	newNBC := func() *datamodel.NodeBootstrappingConfiguration {
		nbc := baseTemplateLinux(t, "westus2", "1.34.8", "amd64")
		nbc.EnableNvidia = true
		nbc.ConfigGPUDriverIfNeeded = true
		nbc.EnableGPUDevicePluginIfNeeded = true
		nbc.ManagedGPUExperienceAFECEnabled = true
		nbc.EnableManagedGPU = true
		return nbc
	}

	t.Run("legacy GPU instance profile", func(t *testing.T) {
		nbc := newNBC()
		nbc.GPUInstanceProfile = "MIG7g"
		nbc.MigStrategy = "Single"

		config := nbcToAKSNodeConfigV1(nbc)

		require.NotNil(t, config.GpuConfig)
		require.True(t, config.GpuConfig.GetEnableNvidia())
		require.True(t, config.GpuConfig.GetConfigGpuDriver())
		require.True(t, config.GpuConfig.GetGpuDevicePlugin())
		require.Equal(t, "MIG7g", config.GpuConfig.GetGpuInstanceProfile())
		require.Empty(t, config.GpuConfig.GetMigProfiles())
		require.True(t, config.GpuConfig.GetManagedGpuExperienceAfecEnabled())
		require.True(t, config.GpuConfig.GetEnableManagedGpu())
		require.Equal(t, "Single", config.GpuConfig.GetMigStrategy())
	})

	t.Run("MIG profiles", func(t *testing.T) {
		nbc := newNBC()
		nbc.MigStrategy = "Mixed"
		nbc.MIGProfiles = []string{"MIG3g", "MIG2g", "MIG1g", "MIG1g"}

		config := nbcToAKSNodeConfigV1(nbc)

		require.NotNil(t, config.GpuConfig)
		require.Empty(t, config.GpuConfig.GetGpuInstanceProfile())
		require.Equal(t, []string{"MIG3g", "MIG2g", "MIG1g", "MIG1g"}, config.GpuConfig.GetMigProfiles())
		require.Equal(t, "Mixed", config.GpuConfig.GetMigStrategy())

		nbc.MIGProfiles[0] = "MIG7g"
		require.Equal(t, "MIG3g", config.GpuConfig.GetMigProfiles()[0], "MIG profiles should not alias the NBC slice")
	})
}
