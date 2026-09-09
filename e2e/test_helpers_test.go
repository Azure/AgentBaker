package e2e

import (
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVMSKUGeneration(t *testing.T) {
	tests := []struct {
		name           string
		sku            string
		wantGeneration int
		wantFound      bool
		wantError      bool
	}{
		{
			name:           "versioned SKU",
			sku:            "Standard_D2s_v3",
			wantGeneration: 3,
			wantFound:      true,
		},
		{
			name:           "uppercase version marker",
			sku:            "Standard_D2pds_V5",
			wantGeneration: 5,
			wantFound:      true,
		},
		{
			name:           "GPU SKU with hardware suffix",
			sku:            "Standard_ND96isr_H100_v5",
			wantGeneration: 5,
			wantFound:      true,
		},
		{
			name: "SKU without version marker",
			sku:  "Standard_NM16ads_MA35D",
		},
		{
			name:      "non-numeric generation",
			sku:       "Standard_D2s_vNext",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generation, found, err := vmSKUGeneration(tt.sku)
			if tt.wantError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantGeneration, generation)
			assert.Equal(t, tt.wantFound, found)
		})
	}
}

func TestEnsureMinVMGenerationUsesMinimumForUnversionedDefault(t *testing.T) {
	originalDefaultVMSKU := config.Config.DefaultVMSKU
	config.Config.DefaultVMSKU = "Standard_NM16ads_MA35D"
	t.Cleanup(func() {
		config.Config.DefaultVMSKU = originalDefaultVMSKU
	})

	assert.Equal(t, "Standard_D2ds_v6", ensureMinVMGeneration("Standard_D2ds_v6"))
}
