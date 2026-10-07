package agent

import (
	"encoding/base64"
	"os/exec"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/require"
)

func TestKubeletFlagsToOmitLegacyCSETransport(test *testing.T) {
	encoded := base64.RawStdEncoding.EncodeToString([]byte(`["--enable-server","--runtime-request-timeout"]`))
	for _, version := range []string{"1.31.0", "1.37.99", "1.38.0-beta.0", "1.38.0", "1.39.0"} {
		for _, configEnabled := range []bool{false, true} {
			test.Run(version+"/config="+map[bool]string{false: "off", true: "on"}[configEnabled], func(test *testing.T) {
				config := newNodeCustomDataRenderConfig(datamodel.AKSUbuntuContainerd2204Gen2)
				config.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = version
				config.EnableKubeletConfigFile = configEnabled
				generator := InitializeTemplateGenerator()
				legacy := generator.getLinuxNodeCSECommand(config)
				require.NotContains(test, legacy, "KUBELET_FLAGS_TO_OMIT=")
				for _, value := range []string{"", "W10=", "invalid!", strings.Repeat("a", 1025)} {
					config.EnabledFeatures = map[string]string{"KUBELET_FLAGS_TO_OMIT": value}
					require.Equal(test, legacy, generator.getLinuxNodeCSECommand(config))
				}
				config.EnabledFeatures = map[string]string{"KUBELET_FLAGS_TO_OMIT": encoded, "NOT_A_CSE_FEATURE": "true"}
				command := generator.getLinuxNodeCSECommand(config)
				require.Equal(test, legacy, strings.Replace(command, `KUBELET_FLAGS_TO_OMIT="`+encoded+`" `, "", 1))
				require.NotContains(test, command, "NOT_A_CSE_FEATURE=")
				start := strings.Index(command, "KUBELET_FLAGS_TO_OMIT=")
				end := strings.Index(command, "/usr/bin/nohup")
				require.Greater(test, end, start)
				require.GreaterOrEqual(test, start, 0)
				child := exec.Command("/bin/bash", "-c", command[start:end]+`/bin/bash -c 'printf "%s" "$KUBELET_FLAGS_TO_OMIT"'`)
				output, err := child.CombinedOutput()
				require.NoError(test, err, "%s", output)
				require.Equal(test, encoded, string(output))
			})
		}
	}
}

func TestKubeletFlagsToOmitScriptlessNBCTransport(test *testing.T) {
	encoded := base64.RawStdEncoding.EncodeToString([]byte(`["--enable-server"]`))
	config := newNodeCustomDataRenderConfig(datamodel.AKSUbuntuContainerd2204Gen2)
	config.EnableScriptlessNBCCSECmd = true
	config.EnabledFeatures = map[string]string{"KUBELET_FLAGS_TO_OMIT": encoded}
	generator := InitializeTemplateGenerator()
	boothook, err := base64.StdEncoding.DecodeString(generator.getScriptlessBoothook(config))
	require.NoError(test, err)
	require.Contains(test, string(boothook), getBase64EncodedGzippedCustomScriptFromStr("KUBELET_FLAGS_TO_OMIT="+encoded+"\n"))
	require.Contains(test, generator.getScriptlessNBCCmd(config), getBase64EncodedGzippedCustomScriptFromStr(generator.getLinuxNodeCSECommand(config)))
}
