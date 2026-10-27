// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/stretchr/testify/require"
)

func migrationRequest(t *testing.T, names ...string) *datamodel.NodeBootstrappingConfiguration {
	t.Helper()
	data, err := json.Marshal(names)
	require.NoError(t, err)
	return &datamodel.NodeBootstrappingConfiguration{
		ContainerService: &datamodel.ContainerService{Properties: &datamodel.Properties{
			OrchestratorProfile: &datamodel.OrchestratorProfile{OrchestratorType: datamodel.Kubernetes, OrchestratorVersion: "1.38.0"},
		}},
		AgentPoolProfile: &datamodel.AgentPoolProfile{OSType: datamodel.Linux, CustomKubeletConfig: &datamodel.CustomKubeletConfig{
			CPUCfsQuota: to.BoolPtr(false), ContainerLogMaxSizeMB: to.Int32Ptr(100),
		}},
		EnableKubeletConfigFile: true,
		EnabledFeatures:         map[string]string{"KUBELET_FLAGS_TO_OMIT": base64.RawStdEncoding.EncodeToString(data)},
		KubeletConfig: map[string]string{
			"--enable-server": "true", "--volume-plugin-dir": "/etc/kubernetes/volumeplugins",
			"--cgroup-driver": "systemd", "--runtime-request-timeout": "15m",
			"--container-runtime-endpoint": "unix:///run/containerd/containerd.sock",
			"--event-qps":                  "0", "--max-pods": "110", "--tls-cert-file": "/certs/kubelet.crt",
		},
	}
}

func TestKubeletConfigMigrationRendering(t *testing.T) {
	config := migrationRequest(t, "--enable-server", "--volume-plugin-dir", "--cgroup-driver", "--runtime-request-timeout", "--container-runtime-endpoint")
	// An unrelated component override exercises the mutating legacy custom-map merger.
	custom := map[string]string{"--v": "4"}
	config.ContainerService.Properties.CustomConfiguration = &datamodel.CustomConfiguration{
		KubernetesConfigurations: map[string]*datamodel.ComponentConfiguration{"kubelet": {Config: custom}},
	}
	before, err := json.Marshal(config)
	require.NoError(t, err)
	var expected map[string]any
	require.NoError(t, json.Unmarshal([]byte(GetKubeletConfigFileContent(config.KubeletConfig, config.AgentPoolProfile.CustomKubeletConfig)), &expected))
	expected["enableServer"] = true
	expected["volumePluginDir"] = "/etc/kubernetes/volumeplugins"
	expected["cgroupDriver"] = "systemd"
	expected["runtimeRequestTimeout"] = "15m"
	expected["containerRuntimeEndpoint"] = "unix:///run/containerd/containerd.sock"
	expectedJSON, err := json.Marshal(expected)
	require.NoError(t, err)
	getters := getContainerServiceFuncMap(config)
	plainGetter, ok := getters["GetKubeletConfigFileContent"].(func() string)
	require.True(t, ok)
	base64Getter, ok := getters["GetKubeletConfigFileContentBase64"].(func() string)
	require.True(t, ok)
	for range 2 {
		argv := GetOrderedKubeletConfigFlagString(config)
		require.Contains(t, argv, "--v=4")
		for name := range kubeletConfigMigrationValues(config) {
			require.NotContains(t, argv, name+"=")
		}
		plain := plainGetter()
		require.JSONEq(t, string(expectedJSON), plain)
		require.Equal(t, base64.StdEncoding.EncodeToString([]byte(plain)), base64Getter())
	}
	after, err := json.Marshal(config)
	require.NoError(t, err)
	require.Equal(t, before, after, "rendering must not turn generated entries into custom overrides")
}

func TestKubeletConfigMigrationFallback(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		change func(*datamodel.NodeBootstrappingConfiguration)
	}{
		{"absent request", func(c *datamodel.NodeBootstrappingConfiguration) { c.EnabledFeatures = nil }},
		{"config off", func(c *datamodel.NodeBootstrappingConfiguration) {
			c.EnableKubeletConfigFile = false
			c.AgentPoolProfile.CustomKubeletConfig = nil
		}},
		{"invalid base64", func(c *datamodel.NodeBootstrappingConfiguration) { c.EnabledFeatures["KUBELET_FLAGS_TO_OMIT"] = "!" }},
		{"oversized", func(c *datamodel.NodeBootstrappingConfiguration) {
			c.EnabledFeatures["KUBELET_FLAGS_TO_OMIT"] = strings.Repeat("a", 1025)
		}},
		{"wrong json shape", func(c *datamodel.NodeBootstrappingConfiguration) {
			c.EnabledFeatures["KUBELET_FLAGS_TO_OMIT"] = base64.RawStdEncoding.EncodeToString([]byte(`{"flag":true}`))
		}},
		{"null entry", func(c *datamodel.NodeBootstrappingConfiguration) {
			c.EnabledFeatures["KUBELET_FLAGS_TO_OMIT"] = base64.RawStdEncoding.EncodeToString([]byte(`["--enable-server",null]`))
		}},
		{"newline", func(c *datamodel.NodeBootstrappingConfiguration) {
			c.EnabledFeatures["KUBELET_FLAGS_TO_OMIT"] += "\n"
		}},
		{"empty list", func(c *datamodel.NodeBootstrappingConfiguration) { c.EnabledFeatures["KUBELET_FLAGS_TO_OMIT"] = "W10" }},
		{"too many names", func(c *datamodel.NodeBootstrappingConfiguration) {
			c.EnabledFeatures["KUBELET_FLAGS_TO_OMIT"] = base64.RawStdEncoding.EncodeToString([]byte(`["--enable-server"` + strings.Repeat(`,"--enable-server"`, 16) + `]`))
		}},
		{"raw config path", func(c *datamodel.NodeBootstrappingConfiguration) { c.KubeletConfig["--config"] = "" }},
		{"raw config dir", func(c *datamodel.NodeBootstrappingConfiguration) { c.KubeletConfig["--config-dir"] = "/custom" }},
		{"custom config path", func(c *datamodel.NodeBootstrappingConfiguration) {
			c.ContainerService.Properties.CustomConfiguration = &datamodel.CustomConfiguration{
				KubernetesConfigurations: map[string]*datamodel.ComponentConfiguration{"kubelet": {Config: map[string]string{"--config": "/custom"}}},
			}
		}},
		{"false override", func(c *datamodel.NodeBootstrappingConfiguration) { c.KubeletConfig["--enable-server"] = "false" }},
		{"missing raw value", func(c *datamodel.NodeBootstrappingConfiguration) { delete(c.KubeletConfig, "--enable-server") }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			config := migrationRequest(t, "--enable-server")
			scenario.change(config)
			require.Empty(t, kubeletConfigMigrationValues(config))
			require.Equal(t, GetKubeletConfigFileContent(config.KubeletConfig, config.AgentPoolProfile.CustomKubeletConfig), getKubeletConfigFileContentForNode(config))
			argv := GetOrderedKubeletConfigFlagString(config)
			if value, present := config.KubeletConfig["--enable-server"]; present {
				require.Contains(t, argv, "--enable-server="+value)
			}
		})
	}
}

