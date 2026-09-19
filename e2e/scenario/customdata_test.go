package scenario

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/require"
)

func TestLegacyLinuxCustomDataFitsARM(t *testing.T) {
	for _, distro := range []datamodel.Distro{
		datamodel.AKSAzureLinuxV2Gen2,
		datamodel.AKSAzureLinuxV3Gen2,
		datamodel.AKSUbuntuContainerd2204Gen2,
	} {
		t.Run(string(distro), func(t *testing.T) {
			nbc, err := baseTemplateLinux("westus3", "1.30.101-akslts", "amd64")
			require.NoError(t, err)
			nbc.AgentPoolProfile.Distro = distro
			nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = distro
			nbc.EnableScriptlessCSECmd = false
			nbc.EnableScriptlessNBCCSECmd = false
			baker, err := agent.NewAgentBaker()
			require.NoError(t, err)
			bootstrap, err := baker.GetNodeBootstrapping(t.Context(), nbc)
			require.NoError(t, err)
			t.Logf("customData: %d characters (ARM maximum %d)", len(bootstrap.CustomData), agent.MaxCustomDataLength)
			require.LessOrEqual(t, len(bootstrap.CustomData), agent.MaxCustomDataLength)
			compressed, err := base64.StdEncoding.DecodeString(bootstrap.CustomData)
			require.NoError(t, err)
			require.LessOrEqual(t, len(compressed), 65535)
			reader, err := gzip.NewReader(bytes.NewReader(compressed))
			require.NoError(t, err)
			rendered, err := io.ReadAll(reader)
			require.NoError(t, err)
			require.NoError(t, reader.Close())
			require.True(t, bytes.HasPrefix(rendered, []byte("#cloud-config\n")))
			require.Contains(t, string(rendered), "/opt/azure/containers/provision_configs_localdns.sh")
			require.NotEmpty(t, bootstrap.CSE)
		})
	}
}
