package nodeconfigutils

import (
	"testing"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestValidateAndNormalizeConfiguration(t *testing.T) {
	for _, version := range []string{"1.10.0", "1.11.0", "1.20.0", "1.24.0", "1.25.0", "1.34.0", "1.34.0-rc.1"} {
		t.Run(version, func(t *testing.T) {
			cfg := &aksnodeconfigv1.Configuration{
				Version: "v1", KubernetesVersion: version,
				KubeletConfig: &aksnodeconfigv1.KubeletConfig{
					EnableKubeletConfigFile: true,
					KubeletFlags: map[string]string{
						"--dynamic-config-dir": "old", "--non-masquerade-cidr": "old",
						"--cni-bin-dir": "old", "--cni-cache-dir": "old", "--cni-conf-dir": "old",
						"--docker-endpoint": "old", "--image-pull-progress-deadline": "old",
						"--network-plugin": "old", "--network-plugin-mtu": "old",
						"--feature-gates":                     "Other=true,DynamicKubeletConfig=true",
						"--streaming-connection-idle-timeout": "4h",
						"--max-pods":                          "30",
					},
					KubeletConfigFileConfig: &aksnodeconfigv1.KubeletConfigFileConfig{
						ServerTlsBootstrap:             true,
						FeatureGates:                   map[string]bool{"Other": true, "DynamicKubeletConfig": true},
						StreamingConnectionIdleTimeout: "4h",
					},
				},
			}
			require.NoError(t, ValidateAndNormalizeConfiguration(cfg))
			flags := cfg.KubeletConfig.KubeletFlags
			for _, key := range []string{
				"--dynamic-config-dir", "--non-masquerade-cidr", "--cni-bin-dir", "--cni-cache-dir", "--cni-conf-dir",
				"--docker-endpoint", "--image-pull-progress-deadline", "--network-plugin", "--network-plugin-mtu",
			} {
				require.NotContains(t, flags, key)
			}
			require.Equal(t, "30", flags["--max-pods"])
			require.Contains(t, flags["--feature-gates"], "Other=true")
			require.Contains(t, flags["--feature-gates"], "RotateKubeletServerCertificate=true")
			file := cfg.KubeletConfig.KubeletConfigFileConfig
			require.True(t, file.FeatureGates["RotateKubeletServerCertificate"])
			switch version {
			case "1.10.0":
				require.True(t, file.FeatureGates["DynamicKubeletConfig"])
				require.Contains(t, flags["--feature-gates"], "DynamicKubeletConfig=true")
			case "1.11.0", "1.20.0":
				require.Contains(t, flags["--feature-gates"], "DynamicKubeletConfig=false")
				require.Contains(t, file.FeatureGates, "DynamicKubeletConfig")
				require.False(t, file.FeatureGates["DynamicKubeletConfig"])
			default:
				require.NotContains(t, flags["--feature-gates"], "DynamicKubeletConfig")
				require.NotContains(t, file.FeatureGates, "DynamicKubeletConfig")
			}
			if version == "1.20.0" || version == "1.24.0" {
				require.Contains(t, flags["--feature-gates"], "DisableAcceleratorUsageMetrics=false")
				require.Contains(t, file.FeatureGates, "DisableAcceleratorUsageMetrics")
				require.False(t, file.FeatureGates["DisableAcceleratorUsageMetrics"])
			} else {
				require.NotContains(t, flags["--feature-gates"], "DisableAcceleratorUsageMetrics")
			}
			if version == "1.34.0" {
				require.NotContains(t, flags, "--streaming-connection-idle-timeout")
				require.Empty(t, file.StreamingConnectionIdleTimeout)
			} else {
				require.Equal(t, "4h", flags["--streaming-connection-idle-timeout"])
				require.Equal(t, "4h", file.StreamingConnectionIdleTimeout)
			}
			before := proto.Clone(cfg)
			require.NoError(t, ValidateAndNormalizeConfiguration(cfg))
			require.True(t, proto.Equal(before, cfg), "normalization must be idempotent")
		})
	}
}

func TestValidateAndNormalizeConfigurationErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  *aksnodeconfigv1.Configuration
		want string
	}{
		{"nil", nil, "AKSNodeConfig is required"},
		{"version", &aksnodeconfigv1.Configuration{Version: "bad"}, "unsupported version"},
		{"kubernetes", &aksnodeconfigv1.Configuration{
			Version: "v1", KubernetesVersion: "bad", KubeletConfig: &aksnodeconfigv1.KubeletConfig{},
		}, "invalid kubernetes_version"},
		{"hugepages", &aksnodeconfigv1.Configuration{Version: "v1", CustomLinuxOsConfig: &aksnodeconfigv1.CustomLinuxOsConfig{TransparentHugepageSupport: "bad"}}, "transparent_hugepage_support"},
		{"defrag", &aksnodeconfigv1.Configuration{Version: "v1", CustomLinuxOsConfig: &aksnodeconfigv1.CustomLinuxOsConfig{TransparentDefrag: "bad"}}, "transparent_defrag"},
		{"gate", &aksnodeconfigv1.Configuration{
			Version: "v1", KubernetesVersion: "1.34.0",
			KubeletConfig: &aksnodeconfigv1.KubeletConfig{KubeletFlags: map[string]string{"--feature-gates": "Other=bad"}},
		}, "invalid kubelet feature gate"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.ErrorContains(t, ValidateAndNormalizeConfiguration(tc.cfg), tc.want)
		})
	}
	for _, enabled := range []string{"", "always", "madvise", "never"} {
		for _, defrag := range []string{"", "always", "defer", "defer+madvise", "madvise", "never"} {
			require.NoError(t, ValidateAndNormalizeConfiguration(&aksnodeconfigv1.Configuration{
				Version: "v0",
				CustomLinuxOsConfig: &aksnodeconfigv1.CustomLinuxOsConfig{
					TransparentHugepageSupport: enabled, TransparentDefrag: defrag,
				},
			}))
		}
	}
}

