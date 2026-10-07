package agent

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"regexp"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbaker/staging"
	"github.com/stretchr/testify/require"
)

func newWindowsKubeletConfigTestInput(test *testing.T) *datamodel.NodeBootstrappingConfiguration {
	test.Helper()
	config := newNodeCustomDataRenderConfig(datamodel.AKSWindows2022Containerd)
	config.AgentPoolProfile.OSType = datamodel.Windows
	config.ContainerService.Properties.WindowsProfile = &datamodel.WindowsProfile{}
	config.SecureTLSBootstrappingConfig = &datamodel.SecureTLSBootstrappingConfig{}
	config.EnabledFeatures = map[string]string{}
	config.EnableKubeletConfigFile = true
	config.KubeletConfigFileConfig = &datamodel.AKSKubeletConfiguration{}
	require.NoError(test, json.Unmarshal([]byte(`{
		"kind":"KubeletConfiguration","apiVersion":"kubelet.config.k8s.io/v1beta1",
		"volumePluginDir":"c:\\k\\volumeplugins","containerRuntimeEndpoint":"npipe://./pipe/containerd-containerd",
		"enableServer":false,"containerLogMaxFiles":0,"serializeImagePulls":false,
		"authentication":{"anonymous":{"enabled":false}},
		"evictionHard":{"memory.available":"750Mi"}
	}`), config.KubeletConfigFileConfig))
	return config
}

func renderWindowsKubeletTestPayload(test *testing.T, config *datamodel.NodeBootstrappingConfiguration) string {
	test.Helper()
	encoded := InitializeTemplateGenerator().getWindowsNodeBootstrappingPayload(config)
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	require.NoError(test, err)
	return string(decoded)
}

func TestWindowsKubeletConfigurationLegacyGate(test *testing.T) {
	for _, version := range []string{"1.31.0", "1.37.99", "1.38.0-alpha.1", "1.38.0", "1.38.1", "1.39.0-beta.0"} {
		test.Run(version, func(test *testing.T) {
			for _, configMode := range []bool{false, true} {
				config := newWindowsKubeletConfigTestInput(test)
				config.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = version
				config.EnableKubeletConfigFile = configMode
				config.KubeletConfigFileConfig = nil
				config.KubeletConfig["--config"] = `c:\custom\configuration.json`
				baseline := renderWindowsKubeletTestPayload(test, config)
				config.EnabledFeatures["KUBELET_FLAGS_TO_OMIT"] = "invalid; $(not-evaluated)"
				require.Equal(test, baseline, renderWindowsKubeletTestPayload(test, config))
				flags, err := windowsKubeletFlagsToOmit(config)
				require.NoError(test, err)
				require.Empty(test, flags)
				require.NotContains(test, baseline, "Set-WindowsKubeletConfiguration")
				if !configMode {
					config.KubeletConfigFileConfig = newWindowsKubeletConfigTestInput(test).KubeletConfigFileConfig
					require.Equal(test, baseline, renderWindowsKubeletTestPayload(test, config))
				}
			}
		})
	}
}

func TestWindowsKubeletConfigurationDeliveryWithoutOmissions(test *testing.T) {
	for _, preProvision := range []bool{false, true} {
		for _, request := range []string{"", "W10", "W10="} {
			config := newWindowsKubeletConfigTestInput(test)
			config.PreProvisionOnly = preProvision
			config.EnabledFeatures["KUBELET_FLAGS_TO_OMIT"] = request
			before, err := json.Marshal(config.KubeletConfigFileConfig)
			require.NoError(test, err)
			payload := renderWindowsKubeletTestPayload(test, config)
			match := regexp.MustCompile(`Set-WindowsKubeletConfiguration -ContentBase64 "([A-Za-z0-9+/=]+)"`).FindStringSubmatch(payload)
			require.Len(test, match, 2)
			contents, err := base64.StdEncoding.DecodeString(match[1])
			require.NoError(test, err)
			expected, err := json.MarshalIndent(config.KubeletConfigFileConfig, "", "    ")
			require.NoError(test, err)
			require.Equal(test, expected, contents)
			require.Contains(test, string(contents), `"containerLogMaxFiles": 0`)
			require.Contains(test, string(contents), `"enableServer": false`)
			after, err := json.Marshal(config.KubeletConfigFileConfig)
			require.NoError(test, err)
			require.Equal(test, before, after)
			require.Contains(test, payload, `-FlagsToOmitBase64 "W10"`)
			assertWindowsKubeletNodePrepOrder(test, payload)
			require.LessOrEqual(test, len(payload), 65535)
			test.Logf("active customData bytes=%d; supplied config bytes=%d; preProvisionOnly=%t", len(payload), len(contents), preProvision)
		}
	}
}

