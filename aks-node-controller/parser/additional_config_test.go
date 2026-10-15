package parser

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/aks-node-controller/pkg/nodeconfigutils"
	"github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestAdditionalKubeletConfigMerge(t *testing.T) {
	additional, err := structpb.NewStruct(map[string]any{
		"maxParallelImagePulls": 5,
		"memorySwap": map[string]any{
			"swapBehavior": "LimitedSwap",
		},
		"showHiddenMetricsForVersion": nil,
	})
	require.NoError(t, err)

	content, err := getKubeletConfigFileContent(&aksnodeconfigv1.KubeletConfig{
		EnableKubeletConfigFile: true,
		KubeletConfigFileConfig: &aksnodeconfigv1.KubeletConfigFileConfig{
			Kind: "KubeletConfiguration",
		},
		AdditionalKubeletConfig: additional,
	})
	require.NoError(t, err)

	var merged map[string]any
	require.NoError(t, json.Unmarshal([]byte(content), &merged))
	require.Equal(t, "KubeletConfiguration", merged["kind"])
	require.Equal(t, float64(5), merged["maxParallelImagePulls"])
	require.Equal(t, map[string]any{"swapBehavior": "LimitedSwap"}, merged["memorySwap"])
	require.Contains(t, merged, "showHiddenMetricsForVersion")
	require.Nil(t, merged["showHiddenMetricsForVersion"])
}

func TestAdditionalKubeletConfigRejectsCollisionAndDisabledFile(t *testing.T) {
	additional, err := structpb.NewStruct(map[string]any{"kind": "Other"})
	require.NoError(t, err)
	_, err = getKubeletConfigFileContent(&aksnodeconfigv1.KubeletConfig{
		EnableKubeletConfigFile: true,
		KubeletConfigFileConfig: &aksnodeconfigv1.KubeletConfigFileConfig{
			Kind: "KubeletConfiguration",
		},
		AdditionalKubeletConfig: additional,
	})
	require.ErrorContains(t, err, `collides with generated field "kind"`)

	additional, err = structpb.NewStruct(map[string]any{"memorySwap": map[string]any{"swapBehavior": "LimitedSwap"}})
	require.NoError(t, err)
	_, err = getKubeletConfigFileContent(&aksnodeconfigv1.KubeletConfig{AdditionalKubeletConfig: additional})
	require.ErrorContains(t, err, "requires enable_kubelet_config_file")
}

func TestAdditionalContainerdConfigMerge(t *testing.T) {
	additional, err := structpb.NewStruct(map[string]any{
		"plugins": map[string]any{
			"io.containerd.grpc.v1.cri": map[string]any{
				"image_pull_progress_timeout": "10m",
				"max_concurrent_downloads":    5,
			},
		},
		"timeouts": map[string]any{
			"io.containerd.timeout.shim.cleanup": "5s",
		},
	})
	require.NoError(t, err)
	config := &aksnodeconfigv1.Configuration{
		ContainerdConfig: &aksnodeconfigv1.ContainerdConfig{AdditionalContainerdConfig: additional},
	}

	content, err := containerdConfigFromAKSNodeConfig(config, true, "1.7.22")
	require.NoError(t, err)
	var merged map[string]any
	require.NoError(t, toml.Unmarshal([]byte(content), &merged))

	plugins := merged["plugins"].(map[string]any)
	cri := plugins["io.containerd.grpc.v1.cri"].(map[string]any)
	require.Equal(t, "10m", cri["image_pull_progress_timeout"])
	require.Equal(t, int64(5), cri["max_concurrent_downloads"])
	require.Equal(t, "5s", merged["timeouts"].(map[string]any)["io.containerd.timeout.shim.cleanup"])

	contentAgain, err := containerdConfigFromAKSNodeConfig(config, true, "1.7.22")
	require.NoError(t, err)
	require.Equal(t, content, contentAgain)
}

func TestAdditionalContainerdConfigAcrossTemplates(t *testing.T) {
	additional, err := structpb.NewStruct(map[string]any{
		"timeouts": map[string]any{
			"io.containerd.timeout.shim.cleanup": "5s",
		},
	})
	require.NoError(t, err)

	for _, containerdVersion := range []string{"1.7.22", "2.0.0"} {
		for _, noGPU := range []bool{false, true} {
			name := containerdVersion
			if noGPU {
				name += "-no-GPU"
			} else {
				name += "-GPU"
			}
			t.Run(name, func(t *testing.T) {
				content, err := containerdConfigFromAKSNodeConfig(&aksnodeconfigv1.Configuration{
					ContainerdConfig: &aksnodeconfigv1.ContainerdConfig{
						ContainerdVersion:          containerdVersion,
						AdditionalContainerdConfig: additional,
					},
				}, noGPU, containerdVersion)
				require.NoError(t, err)

				var merged map[string]any
				require.NoError(t, toml.Unmarshal([]byte(content), &merged))
				require.Equal(t, "5s", merged["timeouts"].(map[string]any)["io.containerd.timeout.shim.cleanup"])
			})
		}
	}
}

