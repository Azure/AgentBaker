package config

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v8"
	"github.com/stretchr/testify/require"
)

func TestSkuArchitecture(t *testing.T) {
	for _, tt := range []struct{ value, want string }{
		{"x64", "amd64"}, {" Arm64 ", "arm64"}, {"X64", "amd64"}, {"riscv64", ""}, {"", ""},
	} {
		sku := &armcompute.ResourceSKU{Capabilities: []*armcompute.ResourceSKUCapabilities{
			nil, {}, {Name: to.Ptr("cpuarchitecturetype"), Value: to.Ptr(tt.value)},
		}}
		got, err := SkuArchitecture(sku)
		if tt.want == "" {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		}
	}
	_, err := SkuArchitecture(nil)
	require.Error(t, err)
	_, err = SkuArchitecture(&armcompute.ResourceSKU{})
	require.Error(t, err)
}
