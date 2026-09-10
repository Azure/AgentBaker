package e2e

import (
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVMSKUGeneration(t *testing.T) {
	for _, tt := range []struct {
		sku  string
		want int
	}{
		{sku: "Standard_D2s_v3", want: 3},
		{sku: "Standard_D2ds_v6", want: 6},
		{sku: "STANDARD_D2DS_V10", want: 10},
	} {
		t.Run(tt.sku, func(t *testing.T) {
			got, ok := vmSKUGeneration(tt.sku)
			require.True(t, ok)
			assert.Equal(t, tt.want, got)
		})
	}

	for _, sku := range []string{
		"Standard_NM16ads_MA35D",
		"Standard_NC6",
		"",
		"Standard_D2ds_v",
		"Standard_D2ds_vnext",
		"Standard_D2ds_v6_extra",
		"Standard_D2ds_v999999999999999999999999",
	} {
		t.Run(sku, func(t *testing.T) {
			got, ok := vmSKUGeneration(sku)
			require.False(t, ok)
			assert.Zero(t, got)
		})
	}
}

func TestEnsureMinVMGeneration(t *testing.T) {
	tests := []struct {
		name       string
		defaultSKU string
		minSKU     string
		want       string
		wantPanic  bool
	}{
		{
			name:       "older default",
			defaultSKU: "Standard_D2ds_v5",
			minSKU:     "Standard_D2ds_v6",
			want:       "Standard_D2ds_v6",
		},
		{
			name:       "equal version keeps default",
			defaultSKU: "Standard_D4ds_v6",
			minSKU:     "Standard_D2ds_v6",
			want:       "Standard_D4ds_v6",
		},
		{
			name:       "newer default",
			defaultSKU: "Standard_D2ds_v7",
			minSKU:     "Standard_D2ds_v6",
			want:       "Standard_D2ds_v7",
		},
		{
			name:       "unversioned default",
			defaultSKU: "Standard_NM16ads_MA35D",
			minSKU:     "Standard_D2ds_v6",
			wantPanic:  true,
		},
		{
			name:       "unversioned minimum",
			defaultSKU: "Standard_D2ds_v6",
			minSKU:     "Standard_NM16ads_MA35D",
			wantPanic:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalSKU := config.Config.DefaultVMSKU
			t.Cleanup(func() { config.Config.DefaultVMSKU = originalSKU })
			config.Config.DefaultVMSKU = tt.defaultSKU

			if tt.wantPanic {
				assert.Panics(t, func() { ensureMinVMGeneration(tt.minSKU) })
				return
			}
			assert.Equal(t, tt.want, ensureMinVMGeneration(tt.minSKU))
		})
	}
}