func TestAdditionalContainerdConfigRejectsCollisionAndNull(t *testing.T) {
	collision, err := structpb.NewStruct(map[string]any{
		"plugins": map[string]any{
			"io.containerd.grpc.v1.cri": map[string]any{
				"sandbox_image": "example.invalid/pause:1",
			},
		},
	})
	require.NoError(t, err)
	_, err = containerdConfigFromAKSNodeConfig(&aksnodeconfigv1.Configuration{
		ContainerdConfig: &aksnodeconfigv1.ContainerdConfig{AdditionalContainerdConfig: collision},
	}, true, "1.7.22")
	require.ErrorContains(t, err, "plugins.io.containerd.grpc.v1.cri.sandbox_image")

	withNull, err := structpb.NewStruct(map[string]any{"new_setting": nil})
	require.NoError(t, err)
	_, err = containerdConfigFromAKSNodeConfig(&aksnodeconfigv1.Configuration{
		ContainerdConfig: &aksnodeconfigv1.ContainerdConfig{AdditionalContainerdConfig: withNull},
	}, true, "1.7.22")
	require.ErrorContains(t, err, "cannot be null")
}

func TestAdditionalSysctlsMerge(t *testing.T) {
	content, err := getSysctlContent(&aksnodeconfigv1.CustomLinuxOsConfig{
		AdditionalSysctls: map[string]string{
			"kernel.pid_max":       "4194304",
			"vm.overcommit_memory": "1",
		},
	})
	require.NoError(t, err)
	decoded, err := base64.StdEncoding.DecodeString(content)
	require.NoError(t, err)
	require.Contains(t, string(decoded), "kernel.pid_max=4194304\n")
	require.Contains(t, string(decoded), "vm.overcommit_memory=1\n")

	_, err = getSysctlContent(&aksnodeconfigv1.CustomLinuxOsConfig{
		AdditionalSysctls: map[string]string{"net.core.somaxconn": "32768"},
	})
	require.ErrorContains(t, err, `collides with generated configuration`)

	_, err = getSysctlContent(&aksnodeconfigv1.CustomLinuxOsConfig{
		AdditionalSysctls: map[string]string{"net core.invalid": "1"},
	})
	require.ErrorContains(t, err, "not a canonical Linux sysctl name")

	_, err = getSysctlContent(&aksnodeconfigv1.CustomLinuxOsConfig{
		AdditionalSysctls: map[string]string{"kernel.pid_max": "1\nkernel.kptr_restrict=0"},
	})
	require.ErrorContains(t, err, "contains an invalid value")
}

func TestAdditionalConfigurationRoundTripAndCSEEnvironment(t *testing.T) {
	additionalKubelet, err := structpb.NewStruct(map[string]any{
		"memorySwap": map[string]any{"swapBehavior": "NoSwap"},
	})
	require.NoError(t, err)
	additionalContainerd, err := structpb.NewStruct(map[string]any{
		"timeouts": map[string]any{"io.containerd.timeout.shim.cleanup": "5s"},
	})
	require.NoError(t, err)
	config := &aksnodeconfigv1.Configuration{
		Version:           "v2",
		KubernetesVersion: "1.34.0",
		KubeletConfig: &aksnodeconfigv1.KubeletConfig{
			EnableKubeletConfigFile: true,
			KubeletConfigFileConfig: &aksnodeconfigv1.KubeletConfigFileConfig{},
			AdditionalKubeletConfig: additionalKubelet,
		},
		ContainerdConfig: &aksnodeconfigv1.ContainerdConfig{
			AdditionalContainerdConfig: additionalContainerd,
		},
		CustomLinuxOsConfig: &aksnodeconfigv1.CustomLinuxOsConfig{
			AdditionalSysctls: map[string]string{"kernel.pid_max": "4194304"},
		},
	}

	payload, err := nodeconfigutils.MarshalConfigurationV1(config)
	require.NoError(t, err)
	restored, err := nodeconfigutils.UnmarshalConfigurationV1(payload)
	require.NoError(t, err)
	require.True(t, proto.Equal(config, restored))

	command, err := BuildCSECmd(context.Background(), restored, nil)
	require.NoError(t, err)
	environment := environToMap(command.Env)
	require.Equal(t, "true", environment["HAS_ADDITIONAL_CONTAINERD_CONFIG"])
	require.Equal(t, "true", environment["HAS_ADDITIONAL_SYSCTLS"])
}

func TestBuildCSECmdFailsClosedOnAdditionalConfigurationCollision(t *testing.T) {
	additional, err := structpb.NewStruct(map[string]any{"kind": "Other"})
	require.NoError(t, err)
	_, err = BuildCSECmd(context.Background(), &aksnodeconfigv1.Configuration{
		KubeletConfig: &aksnodeconfigv1.KubeletConfig{
			EnableKubeletConfigFile: true,
			KubeletConfigFileConfig: &aksnodeconfigv1.KubeletConfigFileConfig{
				Kind: "KubeletConfiguration",
			},
			AdditionalKubeletConfig: additional,
		},
	}, nil)
	require.ErrorContains(t, err, `collides with generated field "kind"`)
}
