package agent

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"maps"
	"regexp"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbaker/staging"
	"github.com/stretchr/testify/require"
)

func newWindowsKubeletConfigTestInput() *datamodel.NodeBootstrappingConfiguration {
	config := newNodeCustomDataRenderConfig(datamodel.AKSWindows2022Containerd)
	config.AgentPoolProfile.OSType = datamodel.Windows
	config.ContainerService.Properties.WindowsProfile = &datamodel.WindowsProfile{}
	config.SecureTLSBootstrappingConfig = &datamodel.SecureTLSBootstrappingConfig{}
	config.EnabledFeatures = map[string]string{
		"KUBELET_FLAGS_TO_OMIT": base64.RawStdEncoding.EncodeToString([]byte(`["--volume-plugin-dir","--container-runtime-endpoint"]`)),
	}
	config.EnableKubeletConfigFile = true
	config.KubeletConfig = map[string]string{
		"--volume-plugin-dir":          `c:\k\volumeplugins`,
		"--container-runtime-endpoint": "npipe://./pipe/containerd-containerd",
		"--anonymous-auth":             "false",
	}
	return config
}

func renderWindowsKubeletTestPayload(test *testing.T, config *datamodel.NodeBootstrappingConfiguration) string {
	test.Helper()
	encoded := InitializeTemplateGenerator().getWindowsNodeBootstrappingPayload(config)
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	require.NoError(test, err)
	return string(decoded)
}

func setWindowsKubeletTestOverrides(config *datamodel.NodeBootstrappingConfiguration, flags map[string]string) {
	config.ContainerService.Properties.CustomConfiguration = &datamodel.CustomConfiguration{
		WindowsKubernetesConfigurations: map[string]*datamodel.ComponentConfiguration{
			string(datamodel.Componentkubelet): {Config: flags},
		},
	}
}

func TestWindowsKubeletMigrationFallback(test *testing.T) {
	for _, scenario := range []struct {
		name   string
		modify func(*datamodel.NodeBootstrappingConfiguration)
	}{
		{name: "absent", modify: func(config *datamodel.NodeBootstrappingConfiguration) { config.EnabledFeatures = nil }},
		{name: "empty", modify: func(config *datamodel.NodeBootstrappingConfiguration) {
			config.EnabledFeatures["KUBELET_FLAGS_TO_OMIT"] = "W10"
		}},
		{name: "invalid", modify: func(config *datamodel.NodeBootstrappingConfiguration) {
			config.EnabledFeatures["KUBELET_FLAGS_TO_OMIT"] = "$(throw)"
		}},
		{name: "mode off", modify: func(config *datamodel.NodeBootstrappingConfiguration) { config.EnableKubeletConfigFile = false }},
		{name: "mode off with custom settings", modify: func(config *datamodel.NodeBootstrappingConfiguration) {
			config.EnableKubeletConfigFile = false
			config.AgentPoolProfile.CustomKubeletConfig = &datamodel.CustomKubeletConfig{CPUManagerPolicy: "static"}
		}},
		{name: "missing values", modify: func(config *datamodel.NodeBootstrappingConfiguration) { config.KubeletConfig = nil }},
		{name: "nonmatching values", modify: func(config *datamodel.NodeBootstrappingConfiguration) {
			config.KubeletConfig["--volume-plugin-dir"] = `d:\custom`
			config.KubeletConfig["--container-runtime-endpoint"] = "npipe://custom"
		}},
		{name: "raw config", modify: func(config *datamodel.NodeBootstrappingConfiguration) { config.KubeletConfig["--config"] = "" }},
		{name: "raw config directory", modify: func(config *datamodel.NodeBootstrappingConfiguration) { config.KubeletConfig["--config-dir"] = "" }},
		{name: "custom config", modify: func(config *datamodel.NodeBootstrappingConfiguration) {
			setWindowsKubeletTestOverrides(config, map[string]string{"--config": ""})
		}},
		{name: "custom config directory", modify: func(config *datamodel.NodeBootstrappingConfiguration) {
			setWindowsKubeletTestOverrides(config, map[string]string{"--config-dir": `c:\custom`})
		}},
		{name: "custom targets", modify: func(config *datamodel.NodeBootstrappingConfiguration) {
			setWindowsKubeletTestOverrides(config, maps.Clone(config.KubeletConfig))
		}},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			config := newWindowsKubeletConfigTestInput()
			scenario.modify(config)
			require.False(test, hasWindowsKubeletConfiguration(config))
			payload := renderWindowsKubeletTestPayload(test, config)
			config.EnabledFeatures = nil
			require.Equal(test, renderWindowsKubeletTestPayload(test, config), payload)
			require.NotContains(test, payload, "Set-WindowsKubeletConfiguration")
		})
	}
}