func assertWindowsKubeletNodePrepOrder(test *testing.T, payload string) {
	test.Helper()
	nodePrep := strings.Index(payload, "function NodePrep {")
	rotation := strings.Index(payload[nodePrep:], "    Configure-KubeletServingCertificateRotation")
	writeCluster := strings.Index(payload[nodePrep:], "    Write-KubeClusterConfig")
	writeConfig := strings.Index(payload[nodePrep:], "    Set-WindowsKubeletConfiguration")
	installServices := strings.Index(payload[nodePrep:], "    Install-KubernetesServices")
	require.GreaterOrEqual(test, rotation, 0)
	require.Greater(test, writeCluster, rotation)
	require.Greater(test, writeConfig, writeCluster)
	require.Greater(test, installServices, writeConfig)
	require.Contains(test, payload, `-BootstrapDirectory "c:\AzureData\windows\kubeletconfiguration"`)
	require.Contains(test, payload, ". c:\\AzureData\\windows\\kubeletconfiguration\\kubeletconfig.ps1")
	require.Contains(test, payload, "if (-not $PreProvisionOnly)")
}

func TestWindowsKubeletOmissionRequest(test *testing.T) {
	for _, scenario := range []struct {
		name     string
		encoded  string
		expected []string
		invalid  bool
	}{
		{name: "absent", expected: []string{}},
		{name: "raw", encoded: base64.RawStdEncoding.EncodeToString([]byte(`["--volume-plugin-dir"]`)), expected: []string{"--volume-plugin-dir"}},
		{name: "padded", encoded: base64.StdEncoding.EncodeToString([]byte(`["--container-runtime-endpoint"]`)), expected: []string{"--container-runtime-endpoint"}},
		{name: "unknown and duplicate",
			encoded:  base64.RawStdEncoding.EncodeToString([]byte(`["--hairpin-mode","--volume-plugin-dir","--volume-plugin-dir","$(throw)"]`)),
			expected: []string{"--volume-plugin-dir"}},
		{name: "bad encoding", encoded: "$(throw)", invalid: true},
		{name: "newline", encoded: "W10=\n", invalid: true},
		{name: "padding bits", encoded: "W11=", invalid: true},
		{name: "oversized", encoded: strings.Repeat("W", 1025), invalid: true},
		{name: "null", encoded: "bnVsbA", invalid: true},
		{name: "object", encoded: "e30", invalid: true},
		{name: "number", encoded: "WzFd", invalid: true},
		{name: "null name", encoded: "W251bGxd", invalid: true},
		{name: "nested", encoded: "W1tdXQ", invalid: true},
		{name: "too many", encoded: base64.RawStdEncoding.EncodeToString([]byte(`[` + strings.Repeat(`"unknown",`, 16) + `"unknown"]`)), invalid: true},
		{name: "16 names", encoded: base64.RawStdEncoding.EncodeToString([]byte(`[` + strings.Repeat(`"unknown",`, 15) + `"unknown"]`)), expected: []string{}},
		{name: "1024 bytes", encoded: base64.StdEncoding.EncodeToString([]byte(`["` + strings.Repeat("x", 764) + `"]`)), expected: []string{}},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			actual, err := decodeWindowsKubeletOmissionRequest(scenario.encoded)
			if scenario.invalid {
				require.Error(test, err)
				return
			}
			require.NoError(test, err)
			require.Equal(test, scenario.expected, actual)
		})
	}
}

func setWindowsKubeletTestOverrides(config *datamodel.NodeBootstrappingConfiguration, flags map[string]string) {
	config.ContainerService.Properties.CustomConfiguration = &datamodel.CustomConfiguration{
		WindowsKubernetesConfigurations: map[string]*datamodel.ComponentConfiguration{
			string(datamodel.Componentkubelet): {Config: flags},
		},
	}
}

