package scenario

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/base64"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestValidateCOSIUpdateInput(t *testing.T) {
	validHash := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0xab}, sha512.Size384))
	require.NoError(t, validateCOSIUpdateInput("https://download.example.com/acl.cosi", validHash))
	require.ErrorContains(t, validateCOSIUpdateInput("http://download.example.com/acl.cosi", validHash), "HTTPS")
	require.ErrorContains(t, validateCOSIUpdateInput("https://download.example.com/acl.cosi", "abcd"), "48 bytes")
}

func TestValidateACLAnnotationUpdateInput(t *testing.T) {
	require.NoError(t, validateACLAnnotationUpdateInput("202609.21.1", "https://nebraska.example.com/v1/update", "11111111-2222-3333-4444-555555555555"))
	require.ErrorContains(t, validateACLAnnotationUpdateInput("", "https://nebraska.example.com/v1/update", "app-id"), "image_version")
	require.ErrorContains(t, validateACLAnnotationUpdateInput("202609.21.1", "http://nebraska.example.com/v1/update", "app-id"), "HTTPS")
	require.ErrorContains(t, validateACLAnnotationUpdateInput("202609.21.1", "https://nebraska.example.com/", "app-id"), "Omaha path")
	require.ErrorContains(t, validateACLAnnotationUpdateInput("202609.21.1", "https://nebraska.example.com/v1/update", ""), "app ID")
}

func TestACLAnnotationTargetMustBeNewer(t *testing.T) {
	require.ErrorContains(t, validateACLTargetVersion(aclCOSIAMD64BaselineImageVersion, "202609.20.9"), "must be newer")
	require.ErrorContains(t, validateACLTargetVersion(aclCOSIAMD64BaselineImageVersion, "202609.21.0"), "must be newer")
	require.ErrorContains(t, validateACLTargetVersion(aclCOSIAMD64BaselineImageVersion, "not-semver"), "parse COSI target")
	require.NoError(t, validateACLTargetVersion(aclCOSIAMD64BaselineImageVersion, "202609.21.1"))
}

func TestCOSIAMD64BaselineVHD(t *testing.T) {
	baseline := *config.VHDACLGen2TL
	baseline.Version = aclCOSIAMD64BaselineVHDVersion

	require.Equal(t, "1.1790005343.11420", baseline.Version)
	require.Empty(t, config.VHDACLGen2TL.Version, "the frozen COSI baseline must not mutate other ACL scenarios")
}

func TestPatchACLUpdateRequestWireFormat(t *testing.T) {
	nodeName := "node-1"
	client := fake.NewSimpleClientset(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeName}})
	request := aclUpdateRequest{
		SchemaVersion: aclUpdateSchemaVersion,
		NodeUpdateID:  "node-update-1",
		OperationID:   "operation-1",
		Operation:     aclUpdateOperationStage,
		TargetVersion: "202609.21.1",
		Server:        "https://nebraska.example.com/v1/update",
		AppID:         "11111111-2222-3333-4444-555555555555",
		Track:         "pin-202609.21.1",
	}

	require.NoError(t, patchACLUpdateRequest(context.Background(), client.CoreV1().Nodes(), nodeName, request))
	node, err := client.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	require.NoError(t, err)
	require.JSONEq(t, `{
		"schemaVersion":"1.0",
		"nodeUpdateId":"node-update-1",
		"operationId":"operation-1",
		"operation":"stage",
		"targetVersion":"202609.21.1",
		"server":"https://nebraska.example.com/v1/update",
		"appId":"11111111-2222-3333-4444-555555555555",
		"track":"pin-202609.21.1"
	}`, node.Annotations[aclUpdateRequestAnnotationKey])
}

func TestMatchACLUpdateStatusIgnoresStaleOperation(t *testing.T) {
	expected := aclUpdateRequest{NodeUpdateID: "node-update-1", OperationID: "operation-current", Operation: aclUpdateOperationStage}
	status, terminal, err := matchACLUpdateStatus(`{
		"schemaVersion":"1.0",
		"nodeUpdateId":"node-update-old",
		"operationId":"operation-old",
		"operation":"stage",
		"code":"Success"
	}`, expected)

	require.NoError(t, err)
	require.Nil(t, status)
	require.False(t, terminal)
}

func TestMatchACLUpdateStatusRejectsMismatchedSequence(t *testing.T) {
	expected := aclUpdateRequest{NodeUpdateID: "node-update-1", OperationID: "operation-1", Operation: aclUpdateOperationStage}
	_, _, err := matchACLUpdateStatus(`{
		"schemaVersion":"1.0",
		"nodeUpdateId":"node-update-other",
		"operationId":"operation-1",
		"operation":"stage",
		"code":"Success"
	}`, expected)

	require.ErrorContains(t, err, "nodeUpdateId")
}

func TestMatchACLUpdateStatusMatchesImplicitCommit(t *testing.T) {
	expected := aclUpdateRequest{NodeUpdateID: "node-update-1", OperationID: "finalize-operation-1", Operation: aclUpdateOperationCommit}
	status, terminal, err := matchACLUpdateStatus(`{
		"schemaVersion":"1.0",
		"nodeUpdateId":"node-update-1",
		"operationId":"finalize-operation-1",
		"operation":"commit",
		"code":"Success"
	}`, expected)

	require.NoError(t, err)
	require.True(t, terminal)
	require.Equal(t, aclUpdateCodeSuccess, status.Code)
}
