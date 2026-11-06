package scenario

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/aks-node-controller/parser"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/stretchr/testify/require"
)

func TestNBCANCConfigurationParityRegressions(t *testing.T) {
	original := config.Config
	config.Config.DisableScriptless = false
	t.Cleanup(func() { config.Config = original })

	kubelet := map[string]string{
		"KUBELET_CONFIG_FILE_CONTENT": "{{GetKubeletConfigFileContentBase64}}",
	}
	containerd := map[string]string{
		"CONTAINERD_CONFIG_CONTENT":        "{{GetContainerdConfigContent}}",
		"CONTAINERD_CONFIG_NO_GPU_CONTENT": "{{GetContainerdConfigNoGPUContent}}",
	}
	ulimits := map[string]string{
		"CONTAINERD_ULIMITS": "{{GetContainerdUlimitString}}",
	}
	for _, tc := range []struct {
		name       string
		containerd string
		templates  map[string]string
	}{
		{"Ubuntu2204_NodeHardening_KubeReservedSlice_CLIFlags", "1.7.22", kubelet},
		{"Ubuntu2604Minimal_NodeHardening_KubeReservedSlice_CLIFlags", "2.2.4", kubelet},
		{"Ubuntu2404_TemporaryKubeletDisk", "2.2.4", containerd},
		{"AzureLinuxV3_TemporaryKubeletDisk", "2.2.4", containerd},
		{"Ubuntu2204_CustomNodeConfig", "1.7.22", ulimits},
		{"Ubuntu2604Minimal_CustomNodeConfig", "2.2.4", ulimits},
		{"AzureLinuxV3Gen2Kata", "1.7.22", map[string]string{"IS_VHD": "{{.IsVHDDistro}}"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var registered *Scenario
			for _, s := range List() {
				if s.Name == tc.name {
					registered = s
					break
				}
			}

			require.NotNil(t, registered)
			s := freshScenario(registered)
			s.ForceScriptlessNBCCSECmd = true
			s.Runtime = &ScenarioRuntime{EnableScriptlessNBCCSECmd: true}

			nbc, err := baseTemplateLinux("westus2", "1.36.5", s.VHD.Arch)
			require.NoError(t, err)
			nbc.AgentPoolProfile.Distro = s.VHD.Distro
			nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = s.VHD.Distro
			require.NoError(t, prepareNodeConfiguration(t.Context(), s, nbc))
			require.NotNil(t, s.Runtime.AKSNodeConfig)

			dir := t.TempDir()
			stub := fmt.Sprintf("#!/bin/sh\n[ \"$1\" = \"--version\" ] || exit 1\nprintf 'containerd containerd.io %s test\\n'\n", tc.containerd)
			require.NoError(t, os.WriteFile(filepath.Join(dir, "containerd"), []byte(stub), 0755))
			t.Setenv("PATH", dir)
			cmd, err := parser.BuildCSECmd(t.Context(), s.Runtime.AKSNodeConfig, nil)
			require.NoError(t, err)
			env := make(map[string]string)
			for _, entry := range cmd.Env {
				key, value, _ := strings.Cut(entry, "=")
				env[key] = value
			}
			for key, template := range tc.templates {
				expected, err := agent.RenderLinuxNodeCustomDataTemplate([]byte(template), nbc)
				require.NoError(t, err)
				// The NBC command minifier flattens embedded newlines.
				expected = strings.ReplaceAll(expected, "\n", " ")
				actual, ok := env[key]
				require.True(t, ok, "ANC must render %s", key)
				require.Equal(t, expected, actual, "%s must match the NBC rendering", key)
			}
		})
	}
}

func TestNBCANCParityScenariosRequirePRVHDAndComparisonMode(t *testing.T) {
	original := config.Config
	config.Config.DisableScriptless = false
	config.Config.TestPreProvision = false
	config.Config.SIGVersionTagName = "buildId"
	config.Config.SIGVersionTagValue = "test-build"
	t.Cleanup(func() { config.Config = original })

	for _, name := range []string{
		"Ubuntu2204_NodeHardening_KubeReservedSlice_CLIFlags",
		"Ubuntu2604Minimal_NodeHardening_KubeReservedSlice_CLIFlags",
		"Ubuntu2404_TemporaryKubeletDisk",
		"AzureLinuxV3_TemporaryKubeletDisk",
		"Ubuntu2604Minimal_CustomNodeConfig",
		"AzureLinuxV3Gen2Kata",
	} {
		t.Run(name, func(t *testing.T) {
			var source, regression *Scenario
			for _, s := range List() {
				switch s.Name {
				case name:
					source = s
				case name + "_NBCANCParity":
					regression = s
				}
			}
			require.NotNil(t, source)
			require.NotNil(t, regression)
			require.False(t, source.ForceScriptlessNBCCSECmd, "preserve the original scenario's provisioning mode")
			require.False(t, regression.SkipDefaultValidation, "common validation must check the comparison log")
			require.NotNil(t, regression.Validator, "preserve the original behavior checks")
			require.Empty(t, regression.NBCSamplingOS())

			config.Config.VHDMetadataFile = ""
			require.Contains(t, regression.SkipIf(t.Context()), "requires VHD metadata")
			config.Config.VHDMetadataFile = "test-pr-vhd-metadata.json"
			require.Empty(t, regression.SkipIf(t.Context()))
			config.Config.DisableScriptless = true
			require.Contains(t, regression.SkipIf(t.Context()), "requires scriptless")
			config.Config.DisableScriptless = false

			s := freshScenario(regression)
			s.Runtime = &ScenarioRuntime{EnableScriptlessNBCCSECmd: true}
			nbc, err := baseTemplateLinux("westus2", "1.36.5", s.VHD.Arch)
			require.NoError(t, err)
			nbc.AgentPoolProfile.Distro = s.VHD.Distro
			nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = s.VHD.Distro
			require.NoError(t, prepareNodeConfiguration(t.Context(), s, nbc))
			require.True(t, usesScriptlessNBCCSECmd(s), "parity must not silently switch to config-only Phase 3")
			require.False(t, nbc.EnableScriptlessAKSNodeConfig)
			require.NotNil(t, s.Runtime.AKSNodeConfig, "both configurations are required for comparison")
			require.NotEmpty(t, nbc.AKSNodeConfigJSON)
		})
	}
}
