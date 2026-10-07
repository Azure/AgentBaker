package scenario

import (
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/stretchr/testify/require"
)

func TestSystemPoolSizeIsIndependentForVMSeries(t *testing.T) {
	c := config.DefaultConfiguration()
	c.DefaultVMSKU = "Standard_E64adus_v7"
	s := &Scenario{}
	require.Equal(t, c.DefaultVMSKU, s.systemPoolVMSize(c))
	c.VMSeriesCoverage = true
	require.Equal(t, config.DEFAULT_VMSKU, s.systemPoolVMSize(c))
	c.SystemPoolVMSKU = "Standard_D2ds_v6"
	require.Equal(t, c.SystemPoolVMSKU, s.systemPoolVMSize(c))
	s.K8sSystemPoolSKU = "explicit-override"
	require.Equal(t, s.K8sSystemPoolSKU, s.systemPoolVMSize(c))
	require.Equal(t, "Standard_E64adus_v7", c.DefaultVMSKU)
}
