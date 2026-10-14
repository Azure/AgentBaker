package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetVHDResourceIDFromMetadataUsesLogicalImageName(t *testing.T) {
	tests := []struct {
		name              string
		image             Image
		metadataImageName string
	}{
		{
			name:              "Ubuntu 2004 CVM",
			image:             *VHDUbuntu2004CVMGen2Containerd,
			metadataImageName: "2004gen2CVMcontainerd",
		},
		{
			name:              "Ubuntu 2404 CVM",
			image:             *VHDUbuntu2404CVMGen2Containerd,
			metadataImageName: "2404gen2CVMcontainerd",
		},
		{
			name:              "Ubuntu 2604 minimal CVM",
			image:             *VHDUbuntu2604MinimalCVMGen2Containerd,
			metadataImageName: "2604minimalgen2CVMcontainerd",
		},
		{
			name:              "Azure Linux V3 CVM",
			image:             *VHDAzureLinuxV3CVMGen2,
			metadataImageName: "V3gen2CVM",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectedResourceID := VHDResourceID("/subscriptions/test/resourceGroups/test/providers/Microsoft.Compute/galleries/test/images/test/versions/1.0.0")
			metadata := map[string]vhdMetadataEntry{
				tt.metadataImageName: {
					ResourceID: expectedResourceID,
					Regions:    []string{"west europe"},
				},
			}

			resourceID, err := getVHDResourceIDFromMetadata(metadata, tt.image, "West Europe")

			require.NoError(t, err)
			require.Equal(t, expectedResourceID, resourceID)
		})
	}
}
