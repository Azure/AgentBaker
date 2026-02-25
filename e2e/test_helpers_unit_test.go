package e2e

import (
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
)

func makeCapabilities(pairs ...string) []*armcompute.ResourceSKUCapabilities {
	caps := make([]*armcompute.ResourceSKUCapabilities, 0, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		name := pairs[i]
		value := pairs[i+1]
		caps = append(caps, &armcompute.ResourceSKUCapabilities{
			Name:  &name,
			Value: &value,
		})
	}
	return caps
}

func Test_skuSupportsNVMe(t *testing.T) {
	tests := []struct {
		name         string
		capabilities []*armcompute.ResourceSKUCapabilities
		want         bool
	}{
		{
			name:         "NVMe only",
			capabilities: makeCapabilities("DiskControllerTypes", "NVMe"),
			want:         true,
		},
		{
			name:         "SCSI and NVMe",
			capabilities: makeCapabilities("DiskControllerTypes", "SCSI, NVMe"),
			want:         true,
		},
		{
			name:         "SCSI only",
			capabilities: makeCapabilities("DiskControllerTypes", "SCSI"),
			want:         false,
		},
		{
			name:         "no DiskControllerTypes capability",
			capabilities: makeCapabilities("vCPUs", "4"),
			want:         false,
		},
		{
			name:         "empty capabilities",
			capabilities: nil,
			want:         false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sku := &armcompute.ResourceSKU{
				Capabilities: tt.capabilities,
			}
			if got := config.SkuSupportsNVMe(sku); got != tt.want {
				t.Errorf("SkuSupportsNVMe() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_skuSupportsOnlyGen2(t *testing.T) {
	tests := []struct {
		name         string
		capabilities []*armcompute.ResourceSKUCapabilities
		want         bool
	}{
		{
			name:         "V2 only",
			capabilities: makeCapabilities("HyperVGenerations", "V2"),
			want:         true,
		},
		{
			name:         "V1 and V2",
			capabilities: makeCapabilities("HyperVGenerations", "V1,V2"),
			want:         false,
		},
		{
			name:         "V1 only",
			capabilities: makeCapabilities("HyperVGenerations", "V1"),
			want:         false,
		},
		{
			name:         "no HyperVGenerations capability",
			capabilities: makeCapabilities("vCPUs", "4"),
			want:         false,
		},
		{
			name:         "empty capabilities",
			capabilities: nil,
			want:         false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sku := &armcompute.ResourceSKU{
				Capabilities: tt.capabilities,
			}
			if got := config.SkuSupportsOnlyGen2(sku); got != tt.want {
				t.Errorf("SkuSupportsOnlyGen2() = %v, want %v", got, tt.want)
			}
		})
	}
}
