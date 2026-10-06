package config

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v8"
	"github.com/stretchr/testify/require"
)

func TestSkuSupportsEphemeralOSDisk(t *testing.T) {
	for _, tt := range []struct {
		name    string
		sku     *armcompute.ResourceSKU
		want    bool
		wantErr bool
	}{
		{name: "supported", sku: ephemeralTestSKU("True"), want: true},
		{name: "unsupported", sku: ephemeralTestSKU("False")},
		{name: "case and whitespace", sku: ephemeralTestSKU(" true "), want: true},
		{name: "invalid value", sku: ephemeralTestSKU("unknown"), wantErr: true},
		{name: "missing capability", sku: &armcompute.ResourceSKU{}, wantErr: true},
		{name: "nil SKU", wantErr: true},
		{name: "nil metadata", sku: &armcompute.ResourceSKU{Capabilities: []*armcompute.ResourceSKUCapabilities{
			nil, {}, {Name: to.Ptr("EphemeralOSDiskSupported")},
		}}, wantErr: true},
		{name: "NVMe alone is insufficient", sku: &armcompute.ResourceSKU{Capabilities: []*armcompute.ResourceSKUCapabilities{
			{Name: to.Ptr("DiskControllerTypes"), Value: to.Ptr("NVMe")},
		}}, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SkuSupportsEphemeralOSDisk(tt.sku)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.want, got)
		})
	}
}

func ephemeralTestSKU(value string) *armcompute.ResourceSKU {
	return &armcompute.ResourceSKU{
		Name: to.Ptr("test-size"),
		Capabilities: []*armcompute.ResourceSKUCapabilities{
			nil, {},
			{Name: to.Ptr("ephemeralosdisksupported"), Value: to.Ptr(value)},
		},
	}
}