func TestWindowsKubeletMigrationUnrequestedVersions(test *testing.T) {
	for _, version := range []string{"1.31.0", "1.37.99", "1.38.0-beta.0", "1.38.0", "1.39.0-beta.0"} {
		config := newWindowsKubeletConfigTestInput()
		config.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = version
		config.EnabledFeatures = nil
		require.False(test, hasWindowsKubeletConfiguration(config))
		require.NotContains(test, renderWindowsKubeletTestPayload(test, config), "Set-WindowsKubeletConfiguration")
	}
}

func TestWindowsKubeletMigrationPreservesNativeSettings(test *testing.T) {
	for _, updateFrequency := range []string{"10s", "1m"} {
		config := newWindowsKubeletConfigTestInput()
		config.KubeletConfig["--node-status-update-frequency"] = updateFrequency
		config.KubeletConfig["--node-status-report-frequency"] = "17m"
		config.KubeletConfig["--cgroups-per-qos"] = "false"
		config.KubeletConfig["--enforce-node-allocatable"] = `""""`
		config.KubeletConfig["--resolv-conf"] = `""""`
		config.KubeletConfig["--hairpin-mode"] = "promiscuous-bridge"
		config.KubeletConfig["--register-with-taints"] = "example.com/init=:NoSchedule"
		setWindowsKubeletTestOverrides(config, map[string]string{"--rotate-server-certificates": "true", "--container-log-max-files": "7"})
		zeroFiles, logSize := int32(0), int32(10)
		config.AgentPoolProfile.CustomKubeletConfig = &datamodel.CustomKubeletConfig{
			ContainerLogMaxFiles: &zeroFiles, ContainerLogMaxSizeMB: &logSize, CPUManagerPolicy: "static",
		}
		original := maps.Clone(config.KubeletConfig)
		payload := renderWindowsKubeletTestPayload(test, config)
		require.Equal(test, original, config.KubeletConfig)
		contents := decodeWindowsKubeletTestContent(test, payload)
		require.Equal(test, datamodel.Duration("5m"), contents.NodeStatusReportFrequency)
		require.Equal(test, datamodel.Duration(updateFrequency), contents.NodeStatusUpdateFrequency)
		require.Equal(test, "10Mi", contents.ContainerLogMaxSize)
		require.NotNil(test, contents.ContainerLogMaxFiles)
		require.Zero(test, *contents.ContainerLogMaxFiles)
		require.Empty(test, contents.CPUManagerPolicy)
		require.Empty(test, contents.EnforceNodeAllocatable)
		require.Empty(test, contents.ResolverConfig)
		require.NotNil(test, contents.CgroupsPerQOS)
		require.False(test, *contents.CgroupsPerQOS)
		require.Equal(test, `c:\k\volumeplugins`, contents.VolumePluginDir)
		require.Equal(test, "npipe://./pipe/containerd-containerd", contents.ContainerRuntimeEndpoint)
		for _, flag := range []string{"--anonymous-auth=false", "--hairpin-mode=promiscuous-bridge", "--register-with-taints=example.com/init=:NoSchedule",
			"--container-log-max-size=10Mi", "--container-log-max-files=0", "--rotate-server-certificates=true"} {
			require.Contains(test, payload, `"`+flag+`"`)
		}
		require.NotContains(test, payload, `"--volume-plugin-dir=`)
		require.NotContains(test, payload, `"--container-runtime-endpoint=`)
		require.NotContains(test, payload, `"--node-status-report-frequency=`)
		require.Contains(test, payload, `$global:EnableKubeletServingCertificateRotation=[System.Convert]::ToBoolean("true")`)
	}
}

func TestWindowsKubeletMigrationExistingNormalization(test *testing.T) {
	for _, customGates := range []bool{false, true} {
		config := newWindowsKubeletConfigTestInput()
		config.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = "1.38.0"
		config.KubeletConfig["--feature-gates"] = "DynamicKubeletConfig=true"
		config.KubeletConfig["--rotate-server-certificates"] = "true"
		config.KubeletConfig["--streaming-connection-idle-timeout"] = "4h"
		if customGates {
			setWindowsKubeletTestOverrides(config, map[string]string{"--feature-gates": "CustomTestGate=false"})
		}
		validateAndSetWindowsNodeBootstrappingConfiguration(config)
		legacy := *config
		legacy.KubeletConfig = maps.Clone(config.KubeletConfig)
		for name := range windowsKubeletMigrationValues(config) {
			delete(legacy.KubeletConfig, name)
		}
		expectedCLI := legacy.GetOrderedKubeletConfigStringForPowershell(config.AgentPoolProfile.CustomKubeletConfig)
		payload := renderWindowsKubeletTestPayload(test, config)
		require.Contains(test, payload, "$global:KubeletConfigArgs=@( "+expectedCLI+" )")
		contents := decodeWindowsKubeletTestContent(test, payload)
		require.Empty(test, contents.StreamingConnectionIdleTimeout)
		require.Equal(test, strKeyValToMapBool(legacy.KubeletConfig["--feature-gates"], ",", "="), contents.FeatureGates)
		require.True(test, contents.ServerTLSBootstrap)
	}
}

