// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

const (
	volumePluginDirFlag          = "--volume-plugin-dir"
	containerRuntimeEndpointFlag = "--container-runtime-endpoint"
)

// RP selects the version/image cohort and supplies values in the existing map.
// Only translate requested values equivalent to the local script appenders;
// arbitrary CLI overrides must keep their existing precedence.
func kubeletConfigMigrationValues(config *datamodel.NodeBootstrappingConfiguration) map[string]string {
	if config == nil || config.ContainerService == nil || config.ContainerService.Properties == nil ||
		config.ContainerService.Properties.OrchestratorProfile == nil || config.AgentPoolProfile == nil ||
		!IsKubeletConfigFileEnabled(config.ContainerService, config.AgentPoolProfile, config.EnableKubeletConfigFile) {
		return nil
	}
	names := kubeletConfigMigrationNames(config.EnabledFeatures["KUBELET_FLAGS_TO_OMIT"])
	if len(names) == 0 {
		return nil
	}
	custom := getKubeletCustomConfiguration(config.ContainerService.Properties)
	if config.AgentPoolProfile.OSType == datamodel.Windows {
		custom = nil
		if windows := config.ContainerService.Properties.GetComponentWindowsKubernetesConfiguration(datamodel.Componentkubelet); windows != nil {
			custom = windows.Config
		}
	}
	for _, flags := range []map[string]string{config.KubeletConfig, custom} {
		_, hasConfig := flags["--config"]
		_, hasConfigDir := flags["--config-dir"]
		if hasConfig || hasConfigDir {
			return nil
		}
	}
	// These literals are equivalence checks, not a second source of RP policy.
	expected := map[string]string{
		"--enable-server":            "true",
		volumePluginDirFlag:          "/etc/kubernetes/volumeplugins",
		"--cgroup-driver":            "systemd",
		"--runtime-request-timeout":  "15m",
		containerRuntimeEndpointFlag: "unix:///run/containerd/containerd.sock",
	}
	switch config.AgentPoolProfile.OSType {
	case datamodel.Linux:
	case datamodel.Windows:
		expected = map[string]string{
			volumePluginDirFlag:          `c:\k\volumeplugins`,
			containerRuntimeEndpointFlag: "npipe://./pipe/containerd-containerd",
		}
	default:
		return nil
	}
	values := make(map[string]string)
	for _, name := range names {
		value, present := config.KubeletConfig[name]
		literal, recognized := expected[name]
		_, overridden := custom[name]
		if present && recognized && !overridden && value == literal {
			values[name] = value
		}
	}
	return values
}

func kubeletConfigMigrationNames(encoded string) []string {
	if len(encoded) == 0 || len(encoded) > 1024 || strings.ContainsAny(encoded, "\r\n") {
		return nil
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil {
		decoded, err = base64.RawStdEncoding.Strict().DecodeString(encoded)
	}
	var entries []any
	if err != nil || json.Unmarshal(decoded, &entries) != nil || len(entries) == 0 || len(entries) > 16 {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name, valid := entry.(string)
		if !valid {
			return nil
		}
		names = append(names, name)
	}
	return names
}

func applyKubeletConfigMigration(configuration *datamodel.AKSKubeletConfiguration, values map[string]string) {
	for name, value := range values {
		switch name {
		case "--enable-server":
			configuration.EnableServer = strToBoolPtr(value)
		case volumePluginDirFlag:
			configuration.VolumePluginDir = value
		case "--cgroup-driver":
			configuration.CgroupDriver = value
		case "--runtime-request-timeout":
			configuration.RuntimeRequestTimeout = datamodel.Duration(value)
		case containerRuntimeEndpointFlag:
			configuration.ContainerRuntimeEndpoint = value
		}
	}
}
