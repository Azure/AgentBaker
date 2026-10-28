package parser

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os/exec"
	"strconv"
	"testing"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/aks-node-controller/pkg/nodeconfigutils"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestKubeletFlagsToOmitUsesCurrentTypedConfig(test *testing.T) {
	encodedOmissions := base64.RawStdEncoding.EncodeToString([]byte(`["--enable-server","--runtime-request-timeout"]`))
	for _, testCase := range []struct {
		name     string
		features map[string]string
		expected string
	}{
		{name: "absent"},
		{name: "empty map", features: map[string]string{}},
		{name: "empty instruction", features: map[string]string{"KUBELET_FLAGS_TO_OMIT": ""}},
		{name: "selected", features: map[string]string{"KUBELET_FLAGS_TO_OMIT": encodedOmissions}, expected: encodedOmissions},
		{name: "unrelated", features: map[string]string{"UNRELATED_FEATURE": "true"}},
		{name: "mixed", features: map[string]string{"KUBELET_FLAGS_TO_OMIT": encodedOmissions, "UNRELATED_FEATURE": "true"}, expected: encodedOmissions},
		{name: "malformed single line", features: map[string]string{"KUBELET_FLAGS_TO_OMIT": "invalid!"}, expected: "invalid!"},
		{name: "newline", features: map[string]string{"KUBELET_FLAGS_TO_OMIT": encodedOmissions + "\n"}},
		{name: "carriage return", features: map[string]string{"KUBELET_FLAGS_TO_OMIT": encodedOmissions + "\r"}},
	} {
		for _, configEnabled := range []bool{false, true} {
			test.Run(testCase.name+"/config="+strconv.FormatBool(configEnabled), func(test *testing.T) {
				test.Setenv("KUBELET_FLAGS_TO_OMIT", "stale-inherited-instruction")
				test.Setenv("PRESERVED_TEST_ENV", "preserved")
				config := &aksnodeconfigv1.Configuration{
					Version: "v1", EnabledFeatures: testCase.features,
					KubeletConfig: &aksnodeconfigv1.KubeletConfig{
						EnableKubeletConfigFile: configEnabled,
						KubeletFlags:            map[string]string{"--runtime-request-timeout": "0s", "--enable-server": "false"},
						KubeletConfigFileConfig: &aksnodeconfigv1.KubeletConfigFileConfig{
							EnableServer: proto.Bool(true), VolumePluginDir: "/etc/kubernetes/volumeplugins",
							CgroupDriver: "systemd", RuntimeRequestTimeout: "15m",
							ContainerRuntimeEndpoint: "unix:///run/containerd/containerd.sock",
						},
					},
				}
				serialized, err := nodeconfigutils.MarshalConfigurationV1(config)
				require.NoError(test, err)
				decoded, err := nodeconfigutils.UnmarshalConfigurationV1(serialized)
				require.NoError(test, err)
				command, err := BuildCSECmd(context.Background(), decoded, nil)
				require.NoError(test, err)
				child := exec.Command("/bin/bash", "-c", `printf '%s\n%s' "${KUBELET_FLAGS_TO_OMIT:-}" "$PRESERVED_TEST_ENV"`)
				child.Env = command.Env
				output, err := child.CombinedOutput()
				require.NoError(test, err, "%s", output)
				require.Equal(test, testCase.expected+"\npreserved", string(output))
				environment := environToMap(command.Env)
				require.NotContains(test, environment, "UNRELATED_FEATURE")
				require.Equal(test, strconv.FormatBool(configEnabled), environment["KUBELET_CONFIG_FILE_ENABLED"])
				require.Equal(test, "--enable-server=false --runtime-request-timeout=0s", environment["KUBELET_FLAGS"])
				configJSON, err := base64.StdEncoding.DecodeString(environment["KUBELET_CONFIG_FILE_CONTENT"])
				require.NoError(test, err)
				var configFields map[string]any
				require.NoError(test, json.Unmarshal(configJSON, &configFields))
				require.Equal(test, true, configFields["enableServer"])
				require.Equal(test, "/etc/kubernetes/volumeplugins", configFields["volumePluginDir"])
				require.Equal(test, "systemd", configFields["cgroupDriver"])
				require.Equal(test, "15m", configFields["runtimeRequestTimeout"])
				require.Equal(test, "unix:///run/containerd/containerd.sock", configFields["containerRuntimeEndpoint"])
			})
		}
	}
}
