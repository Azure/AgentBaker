package nodeconfigutils

import (
	"encoding/base64"
	"testing"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/stretchr/testify/require"
)

func TestCustomDataTransportsKubeletFlagsToOmit(test *testing.T) {
	encoded := base64.RawStdEncoding.EncodeToString([]byte(`["--runtime-request-timeout"]`))
	config := &aksnodeconfigv1.Configuration{EnabledFeatures: map[string]string{"KUBELET_FLAGS_TO_OMIT": encoded}}
	require.Contains(test, decodeBoothook(test, config), "\nKUBELET_FLAGS_TO_OMIT="+encoded+"\n")
	require.Empty(test, enabledFeaturesBlock(&aksnodeconfigv1.Configuration{}))
	require.Equal(test, decodeBoothook(test, &aksnodeconfigv1.Configuration{}), decodeBoothook(test, &aksnodeconfigv1.Configuration{EnabledFeatures: map[string]string{}}))
}
