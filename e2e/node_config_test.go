package e2e

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNBCToAKSNodeConfigV1PropagatesMIGTotalSlices(t *testing.T) {
	nbc := baseTemplateLinux(t, "eastus", "1.30.0", "amd64")
	migTotalSlices := int32(4)
	nbc.GPUInstanceProfile = "MIG2g"
	nbc.TotalGPUInstanceSlices = &migTotalSlices

	config := nbcToAKSNodeConfigV1(nbc)

	require.Equal(t, "MIG2g", config.GetGpuConfig().GetGpuInstanceProfile())
	require.Equal(t, int32(4), config.GetGpuConfig().GetMigTotalSlices())
	require.NotNil(t, config.GetGpuConfig().MigTotalSlices)
}
