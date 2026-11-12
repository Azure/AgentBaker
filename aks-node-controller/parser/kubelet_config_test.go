package parser

import (
	"context"
	"encoding/base64"
	"testing"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/aks-node-controller/pkg/nodeconfigutils"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestKubeletConfigFileFieldsRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		config *aksnodeconfigv1.KubeletConfigFileConfig
		want   string
	}{
		{
			name:   "omitted fields",
			config: &aksnodeconfigv1.KubeletConfigFileConfig{},
			want:   `{}`,
		},
		{
			name: "explicit false and zero duration",
			config: &aksnodeconfigv1.KubeletConfigFileConfig{
				EnableServer:          proto.Bool(false),
				RuntimeRequestTimeout: "0s",
			},
			want: `{"enableServer": false, "runtimeRequestTimeout": "0s"}`,
		},
		{
			name: "linux fields and taints",
			config: &aksnodeconfigv1.KubeletConfigFileConfig{
				EnableServer:             proto.Bool(true),
				VolumePluginDir:          "/etc/kubernetes/volumeplugins",
				CgroupDriver:             "systemd",
				RuntimeRequestTimeout:    "2m",
				ContainerRuntimeEndpoint: "unix:///run/containerd/containerd.sock",
				RegisterWithTaints: []*aksnodeconfigv1.KubeletTaint{
					{Key: "workload", Value: "batch", Effect: "NoSchedule"},
					{Key: "workload", Value: "batch", Effect: "PreferNoSchedule"},
					{Key: "maintenance", Effect: "NoExecute", TimeAdded: "2026-01-02T03:04:05Z"},
				},
				HairpinMode: "promiscuous-bridge",
			},
			want: `{
				"enableServer": true,
				"volumePluginDir": "/etc/kubernetes/volumeplugins",
				"cgroupDriver": "systemd",
				"runtimeRequestTimeout": "2m",
				"containerRuntimeEndpoint": "unix:///run/containerd/containerd.sock",
				"registerWithTaints": [
					{"key": "workload", "value": "batch", "effect": "NoSchedule"},
					{"key": "workload", "value": "batch", "effect": "PreferNoSchedule"},
					{"key": "maintenance", "effect": "NoExecute", "timeAdded": "2026-01-02T03:04:05Z"}
				],
				"hairpinMode": "promiscuous-bridge"
			}`,
		},
		{
			name: "windows paths",
			config: &aksnodeconfigv1.KubeletConfigFileConfig{
				VolumePluginDir:          `C:\k\volumeplugins`,
				ContainerRuntimeEndpoint: "npipe:////./pipe/containerd-containerd",
			},
			want: `{
				"volumePluginDir": "C:\\k\\volumeplugins",
				"containerRuntimeEndpoint": "npipe:////./pipe/containerd-containerd"
			}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			original := &aksnodeconfigv1.Configuration{
				KubeletConfig: &aksnodeconfigv1.KubeletConfig{
					KubeletConfigFileConfig: test.config,
				},
			}
			payload, err := nodeconfigutils.MarshalConfigurationV1(original)
			require.NoError(t, err)
			restored, err := nodeconfigutils.UnmarshalConfigurationV1(payload)
			require.NoError(t, err)
			require.True(t, proto.Equal(original, restored), "node configuration JSON round trip")

			wire, err := proto.Marshal(original)
			require.NoError(t, err)
			decoded := &aksnodeconfigv1.Configuration{}
			require.NoError(t, proto.Unmarshal(wire, decoded))
			require.True(t, proto.Equal(original, decoded), "protobuf wire round trip")

			content := getKubeletConfigFileContent(restored.GetKubeletConfig())
			require.JSONEq(t, test.want, content)
			encoded := getKubeletConfigFileContentBase64(restored.GetKubeletConfig())
			decodedContent, err := base64.StdEncoding.DecodeString(encoded)
			require.NoError(t, err)
			require.Equal(t, content, string(decodedContent))
		})
	}
}

func TestKubeletConfigFileLegacyOutputUnchanged(t *testing.T) {
	config := &aksnodeconfigv1.KubeletConfig{
		KubeletConfigFileConfig: &aksnodeconfigv1.KubeletConfigFileConfig{
			Kind:           "KubeletConfiguration",
			ApiVersion:     "kubelet.config.k8s.io/v1beta1",
			Address:        "0.0.0.0",
			EventRecordQps: proto.Int32(0),
			CpuCfsQuota:    proto.Bool(false),
		},
	}
	require.Equal(t, `{
    "kind": "KubeletConfiguration",
    "apiVersion": "kubelet.config.k8s.io/v1beta1",
    "address": "0.0.0.0",
    "eventRecordQPS": 0,
    "cpuCFSQuota": false
}`, getKubeletConfigFileContent(config))
	require.Equal(t, "", getKubeletConfigFileContent(nil))
	require.Equal(t, "{}", getKubeletConfigFileContent(&aksnodeconfigv1.KubeletConfig{}))
}

func TestKubeletConfigSchemaDoesNotActivateFlagMigration(t *testing.T) {
	for _, version := range []string{"1.31.0", "1.32.0", "1.33.0", "1.34.0", "1.35.0", "1.36.0", "1.37.0", "1.38.0-alpha.1", "1.38.0"} {
		t.Run(version, func(t *testing.T) {
			config := &aksnodeconfigv1.Configuration{
				KubernetesVersion: version,
				KubeletConfig: &aksnodeconfigv1.KubeletConfig{
					KubeletFlags: map[string]string{
						"--enable-server":              "false",
						"--volume-plugin-dir":          "/etc/kubernetes/volumeplugins",
						"--cgroup-driver":              "systemd",
						"--runtime-request-timeout":    "2m",
						"--container-runtime-endpoint": "unix:///run/containerd/containerd.sock",
						"--register-with-taints":       "workload=batch:NoSchedule",
						"--hairpin-mode":               "promiscuous-bridge",
					},
				},
			}
			original := proto.Clone(config)
			command, err := BuildCSECmd(context.Background(), config, nil)
			require.NoError(t, err)
			env := environToMap(command.Env)
			require.Equal(t, "false", env["KUBELET_CONFIG_FILE_ENABLED"])
			wantFlags := "--cgroup-driver=systemd" +
				" --container-runtime-endpoint=unix:///run/containerd/containerd.sock" +
				" --enable-server=false --hairpin-mode=promiscuous-bridge" +
				" --register-with-taints=workload=batch:NoSchedule" +
				" --runtime-request-timeout=2m --volume-plugin-dir=/etc/kubernetes/volumeplugins"
			require.Equal(t, wantFlags, env["KUBELET_FLAGS"])
			require.True(t, proto.Equal(original, config), "flag-only input must not be modified")
		})
	}
}