func decodeWindowsKubeletTestContent(test *testing.T, payload string) *datamodel.AKSKubeletConfiguration {
	test.Helper()
	match := regexp.MustCompile(`Set-WindowsKubeletConfiguration -ContentBase64 "([A-Za-z0-9+/=]+)"`).FindStringSubmatch(payload)
	require.Len(test, match, 2)
	contents, err := base64.StdEncoding.DecodeString(match[1])
	require.NoError(test, err)
	var configuration datamodel.AKSKubeletConfiguration
	require.NoError(test, json.Unmarshal(contents, &configuration))
	return &configuration
}

func TestWindowsKubeletMigrationKeepsUnselectedOverrides(test *testing.T) {
	for _, custom := range []bool{false, true} {
		config := newWindowsKubeletConfigTestInput()
		if custom {
			setWindowsKubeletTestOverrides(config, map[string]string{"--volume-plugin-dir": `c:\k\volumeplugins`})
		} else {
			config.KubeletConfig["--volume-plugin-dir"] = `d:\custom`
		}
		selected := windowsKubeletMigrationValues(config)
		require.Equal(test, map[string]string{"--container-runtime-endpoint": "npipe://./pipe/containerd-containerd"}, selected)
		payload := renderWindowsKubeletTestPayload(test, config)
		require.Contains(test, payload, `"--volume-plugin-dir=`)
		require.NotContains(test, payload, `"--container-runtime-endpoint=`)
	}
}

func TestWindowsKubeletConfigurationRejectsOversizedCustomData(test *testing.T) {
	config := newWindowsKubeletConfigTestInput()
	config.KubeletConfig["--pod-manifest-path"] = strings.Repeat("x", 65536)
	result, err := (&agentBakerImpl{}).GetNodeBootstrapping(context.Background(), config)
	require.ErrorContains(test, err, "CustomData size limit")
	require.Nil(test, result)
}

func windowsKubeletTestZipEntries(test *testing.T, encoded string) map[string][]byte {
	test.Helper()
	contents, err := base64.StdEncoding.DecodeString(encoded)
	require.NoError(test, err)
	archive, err := zip.NewReader(bytes.NewReader(contents), int64(len(contents)))
	require.NoError(test, err)
	entries := make(map[string][]byte)
	for _, entry := range archive.File {
		reader, openErr := entry.Open()
		require.NoError(test, openErr)
		content, readErr := io.ReadAll(reader)
		require.NoError(test, readErr)
		require.NoError(test, reader.Close())
		require.NotContains(test, entries, entry.Name)
		entries[entry.Name] = content
	}
	return entries
}

func TestWindowsKubeletConfigurationBootstrapScripts(test *testing.T) {
	config := newWindowsKubeletConfigTestInput()
	baseScripts, ok := getContainerServiceFuncMap(config)["GetKubernetesWindowsAgentFunctions"].(func() string)
	require.True(test, ok)
	original := baseScripts()
	encoded, err := appendWindowsKubeletConfigScripts(original)
	require.NoError(test, err)
	originalEntries := windowsKubeletTestZipEntries(test, original)
	entries := windowsKubeletTestZipEntries(test, encoded)
	require.Len(test, entries, len(originalEntries)+2)
	for name, content := range originalEntries {
		require.Equal(test, content, entries[name])
	}
	for _, path := range []string{"cse/windows/kubeletconfig.ps1", "cse/windows/provisioningscripts/kubeletstart.ps1"} {
		contents, readErr := staging.WindowsKubeletScripts.ReadFile(path)
		require.NoError(test, readErr)
		require.Equal(test, contents, entries["windows/kubeletconfiguration/"+path[strings.LastIndex(path, "/")+1:]])
	}
	for _, preProvision := range []bool{false, true} {
		config.PreProvisionOnly = preProvision
		payload := renderWindowsKubeletTestPayload(test, config)
		require.Contains(test, payload, encoded)
		nodePrepStart := strings.Index(payload, "function NodePrep {")
		require.GreaterOrEqual(test, nodePrepStart, 0)
		nodePrep := payload[nodePrepStart:]
		require.Less(test, strings.Index(nodePrep, "    Write-KubeClusterConfig"), strings.Index(nodePrep, "    Set-WindowsKubeletConfiguration"))
		require.Less(test, strings.Index(nodePrep, "    Set-WindowsKubeletConfiguration"), strings.Index(nodePrep, "    Install-KubernetesServices"))
		require.Contains(test, payload, "if (-not $PreProvisionOnly)")
		require.LessOrEqual(test, len(payload), 65535)
		test.Logf("active customData=%d bytes, bootstrap ZIP base64 increase=%d", len(payload), len(encoded)-len(original))
	}
	config.EnabledFeatures = nil
	require.Contains(test, renderWindowsKubeletTestPayload(test, config), original)
}
