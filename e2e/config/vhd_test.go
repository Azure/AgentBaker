package config

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetVHDResourceIDExplicit(t *testing.T) {
	expected := VHDResourceID("/SharedGalleries/gallery/Images/image/Versions/1.0.0")

	actual, err := GetVHDResourceID(context.Background(), Image{
		ResourceID: expected,
		Version:    "would-trigger-gallery-lookup-without-resource-id",
	}, "westus2")

	require.NoError(t, err)
	require.Equal(t, expected, actual)
}