func TestKubeletConfigMigrationSelection(t *testing.T) {
	config := migrationRequest(t, "--enable-server", "--runtime-request-timeout", "--cgroup-driver", "--unknown")
	config.KubeletConfig["--runtime-request-timeout"] = "0s"
	config.ContainerService.Properties.CustomConfiguration = &datamodel.CustomConfiguration{
		KubernetesConfigurations: map[string]*datamodel.ComponentConfiguration{"kubelet": {Config: map[string]string{"--cgroup-driver": "systemd"}}},
	}
	require.Equal(t, map[string]string{"--enable-server": "true"}, kubeletConfigMigrationValues(config))
	argv := GetOrderedKubeletConfigFlagString(config)
	require.NotContains(t, argv, "--enable-server=")
	require.Contains(t, argv, "--runtime-request-timeout=0s")
	require.Contains(t, argv, "--cgroup-driver=systemd")
	require.Contains(t, argv, "--volume-plugin-dir=/etc/kubernetes/volumeplugins", "unrequested values stay on CLI")
	content := getKubeletConfigFileContentForNode(config)
	require.Contains(t, content, `"enableServer": true`)
	require.NotContains(t, content, `"runtimeRequestTimeout"`)
	require.NotContains(t, content, `"cgroupDriver"`)
}

func TestKubeletConfigMigrationOrdinaryCLI(t *testing.T) {
	config := migrationRequest(t, "--enable-server", "--runtime-request-timeout")
	config.EnableKubeletConfigFile = false // A typed custom config still enables file mode.
	argv := GetOrderedKubeletConfigFlagString(config)
	require.NotContains(t, argv, "--enable-server=")
	require.NotContains(t, argv, "--runtime-request-timeout=")
	require.Contains(t, argv, "--volume-plugin-dir=")
	require.Contains(t, getKubeletConfigFileContentForNode(config), `"runtimeRequestTimeout": "15m"`)
}

func TestKubeletConfigMigrationWindows(t *testing.T) {
	config := migrationRequest(t, "--enable-server", "--volume-plugin-dir", "--container-runtime-endpoint")
	config.AgentPoolProfile.OSType = datamodel.Windows
	config.KubeletConfig["--volume-plugin-dir"] = `c:\k\volumeplugins`
	config.KubeletConfig["--container-runtime-endpoint"] = "npipe://./pipe/containerd-containerd"
	require.Equal(t, map[string]string{
		"--volume-plugin-dir": `c:\k\volumeplugins`, "--container-runtime-endpoint": "npipe://./pipe/containerd-containerd",
	}, kubeletConfigMigrationValues(config))
	// Both encodings are accepted, including the padding used by existing clients.
	data, err := base64.RawStdEncoding.DecodeString(config.EnabledFeatures["KUBELET_FLAGS_TO_OMIT"])
	require.NoError(t, err)
	config.EnabledFeatures["KUBELET_FLAGS_TO_OMIT"] = base64.StdEncoding.EncodeToString(data)
	require.Len(t, kubeletConfigMigrationValues(config), 2)
	config.ContainerService.Properties.CustomConfiguration = &datamodel.CustomConfiguration{
		// A Linux override must not control Windows migration.
		KubernetesConfigurations:        map[string]*datamodel.ComponentConfiguration{"kubelet": {Config: map[string]string{"--config": "/linux-only"}}},
		WindowsKubernetesConfigurations: map[string]*datamodel.ComponentConfiguration{"kubelet": {Config: map[string]string{"--volume-plugin-dir": `c:\k\volumeplugins`}}},
	}
	require.Equal(t, map[string]string{"--container-runtime-endpoint": "npipe://./pipe/containerd-containerd"}, kubeletConfigMigrationValues(config))
	config.ContainerService.Properties.CustomConfiguration.WindowsKubernetesConfigurations["kubelet"].Config["--config-dir"] = ""
	require.Empty(t, kubeletConfigMigrationValues(config))
}