func TestNormalizeReservedCgroups(t *testing.T) {
	for _, tc := range []struct {
		name     string
		flags    map[string]string
		file     bool
		enforced []string
		want     bool
	}{
		{"flags", map[string]string{"--enforce-node-allocatable": "[pods, kube-reserved, system-reserved]"}, false, nil, true},
		{"partial", map[string]string{"--enforce-node-allocatable": "pods,kube-reserved"}, false, nil, false},
		{"file", nil, true, []string{"pods", "kube-reserved", "system-reserved"}, true},
		{"flags override file", map[string]string{"--enforce-node-allocatable": "pods"}, true, []string{"kube-reserved", "system-reserved"}, false},
		{"inactive file", nil, false, []string{"kube-reserved", "system-reserved"}, false},
		{"nil flags", nil, true, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &aksnodeconfigv1.Configuration{
				Version: "v1", KubernetesVersion: "1.34.0",
				KubeletConfig: &aksnodeconfigv1.KubeletConfig{
					EnableKubeletConfigFile: tc.file,
					KubeletFlags:            tc.flags,
					KubeletConfigFileConfig: &aksnodeconfigv1.KubeletConfigFileConfig{
						EnforceNodeAllocatable: tc.enforced,
						KubeReservedCgroup:     "untrusted", SystemReservedCgroup: "untrusted",
					},
				},
			}
			if tc.flags != nil {
				tc.flags["--kube-reserved-cgroup"] = "untrusted"
				tc.flags["--system-reserved-cgroup"] = "untrusted"
			}
			require.NoError(t, ValidateAndNormalizeConfiguration(cfg))
			file := cfg.KubeletConfig.KubeletConfigFileConfig
			if tc.want {
				require.Equal(t, "/kubereserved.slice", file.KubeReservedCgroup)
				require.Equal(t, "/system.slice", file.SystemReservedCgroup)
			} else {
				require.Empty(t, file.KubeReservedCgroup)
				require.Empty(t, file.SystemReservedCgroup)
			}
			if tc.want && tc.flags != nil {
				require.Equal(t, "/kubereserved.slice", tc.flags["--kube-reserved-cgroup"])
				require.Equal(t, "/system.slice", tc.flags["--system-reserved-cgroup"])
			} else {
				require.NotContains(t, cfg.KubeletConfig.KubeletFlags, "--kube-reserved-cgroup")
				require.NotContains(t, cfg.KubeletConfig.KubeletFlags, "--system-reserved-cgroup")
			}
		})
	}
}