func TestWindowsKubeletConfigurationRejectsUnsafeInputBeforeMutation(test *testing.T) {
	for _, scenario := range []struct {
		name   string
		modify func(*datamodel.NodeBootstrappingConfiguration)
	}{
		{name: "raw config", modify: func(config *datamodel.NodeBootstrappingConfiguration) {
			config.KubeletConfig["--config"] = "custom.json"
		}},
		{name: "custom config directory", modify: func(config *datamodel.NodeBootstrappingConfiguration) {
			setWindowsKubeletTestOverrides(config, map[string]string{"--config-dir": "custom"})
		}},
		{name: "missing rotation CLI", modify: func(config *datamodel.NodeBootstrappingConfiguration) {
			config.KubeletConfigFileConfig.ServerTLSBootstrap = true
		}},
		{name: "custom disables rotation", modify: func(config *datamodel.NodeBootstrappingConfiguration) {
			config.KubeletConfigFileConfig.ServerTLSBootstrap = true
			config.KubeletConfig["--rotate-server-certificates"] = "true"
			setWindowsKubeletTestOverrides(config, map[string]string{"--rotate-server-certificates": "false"})
		}},
		{name: "invalid omission", modify: func(config *datamodel.NodeBootstrappingConfiguration) {
			config.EnabledFeatures["KUBELET_FLAGS_TO_OMIT"] = "invalid"
		}},
		{name: "wrong kind", modify: func(config *datamodel.NodeBootstrappingConfiguration) { config.KubeletConfigFileConfig.Kind = "Other" }},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			config := newWindowsKubeletConfigTestInput(test)
			scenario.modify(config)
			before, err := json.Marshal(config)
			require.NoError(test, err)
			result, err := (&agentBakerImpl{}).GetNodeBootstrapping(context.Background(), config)
			require.Error(test, err)
			require.Nil(test, result)
			after, err := json.Marshal(config)
			require.NoError(test, err)
			require.Equal(test, before, after)
		})
	}
}

func TestWindowsKubeletConfigurationPreservesCustomCLI(test *testing.T) {
	config := newWindowsKubeletConfigTestInput(test)
	config.KubeletConfigFileConfig.ServerTLSBootstrap = true
	config.KubeletConfig["--anonymous-auth"] = "false"
	config.KubeletConfig["--hairpin-mode"] = "promiscuous-bridge"
	config.KubeletConfig["--register-with-taints"] = "example.com/init=:NoSchedule"
	setWindowsKubeletTestOverrides(config, map[string]string{"--rotate-server-certificates": "true", "--container-log-max-files": "7"})
	zeroFiles := int32(0)
	config.AgentPoolProfile.CustomKubeletConfig = &datamodel.CustomKubeletConfig{ContainerLogMaxFiles: &zeroFiles}
	_, err := windowsKubeletFlagsToOmit(config)
	require.NoError(test, err)
	payload := renderWindowsKubeletTestPayload(test, config)
	for _, flag := range []string{
		"--anonymous-auth=false", "--hairpin-mode=promiscuous-bridge", "--register-with-taints=example.com/init=:NoSchedule",
		"--container-log-max-files=0", "--rotate-server-certificates=true",
	} {
		require.Contains(test, payload, `"`+flag+`"`)
	}
	require.Contains(test, payload, `$global:EnableKubeletServingCertificateRotation=[System.Convert]::ToBoolean("true")`)
}

func TestWindowsKubeletConfigurationRejectsOversizedCustomData(test *testing.T) {
	config := newWindowsKubeletConfigTestInput(test)
	config.KubeletConfigFileConfig.StaticPodPath = strings.Repeat("x", 65536)
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
	config := newWindowsKubeletConfigTestInput(test)
	functions := getContainerServiceFuncMap(config)
	baseScripts, ok := functions["GetKubernetesWindowsAgentFunctions"].(func() string)
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
		filename := path[strings.LastIndex(path, "/")+1:]
		require.Equal(test, contents, entries["windows/kubeletconfiguration/"+filename])
	}
	payload := renderWindowsKubeletTestPayload(test, config)
	require.Contains(test, payload, encoded)
	config.KubeletConfigFileConfig = nil
	legacyPayload := renderWindowsKubeletTestPayload(test, config)
	require.Contains(test, legacyPayload, original)
	test.Logf("legacy customData=%d bytes; active=%d bytes; bootstrap ZIP base64 delta=%d bytes", len(legacyPayload), len(payload), len(encoded)-len(original))
}
