package scenario

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/aks-node-controller/parser"
	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/stretchr/testify/require"
)

func TestLinuxKubeletConfigFileDefaults(t *testing.T) {
	for _, version := range []string{"1.33.12", "1.36.5", "1.37.1"} {
		for _, arch := range []string{"amd64", "arm64"} {
			t.Run(version+"/"+arch, func(t *testing.T) {
				nbc, err := baseTemplateLinux("westus2", version, arch)
				require.NoError(t, err)
				require.True(t, nbc.EnableKubeletConfigFile)
				cfg, err := nbcToAKSNodeConfigV1(nbc)
				require.NoError(t, err)
				require.True(t, cfg.KubeletConfig.EnableKubeletConfigFile)
				require.NotContains(t, cfg.KubeletConfig.KubeletFlags, "--max-pods")
				require.NotContains(t, cfg.KubeletConfig.KubeletFlags, "--kube-reserved")
				cmd, err := parser.BuildCSECmd(t.Context(), cfg, nil)
				require.NoError(t, err)
				env := make(map[string]string)
				for _, entry := range cmd.Env {
					key, value, _ := strings.Cut(entry, "=")
					env[key] = value
				}
				require.Equal(t, "true", env["KUBELET_CONFIG_FILE_ENABLED"])
				content, err := base64.StdEncoding.DecodeString(env["KUBELET_CONFIG_FILE_CONTENT"])
				require.NoError(t, err)
				var file datamodel.AKSKubeletConfiguration
				require.NoError(t, json.Unmarshal(content, &file))
				require.Equal(t, "KubeletConfiguration", file.Kind)
				require.Equal(t, int32(110), file.MaxPods)
				require.Equal(t, clusterDNSServiceIP, file.ClusterDNS[0])
				require.Equal(t, "1638Mi", file.KubeReserved["memory"])
			})
		}
	}
}

func TestKubeletConfigFileConversionPreservesScenarioOverrides(t *testing.T) {
	nbc, err := baseTemplateLinux("westus2", "1.37.1", "amd64")
	require.NoError(t, err)
	nbc.KubeletConfig["--max-pods"] = "73"
	nbc.KubeletConfig["--kube-reserved"] = "cpu=200m,memory=1Gi"
	nbc.KubeletConfig["--cluster-dns"] = "169.254.10.11"
	nbc.KubeletConfig["--rotate-server-certificates"] = "false"
	nbc.AgentPoolProfile.CustomKubeletConfig = &datamodel.CustomKubeletConfig{FailSwapOn: to.Ptr(false)}
	cfg, err := nbcToAKSNodeConfigV1(nbc)
	require.NoError(t, err)
	file := cfg.KubeletConfig.KubeletConfigFileConfig
	require.Equal(t, int32(73), file.GetMaxPods())
	require.Equal(t, map[string]string{"cpu": "200m", "memory": "1Gi"}, file.KubeReserved)
	require.Equal(t, []string{"169.254.10.11"}, file.ClusterDns)
	require.False(t, file.ServerTlsBootstrap)
	require.NotNil(t, file.FailSwapOn)
	require.False(t, *file.FailSwapOn)
}

func TestKubeletConfigFileExplicitOptOut(t *testing.T) {
	for _, name := range []string{
		"Ubuntu2204_NodeHardening_KubeReservedSlice_CLIFlags",
		"Ubuntu2604Minimal_NodeHardening_KubeReservedSlice_CLIFlags",
	} {
		t.Run(name, func(t *testing.T) {
			var scenario *Scenario
			for _, registered := range List() {
				if registered.Name == name {
					scenario = registered
					break
				}
			}
			require.NotNil(t, scenario, "scenario %s must remain registered", name)
			nbc, err := baseTemplateLinux("westus2", "1.37.1", "amd64")
			require.NoError(t, err)
			scenario.BootstrapConfigMutator(nil, nbc)
			require.False(t, nbc.EnableKubeletConfigFile)
			cfg, err := nbcToAKSNodeConfigV1(nbc)
			require.NoError(t, err)
			require.False(t, cfg.KubeletConfig.EnableKubeletConfigFile)
			require.Equal(t, "110", cfg.KubeletConfig.KubeletFlags["--max-pods"])
			require.Equal(t, "pods,kube-reserved,system-reserved", cfg.KubeletConfig.KubeletFlags["--enforce-node-allocatable"])
		})
	}
}

func TestLocalDNSForwardsToClusterDNSServiceIP(t *testing.T) {
	assertCoreDNSForwarding := func(t *testing.T, corefile string) {
		t.Helper()
		require.Contains(t, corefile, "forward . "+clusterDNSServiceIP)
		require.NotContains(t, corefile, "10.0.0.10", "LocalDNS must not use the default service IP")
	}

	t.Run("CSE", func(t *testing.T) {
		nbc, err := baseTemplateLinux("eastus", "1.34.0", "amd64")
		require.NoError(t, err)

		for _, variant := range []struct {
			name         string
			includeHosts bool
		}{
			{name: "base"},
			{name: "with_hosts", includeHosts: true},
		} {
			t.Run(variant.name, func(t *testing.T) {
				// CSE uses this separate top-level profile, not the nested AgentPoolProfiles[0].
				corefile, err := agent.GenerateLocalDNSCoreFile(nbc, nbc.AgentPoolProfile, variant.includeHosts)
				require.NoError(t, err)
				assertCoreDNSForwarding(t, corefile)
			})
		}
	})

	t.Run("ANC", func(t *testing.T) {
		nbc, err := baseTemplateLinux("eastus", "1.34.0", "amd64")
		require.NoError(t, err)
		cfg, err := nbcToAKSNodeConfigV1(nbc)
		require.NoError(t, err)
		require.Equal(t, clusterDNSServiceIP, cfg.GetClusterConfig().GetClusterNetworkConfig().GetCoreDnsServiceIp())

		// Build the command to inspect its rendered corefiles without executing provisioning.
		cmd, err := parser.BuildCSECmd(t.Context(), cfg, nil)
		require.NoError(t, err)
		env := make(map[string]string)
		for _, entry := range cmd.Env {
			key, value, _ := strings.Cut(entry, "=")
			env[key] = value
		}
		for _, key := range []string{"LOCALDNS_GENERATED_COREFILE", "LOCALDNS_COREFILE_BASE", "LOCALDNS_COREFILE_WITH_HOSTS"} {
			t.Run(key, func(t *testing.T) {
				require.NotEmpty(t, env[key])
				corefile, err := base64.StdEncoding.DecodeString(env[key])
				require.NoError(t, err)
				assertCoreDNSForwarding(t, string(corefile))
			})
		}
	})
}
