package parser

import (
	"testing"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/stretchr/testify/require"
)

func TestKataCCRuntimeContract(t *testing.T) {
	for _, version := range []string{"1.7.0", "2.0.0"} {
		for _, noGPU := range []bool{true, false} {
			config := &aksnodeconfigv1.Configuration{
				IsKata: true, KubernetesVersion: "1.33.0",
				WorkloadRuntime: aksnodeconfigv1.WorkloadRuntime_WORKLOAD_RUNTIME_KATA_CC_ISOLATION,
			}
			rendered, err := containerdConfigFromAKSNodeConfig(config, noGPU, version)
			require.NoError(t, err)
			plugin := "io.containerd.grpc.v1.cri"
			if version == "2.0.0" {
				plugin = "io.containerd.cri.v1.runtime"
			}
			require.Contains(t, rendered, `[plugins."`+plugin+`".containerd.runtimes.kata-cc]`)
			require.Contains(t, rendered, `runtime_type = "io.containerd.kata-cc.v2"`)
			require.Contains(t, rendered, "configuration-clh-snp.toml")
			// Ordinary Kata's v2 configuration remains unchanged.
			config.WorkloadRuntime = aksnodeconfigv1.WorkloadRuntime_WORKLOAD_RUNTIME_OCI_CONTAINER
			ordinary, err := containerdConfigFromAKSNodeConfig(config, noGPU, version)
			require.NoError(t, err)
			if version == "2.0.0" {
				require.NotContains(t, ordinary, `[plugins."io.containerd.cri.v1.runtime".containerd.runtimes.kata-cc]`)
			}
		}
	}
}
