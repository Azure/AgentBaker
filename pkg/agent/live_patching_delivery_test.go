package agent

import (
	"testing"

	"github.com/Azure/agentbaker/parts"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/require"
)

func TestLivePatchingUsesVHDInsteadOfCustomData(t *testing.T) {
	template, err := parts.Templates.ReadFile("linux/cloud-init/nodecustomdata.yml")
	require.NoError(t, err)
	config := newNodeCustomDataRenderConfig(datamodel.AKSUbuntuContainerd2204Gen2)
	config.EnableScriptlessCSECmd = false
	rendered, err := RenderLinuxNodeCustomDataTemplate(template, config)
	require.NoError(t, err)
	require.NotContains(t, rendered, "/opt/azure/containers/ubuntu-snapshot-update.sh")
	require.NotContains(t, rendered, "/opt/azure/containers/security-update.sh")
	require.NotContains(t, rendered, "/opt/azure/containers/npd-update.sh")
	size := len(getBase64EncodedGzippedCustomScriptFromStr(rendered))
	t.Logf("traditional custom data: %d encoded characters", size)
	require.Less(t, size, MaxCustomDataLength)
}
