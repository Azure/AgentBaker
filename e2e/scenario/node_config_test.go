package scenario

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/aks-node-controller/parser"
	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/aks-node-controller/pkg/nodeconfigutils"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/stretchr/testify/require"
)

func TestCreateAKSNodeConfigPreservesProvisioningPhase(t *testing.T) {
	for _, bake := range []bool{true, false} {
		t.Run(fmt.Sprintf("bake=%t", bake), func(t *testing.T) {
			nbc, err := baseTemplateLinux("westus2", "1.37.1", "amd64")
			require.NoError(t, err)
			nbc.EnableScriptlessAKSNodeConfig = true
			nbc.PreProvisionOnly = bake
			cfg, err := createAKSNodeConfig(nodeConfigTestCluster("1.37.1"), config.VHDUbuntu2404Gen2Containerd,
				func(_ *Cluster, cfg *aksnodeconfigv1.Configuration) error {
					cfg.PreProvisionOnly = bake
					return nil
				})
			require.NoError(t, err)
			require.Equal(t, bake, cfg.PreProvisionOnly)
			data, err := nodeconfigutils.MarshalConfigurationV1(cfg)
			require.NoError(t, err)
			nbc.AKSNodeConfigJSON = string(data)
			require.NoError(t, agent.ValidateAndSetLinuxNodeBootstrappingConfigurationWithError(nbc))
			require.Equal(t, string(data), nbc.AKSNodeConfigJSON)
			cmd, err := parser.BuildCSECmd(t.Context(), cfg, nil)
			require.NoError(t, err)
			require.Contains(t, cmd.Env, fmt.Sprintf("PRE_PROVISION_ONLY=%t", bake))
		})
	}
}

func TestPrepareNodeConfigurationNBCOptOutPreservesComparison(t *testing.T) {
	originalDisableScriptless := config.Config.DisableScriptless
	config.Config.DisableScriptless = false
	t.Cleanup(func() { config.Config.DisableScriptless = originalDisableScriptless })

	for _, vhd := range []*config.Image{
		config.VHDUbuntu2204Gen2Containerd,
		config.VHDUbuntu2404Gen1Containerd,
		config.VHDUbuntu2604MinimalGen2Containerd,
		config.VHDAzureLinuxV3Gen2,
	} {
		for _, tc := range []struct {
			name    string
			optOut  bool
			withANC bool
			force   bool
		}{
			{name: "ANC default", withANC: true},
			{name: "ANC default without mutators"},
			{name: "explicit NBC opt-out with comparison", optOut: true, withANC: true},
			{name: "explicit NBC opt-out without comparison", optOut: true},
			{name: "sampled NBC overrides both bootstrap mutators", withANC: true, force: true},
			{name: "sampled NBC without comparison", force: true},
		} {
			t.Run(vhd.Name+"/"+tc.name, func(t *testing.T) {
				nbc, err := baseTemplateLinux("westus2", "1.36.5", "amd64")
				require.NoError(t, err)
				nbc.AgentPoolProfile.Distro = vhd.Distro
				nbc.SubscriptionID = "test-subscription"
				nbc.ResourceGroupName = "test-resource-group"
				nbc.ContainerService.Properties.HostedMasterProfile = &datamodel.HostedMasterProfile{
					FQDN: "test-api.example.com",
				}
				s := &Scenario{
					Config: Config{
						VHD:                      vhd,
						ForceScriptlessNBCCSECmd: tc.force,
					},
					Runtime: &ScenarioRuntime{Cluster: nodeConfigTestCluster("1.36.5"), EnableScriptlessNBCCSECmd: true},
				}
				if tc.withANC {
					s.AKSNodeConfigMutator = func(_ *Cluster, cfg *aksnodeconfigv1.Configuration) {
						cfg.NetworkConfig.NetworkPlugin = aksnodeconfigv1.NetworkPlugin_NETWORK_PLUGIN_AZURE
					}
				}
				if tc.optOut {
					s.BootstrapConfigMutator = func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
						nbc.EnableScriptlessAKSNodeConfig = false
					}
				}
				if tc.force {
					s.BootstrapConfigMutator = func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
						nbc.EnableScriptlessAKSNodeConfig = true
					}
					s.BootstrapConfigMutatorWithError = func(_ context.Context, _ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) error {
						nbc.EnableScriptlessAKSNodeConfig = true
						nbc.EnableScriptlessNBCCSECmd = false
						return nil
					}
				}

				require.NoError(t, prepareNodeConfiguration(t.Context(), s, nbc))
				wantNBC := tc.optOut || tc.force
				require.Equal(t, !wantNBC, nbc.EnableScriptlessAKSNodeConfig)
				require.Equal(t, wantNBC, nbc.EnableScriptlessNBCCSECmd)
				require.Equal(t, wantNBC, usesScriptlessNBCCSECmd(s))
				require.False(t, nbc.EnableScriptlessCSECmd)
				if tc.withANC || !wantNBC {
					require.NotNil(t, s.Runtime.AKSNodeConfig)
					require.True(t, json.Valid([]byte(nbc.AKSNodeConfigJSON)))
					if tc.withANC {
						require.Equal(t, aksnodeconfigv1.NetworkPlugin_NETWORK_PLUGIN_AZURE, s.Runtime.AKSNodeConfig.NetworkConfig.NetworkPlugin)
						require.Contains(t, nbc.AKSNodeConfigJSON, "NETWORK_PLUGIN_AZURE")
					}
				} else {
					require.Nil(t, s.Runtime.AKSNodeConfig)
					require.Empty(t, nbc.AKSNodeConfigJSON)
				}
			})
		}
	}
}

