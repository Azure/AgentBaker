// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/require"
)

func TestContainerdConfigSchema(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    int
		wantErr string
	}{
		{name: "containerd 1", version: "1.7.36", want: 2},
		{name: "containerd 2.2", version: "2.2.4", want: 3},
		{name: "containerd 2.3", version: "2.3.4", want: 4},
		{name: "missing version", wantErr: `unsupported or missing containerd version ""`},
		{name: "invalid version", version: "latest", wantErr: `unsupported or missing containerd version "latest"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := containerdConfigSchema(test.version)
			if test.wantErr != "" {
				require.EqualError(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}
}

func TestScriptfulContainerdConfigUsesVersionedSchemaAndPluginPaths(t *testing.T) {
	tests := []struct {
		name               string
		version            string
		distro             datamodel.Distro
		wantSchema         string
		wantPluginPath     string
		unwantedPluginPath string
	}{
		{
			name:               "containerd 1",
			version:            "1.7.36",
			distro:             datamodel.AKSUbuntuContainerd2204,
			wantSchema:         "version = 2\n",
			wantPluginPath:     `io.containerd.grpc.v1.cri`,
			unwantedPluginPath: `io.containerd.cri.v1.runtime`,
		},
		{
			name:               "containerd 2.2",
			version:            "2.2.4",
			distro:             datamodel.AKSAzureLinuxV3Gen2Kata,
			wantSchema:         "version = 3\n",
			wantPluginPath:     `io.containerd.cri.v1.runtime`,
			unwantedPluginPath: `io.containerd.grpc.v1.cri`,
		},
		{
			name:               "containerd 2.3",
			version:            "2.3.4",
			distro:             datamodel.AKSAzureLinuxV3Gen2Kata,
			wantSchema:         "version = 4\n",
			wantPluginPath:     `io.containerd.cri.v1.runtime`,
			unwantedPluginPath: `io.containerd.grpc.v1.cri`,
		},
	}

	for _, test := range tests {
		for _, noGPU := range []bool{false, true} {
			name := "default"
			if noGPU {
				name = "no GPU"
			}
			t.Run(test.name+"/"+name, func(t *testing.T) {
				config := newNodeCustomDataRenderConfig(test.distro)
				config.ContainerdVersion = test.version
				config.AgentPoolProfile.KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime:       "containerd",
					ContainerRuntimeConfig: map[string]string{},
				}

				tmpl, err := selectContainerdConfigTemplate(config, config.AgentPoolProfile, noGPU)
				require.NoError(t, err)
				encoded, err := containerdConfigFromTemplate(config, config.AgentPoolProfile, tmpl)
				require.NoError(t, err)
				rendered, err := base64.StdEncoding.DecodeString(encoded)
				require.NoError(t, err)
				configText := string(rendered)

				require.True(t, strings.HasPrefix(configText, test.wantSchema), configText)
				require.Contains(t, configText, test.wantPluginPath)
				require.NotContains(t, configText, test.unwantedPluginPath)
				if test.distro.IsKataDistro() {
					require.Contains(t, configText, `plugins."io.containerd.cri.v1.runtime".containerd.runtimes.kata]`)
					require.Contains(t, configText, `plugins."io.containerd.cri.v1.runtime".containerd.runtimes.kata-v2]`)
				}
			})
		}
	}
}

func TestScriptfulContainerdConfigUsesProfileVersionFallback(t *testing.T) {
	config := newNodeCustomDataRenderConfig(datamodel.AKSAzureLinuxV3Gen2Kata)
	config.AgentPoolProfile.KubernetesConfig = &datamodel.KubernetesConfig{
		ContainerRuntime:       "containerd",
		ContainerdVersion:      "2.3.4",
		ContainerRuntimeConfig: map[string]string{},
	}

	schema, err := containerdConfigSchemaForConfig(config, config.AgentPoolProfile)
	require.NoError(t, err)
	require.Equal(t, 4, schema)
}

func TestKataContainerdConfigRequiresVersion(t *testing.T) {
	config := newNodeCustomDataRenderConfig(datamodel.AKSAzureLinuxV3Gen2Kata)
	config.AgentPoolProfile.KubernetesConfig = &datamodel.KubernetesConfig{
		ContainerRuntime:       "containerd",
		ContainerRuntimeConfig: map[string]string{},
	}

	err := ValidateAndSetLinuxNodeBootstrappingConfigurationWithError(config)
	require.EqualError(t, err, `validate containerd config: containerd version is required for Kata distro "aks-azurelinux-v3-gen2-kata"`)
}

func TestContainerdConfigRejectsUnsupportedVersion(t *testing.T) {
	config := newNodeCustomDataRenderConfig(datamodel.AKSUbuntuContainerd2204)
	config.ContainerdVersion = "latest"

	err := ValidateAndSetLinuxNodeBootstrappingConfigurationWithError(config)
	require.EqualError(t, err, `validate containerd config: unsupported or missing containerd version "latest"`)
}
