package scenario

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/base64"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/stretchr/testify/require"
)

const aclCOSIAMD64ImageID = "/SharedGalleries/035db282-f1c8-4ce7-b78f-2a7265d5398c-ACLDEVEL/Images/acldevel/Versions/0.20260827.1192019"

// Test_ACL_COSI validates the contents of the AMD64 ACL COSI artifact
// (tar structure, metadata.json, and image SHA-384 hashes) without
// provisioning a VM.
func Test_ACL_COSI(t *testing.T) {
	info, ok := loadCOSIPublishingInfo(cosiAMD64PublishingArtifact)
	if !ok {
		t.Skip("COSI artifact not available for acl-tl-gen2, skipping COSI content validation")
	}
	t.Parallel()
	ValidateACLCOSI(t, info.CosiURL)
}

// Test_ACL_COSI_ARM64 validates the contents of the ARM64 ACL COSI artifact.
func Test_ACL_COSI_ARM64(t *testing.T) {
	info, ok := loadCOSIPublishingInfo(cosiARM64PublishingArtifact)
	if !ok {
		t.Skip("COSI artifact not available for acl-arm64-tl-gen2, skipping COSI content validation")
	}
	t.Parallel()
	ValidateACLCOSI(t, info.CosiURL)
}

// Test_ACL_COSI_FIPS validates the contents of the AMD64 FIPS ACL COSI artifact.
func Test_ACL_COSI_FIPS(t *testing.T) {
	info, ok := loadCOSIPublishingInfo(cosiAMD64FIPSPublishingArtifact)
	if !ok {
		t.Skip("COSI artifact not available for acl-fips-tl-gen2, skipping COSI content validation")
	}
	t.Parallel()
	ValidateACLCOSI(t, info.CosiURL)
}

// Test_ACL_COSI_ARM64_FIPS validates the contents of the ARM64 FIPS ACL COSI artifact.
func Test_ACL_COSI_ARM64_FIPS(t *testing.T) {
	info, ok := loadCOSIPublishingInfo(cosiARM64FIPSPublishingArtifact)
	if !ok {
		t.Skip("COSI artifact not available for acl-arm64-fips-tl-gen2, skipping COSI content validation")
	}
	t.Parallel()
	ValidateACLCOSI(t, info.CosiURL)
}

func TestValidateCOSIUpdateInput(t *testing.T) {
	validHash := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0xab}, sha512.Size384))
	require.NoError(t, validateCOSIUpdateInput("https://download.example.com/acl.cosi", validHash))
	require.ErrorContains(t, validateCOSIUpdateInput("http://download.example.com/acl.cosi", validHash), "HTTPS")
	require.ErrorContains(t, validateCOSIUpdateInput("https://download.example.com/acl.cosi", "abcd"), "48 bytes")
}

func TestResolveCOSISharedGalleryImageReference(t *testing.T) {
	imageReference, err := resolveImageReference(context.Background(), &config.Image{
		SharedGalleryImageID: aclCOSIAMD64ImageID,
		Version:              "would-trigger-gallery-lookup-without-shared-id",
	}, "westus2")

	require.NoError(t, err)
	require.Nil(t, imageReference.ID)
	require.Equal(t, aclCOSIAMD64ImageID, *imageReference.SharedGalleryImageID)
}
