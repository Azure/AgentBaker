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