func TestPrepareNodeConfigurationPreservesCachingMode(t *testing.T) {
	for _, phase3 := range []bool{false, true} {
		for _, bake := range []bool{false, true} {
			t.Run(fmt.Sprintf("phase3=%t/bake=%t", phase3, bake), func(t *testing.T) {
				nbc, err := baseTemplateLinux("westus2", "1.36.5", "amd64")
				require.NoError(t, err)
				nbc.PreProvisionOnly = bake
				nbc.SubscriptionID = "test-subscription"
				nbc.ResourceGroupName = "test-resource-group"
				nbc.ContainerService.Properties.HostedMasterProfile = &datamodel.HostedMasterProfile{
					FQDN: "test-api.example.com",
				}
				s := &Scenario{
					Config: Config{
						VHD:                  config.VHDUbuntu2604MinimalGen2Containerd,
						VHDCaching:           true,
						AKSNodeConfigMutator: func(_ *Cluster, _ *aksnodeconfigv1.Configuration) {},
					},
					Runtime: &ScenarioRuntime{Cluster: nodeConfigTestCluster("1.36.5"), EnableScriptlessNBCCSECmd: true},
				}
				if phase3 {
					s.BootstrapConfigMutator = func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
						nbc.EnableScriptlessAKSNodeConfig = true
					}
				}
				require.NoError(t, prepareNodeConfiguration(t.Context(), s, nbc))
				require.Equal(t, phase3, nbc.EnableScriptlessAKSNodeConfig)
				require.Equal(t, !phase3, nbc.EnableScriptlessNBCCSECmd)
				require.Equal(t, bake, s.Runtime.AKSNodeConfig.PreProvisionOnly)
			})
		}
	}
}

func TestForcedNBCRejectsUnsupportedProvisioning(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Scenario, *datamodel.NodeBootstrappingConfiguration)
	}{
		{"disabled", func(_ *Scenario, _ *datamodel.NodeBootstrappingConfiguration) { config.Config.DisableScriptless = true }},
		{"pre-provision run", func(_ *Scenario, _ *datamodel.NodeBootstrappingConfiguration) { config.Config.TestPreProvision = true }},
		{"pre-provision config", func(_ *Scenario, nbc *datamodel.NodeBootstrappingConfiguration) { nbc.PreProvisionOnly = true }},
		{"VHD caching", func(s *Scenario, _ *datamodel.NodeBootstrappingConfiguration) { s.VHDCaching = true }},
		{"CustomData only", func(_ *Scenario, nbc *datamodel.NodeBootstrappingConfiguration) {
			nbc.CustomDataOnlyProvisioning = true
		}},
		{"unsupported image", func(s *Scenario, _ *datamodel.NodeBootstrappingConfiguration) { s.VHD = config.VHDAzureLinuxV2Gen2 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := config.Config
			config.Config = config.DefaultConfiguration()
			t.Cleanup(func() { config.Config = original })
			nbc, err := baseTemplateLinux("westus2", "1.36.5", "amd64")
			require.NoError(t, err)
			s := &Scenario{
				Config:  Config{VHD: config.VHDUbuntu2204Gen2Containerd, ForceScriptlessNBCCSECmd: true},
				Runtime: &ScenarioRuntime{},
			}
			tc.change(s, nbc)
			require.ErrorContains(t, prepareNodeConfiguration(t.Context(), s, nbc), "cannot use forced scriptless NBC-command")
		})
	}
}

