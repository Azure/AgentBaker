package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVHDBuilderGalleryImageName(t *testing.T) {
	tests := []struct {
		name     string
		image    Image
		expected string
	}{
		{
			name:     "Ubuntu 2004 CVM",
			image:    *VHDUbuntu2004CVMGen2Containerd,
			expected: "2004gen2CVMcontainerdSpecialized",
		},
		{
			name:     "Ubuntu 2404 CVM",
			image:    *VHDUbuntu2404CVMGen2Containerd,
			expected: "2404gen2CVMcontainerdSpecialized",
		},
		{
			name:     "Ubuntu 2604 minimal CVM",
			image:    *VHDUbuntu2604MinimalCVMGen2Containerd,
			expected: "2604minimalgen2CVMcontainerdSpecialized",
		},
		{
			name:     "Azure Linux V3 CVM",
			image:    *VHDAzureLinuxV3CVMGen2,
			expected: "AzureLinuxV3gen2CVM",
		},
		{
			name:     "non-CVM image",
			image:    *VHDUbuntu2404Gen2Containerd,
			expected: "2404gen2containerd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.image.vhdBuilderGalleryImageName())
		})
	}
}

func TestGetVHDResourceIDFromMetadataUsesVHDBuilderGalleryImageName(t *testing.T) {
	tests := []struct {
		name              string
		image             Image
		metadataImageName string
	}{
		{
			name:              "Ubuntu 2004 CVM",
			image:             *VHDUbuntu2004CVMGen2Containerd,
			metadataImageName: "2004gen2CVMcontainerdSpecialized",
		},
		{
			name:              "Ubuntu 2404 CVM",
			image:             *VHDUbuntu2404CVMGen2Containerd,
			metadataImageName: "2404gen2CVMcontainerdSpecialized",
		},
		{
			name:              "Ubuntu 2604 minimal CVM",
			image:             *VHDUbuntu2604MinimalCVMGen2Containerd,
			metadataImageName: "2604minimalgen2CVMcontainerdSpecialized",
		},
		{
			name:              "Azure Linux V3 CVM",
			image:             *VHDAzureLinuxV3CVMGen2,
			metadataImageName: "AzureLinuxV3gen2CVM",
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
