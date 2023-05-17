// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"context"
	"fmt"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

//nolint:revive // Name does not need to be modified to baker
type AgentBaker interface {
	GetNodeBootstrapping(ctx context.Context, config *datamodel.NodeBootstrappingConfiguration) (*datamodel.NodeBootstrapping, error)
	GetLatestSigImageConfig(sigConfig datamodel.SIGConfig, region string, distro datamodel.Distro) (*datamodel.SigImageConfig, error)
	GetDistroSigImageConfig(sigConfig datamodel.SIGConfig, region string) (map[datamodel.Distro]datamodel.SigImageConfig, error)
}

func NewAgentBaker() (AgentBaker, error) {
	return &agentBakerImpl{}, nil
}

type agentBakerImpl struct{}

func (agentBaker *agentBakerImpl) GetNodeBootstrapping(ctx context.Context,
	config *datamodel.NodeBootstrappingConfiguration) (*datamodel.NodeBootstrapping, error) {
	// validate and fix input before passing config to the template generator.
	if config.AgentPoolProfile.IsWindows() {
		validateAndSetWindowsNodeBootstrappingConfiguration(config)
	} else {
		validateAndSetLinuxNodeBootstrappingConfiguration(config)
	}

	templateGenerator := InitializeTemplateGenerator()
	nodeBootstrapping := &datamodel.NodeBootstrapping{
		CustomData: templateGenerator.getNodeBootstrappingPayload(config),
		CSE:        templateGenerator.getNodeBootstrappingCmd(config),
	}

	distro := config.AgentPoolProfile.Distro
	if distro == datamodel.CustomizedWindowsOSImage || distro == datamodel.CustomizedImage {
		return nodeBootstrapping, nil
	}

	osImageConfigMap, hasCloud := datamodel.AzureCloudToOSImageMap[config.CloudSpecConfig.CloudName]
	if !hasCloud {
		return nil, fmt.Errorf("don't have settings for cloud %s", config.CloudSpecConfig.CloudName)
	}

	if osImageConfig, hasImage := osImageConfigMap[distro]; hasImage {
		nodeBootstrapping.OSImageConfig = &osImageConfig
	}

	sigAzureEnvironmentSpecConfig, err := datamodel.GetSIGAzureCloudSpecConfig(config.SIGConfig, config.ContainerService.Location)
	if err != nil {
		return nil, err
	}

	nodeBootstrapping.SigImageConfig = findSIGImageConfig(sigAzureEnvironmentSpecConfig, distro)
	if nodeBootstrapping.SigImageConfig == nil && nodeBootstrapping.OSImageConfig == nil {
		return nil, fmt.Errorf("can't find image for distro %s", distro)
	}

	return nodeBootstrapping, nil
}

func findSIGImageConfig(sigConfig datamodel.SIGAzureEnvironmentSpecConfig, distro datamodel.Distro) *datamodel.SigImageConfig {
	if imageConfig, ok := sigConfig.SigUbuntuImageConfig[distro]; ok {
		return &imageConfig
	}
	if imageConfig, ok := sigConfig.SigCBLMarinerImageConfig[distro]; ok {
		return &imageConfig
	}
	if imageConfig, ok := sigConfig.SigWindowsImageConfig[distro]; ok {
		return &imageConfig
	}
	if imageConfig, ok := sigConfig.SigUbuntuEdgeZoneImageConfig[distro]; ok {
		return &imageConfig
	}

	return nil
}

func (agentBaker *agentBakerImpl) GetLatestSigImageConfig(
	sigConfig datamodel.SIGConfig, region string, distro datamodel.Distro) (*datamodel.SigImageConfig, error) {
	sigAzureEnvironmentSpecConfig, err := datamodel.GetSIGAzureCloudSpecConfig(sigConfig, region)
	if err != nil {
		return nil, err
	}

	sigImageConfig := findSIGImageConfig(sigAzureEnvironmentSpecConfig, distro)
	if sigImageConfig == nil {
		return nil, fmt.Errorf("can't find SIG image config for distro %s in region %s", distro, region)
	}
	return sigImageConfig, nil
}

func (agentBaker *agentBakerImpl) GetDistroSigImageConfig(
	sigConfig datamodel.SIGConfig, region string) (map[datamodel.Distro]datamodel.SigImageConfig, error) {
	allAzureSigConfig, err := datamodel.GetSIGAzureCloudSpecConfig(sigConfig, region)
	if err != nil {
		return nil, fmt.Errorf("failed to get sig image config: %w", err)
	}

	allDistros := map[datamodel.Distro]datamodel.SigImageConfig{}
	for distro, sigConfig := range allAzureSigConfig.SigWindowsImageConfig {
		allDistros[distro] = sigConfig
	}

	for distro, sigConfig := range allAzureSigConfig.SigCBLMarinerImageConfig {
		allDistros[distro] = sigConfig
	}

	for distro, sigConfig := range allAzureSigConfig.SigUbuntuImageConfig {
		allDistros[distro] = sigConfig
	}

	for distro, sigConfig := range allAzureSigConfig.SigUbuntuEdgeZoneImageConfig {
		allDistros[distro] = sigConfig
	}

	return allDistros, nil
}
