// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"fmt"
	"strings"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

func containerdConfigSchema(version string) (int, error) {
	switch {
	case isVersionGreaterThanOrEqualTo(version, "2.3.0"):
		return 4, nil
	case isVersionGreaterThanOrEqualTo(version, "2.0.0"):
		return 3, nil
	case isVersionGreaterThanOrEqualTo(version, "1.0.0"):
		return 2, nil
	default:
		return 0, fmt.Errorf("unsupported or missing containerd version %q", version)
	}
}

func containerdVersionForConfig(
	config *datamodel.NodeBootstrappingConfiguration,
	profile *datamodel.AgentPoolProfile,
) (string, error) {
	version := strings.TrimSpace(config.ContainerdVersion)
	if version == "" && profile != nil && profile.KubernetesConfig != nil {
		version = strings.TrimSpace(profile.KubernetesConfig.ContainerdVersion)
	}
	if version != "" {
		if _, err := containerdConfigSchema(version); err != nil {
			return "", err
		}
		return version, nil
	}
	if profile != nil && profile.IsContainerdV2Distro() {
		return "2.0.0", nil
	}
	return "1.0.0", nil
}

func propagateConfiguredContainerdVersion(
	config *datamodel.NodeBootstrappingConfiguration,
	profile *datamodel.AgentPoolProfile,
) {
	if strings.TrimSpace(config.ContainerdVersion) != "" || profile == nil || profile.KubernetesConfig == nil {
		return
	}
	config.ContainerdVersion = strings.TrimSpace(profile.KubernetesConfig.ContainerdVersion)
}

func containerdConfigSchemaForConfig(
	config *datamodel.NodeBootstrappingConfiguration,
	profile *datamodel.AgentPoolProfile,
) (int, error) {
	version, err := containerdVersionForConfig(config, profile)
	if err != nil {
		return 0, err
	}
	return containerdConfigSchema(version)
}

func selectContainerdConfigTemplate(
	config *datamodel.NodeBootstrappingConfiguration,
	profile *datamodel.AgentPoolProfile,
	noGPU bool,
) (ContainerdConfigTemplate, error) {
	schema, err := containerdConfigSchemaForConfig(config, profile)
	if err != nil {
		return "", err
	}
	if schema >= 3 {
		if noGPU {
			return containerdV2NoGPUConfigTemplate, nil
		}
		return containerdV2ConfigTemplate, nil
	}
	if noGPU {
		return containerdV1NoGPUConfigTemplate, nil
	}
	return containerdV1ConfigTemplate, nil
}
