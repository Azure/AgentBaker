package agent

import (
	"testing"

	"github.com/Azure/agentbaker/parts"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/require"
)

func TestMarinerLivePatchingUsesVHDInsteadOfCustomData(t *testing.T) {
	template, err := parts.Templates.ReadFile("linux/cloud-init/nodecustomdata.yml")
	require.NoError(t, err)
	for _, distro := range []datamodel.Distro{datamodel.AKSAzureLinuxV2Gen2, datamodel.AKSAzureLinuxV3Gen2} {
		t.Run(string(distro), func(t *testing.T) {
			config := newNodeCustomDataRenderConfig(distro)
			config.EnableScriptlessCSECmd = false
			rendered, err := RenderLinuxNodeCustomDataTemplate(template, config)
			require.NoError(t, err)
			require.NotContains(t, rendered, "/opt/azure/containers/mariner-package-update.sh")
			require.NotContains(t, rendered, "/opt/azure/containers/security-update.sh")
			size := len(getBase64EncodedGzippedCustomScriptFromStr(rendered))
			t.Logf("traditional custom data: %d encoded characters", size)
			require.Less(t, size, MaxCustomDataLength)
		})
	}
}
