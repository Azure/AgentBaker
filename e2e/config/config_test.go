package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultConfigurationInADO(t *testing.T) {
	for _, value := range []string{"", "False", "True", "true", "TRUE"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("TF_BUILD", value)
			require.Equal(t, value == "True" || value == "true" || value == "TRUE", DefaultConfiguration().InADO)
		})
	}

}

func TestResourceGroupIsolation(t *testing.T) {
	previous := Config
	t.Cleanup(func() { Config = previous })
	Config = DefaultConfiguration()
	require.Equal(t, "abe2e-westus3", ResourceGroupName("westus3"))
	Config.ResourceGroupPrefix = "abe2e-boot-trace-isolated"
	require.Equal(t, "abe2e-boot-trace-isolated-westus3", ResourceGroupName("westus3"))
}