func TestLinuxKubeletConfigFileDefaults(t *testing.T) {
	for _, version := range []string{"1.33.12", "1.36.5", "1.37.1"} {
		for _, arch := range []string{"amd64", "arm64"} {
			t.Run(version+"/"+arch, func(t *testing.T) {
				nbc, err := baseTemplateLinux("westus2", version, arch)
				require.NoError(t, err)
				require.True(t, nbc.EnableKubeletConfigFile)
				vhd := *config.VHDUbuntu2404Gen2Containerd
				vhd.Arch = arch
				cfg, err := createAKSNodeConfig(nodeConfigTestCluster(version), &vhd)
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

func TestCreateAKSNodeConfigPreservesScenarioOverrides(t *testing.T) {
	cfg, err := createAKSNodeConfig(nodeConfigTestCluster("1.37.1"), config.VHDUbuntu2404Gen2Containerd,
		func(_ *Cluster, cfg *aksnodeconfigv1.Configuration) error {
			file := cfg.KubeletConfig.KubeletConfigFileConfig
			file.MaxPods = to.Ptr(int32(73))
			file.KubeReserved = map[string]string{"cpu": "200m", "memory": "1Gi"}
			file.ClusterDns = []string{"169.254.10.11"}
			file.ServerTlsBootstrap = false
			file.FailSwapOn = to.Ptr(false)
			return nil
		})
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
			cfg, err := createAKSNodeConfig(nodeConfigTestCluster("1.37.1"), scenario.VHD,
				func(cluster *Cluster, cfg *aksnodeconfigv1.Configuration) error {
					scenario.AKSNodeConfigMutator(cluster, cfg)
					return nil
				})
			require.NoError(t, err)
			require.False(t, cfg.KubeletConfig.EnableKubeletConfigFile)
			require.Equal(t, "110", cfg.KubeletConfig.KubeletFlags["--max-pods"])
			require.Equal(t, "pods,kube-reserved,system-reserved", cfg.KubeletConfig.KubeletFlags["--enforce-node-allocatable"])
		})
	}
}

func TestLocalDNSKubeletSettingsPreserveConfigFileParity(t *testing.T) {
	for _, registered := range List() {
		if !strings.HasPrefix(registered.Name, "LocalDNS/") &&
			!strings.HasPrefix(registered.Name, "LocalDNSNoHostsPlugin/") {
			continue
		}
		t.Run(registered.Name, func(t *testing.T) {
			nbc, err := baseTemplateLinux("eastus", "1.34.0", "amd64")
			require.NoError(t, err)
			registered.BootstrapConfigMutator(nil, nbc)
			cfg, err := createAKSNodeConfig(nodeConfigTestCluster("1.34.0"), registered.VHD,
				func(cluster *Cluster, cfg *aksnodeconfigv1.Configuration) error {
					registered.AKSNodeConfigMutator(cluster, cfg)
					return nil
				})
			require.NoError(t, err)
			require.True(t, cfg.KubeletConfig.EnableKubeletConfigFile)
			require.NotContains(t, cfg.KubeletConfig.KubeletFlags, "--cluster-dns")
			require.NotContains(t, cfg.KubeletConfig.KubeletFlags, "--cluster-dns",
				"LocalDNS must not reintroduce a flag translated into the config file")
			require.Equal(t, []string{localDNSClusterListenerIP},
				cfg.KubeletConfig.KubeletConfigFileConfig.ClusterDns)
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
		cfg, err := createAKSNodeConfig(nodeConfigTestCluster("1.34.0"), config.VHDUbuntu2404Gen2Containerd)
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
