// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"context"
	"fmt"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

type AgentBaker interface {
	GetNodeBootstrapping(ctx context.Context, config *datamodel.NodeBootstrappingConfiguration) (*datamodel.NodeBootstrapping, error)
	GetLatestSigImageConfig(sigConfig datamodel.SIGConfig, region string, distro datamodel.Distro) (*datamodel.SigImageConfig, error)
}

func NewAgentBaker() (AgentBaker, error) {
	return &agentBakerImpl{}, nil
}

type agentBakerImpl struct{}

func (agentBaker *agentBakerImpl) GetNodeBootstrapping(ctx context.Context,
	config *datamodel.NodeBootstrappingConfiguration) (*datamodel.NodeBootstrapping, error) {
	templateGenerator := InitializeTemplateGenerator()
	nodeBootstrapping := &datamodel.NodeBootstrapping{
		CustomData: templateGenerator.GetNodeBootstrappingPayload(config),
		CSE:        templateGenerator.GetNodeBootstrappingCmd(config),
	}

	osImageConfigMap, hasCloud := datamodel.AzureCloudToOSImageMap[config.CloudSpecConfig.CloudName]
	if !hasCloud {
		return nil, fmt.Errorf("don't have settings for cloud %s", config.CloudSpecConfig.CloudName)
	}

	distro := config.AgentPoolProfile.Distro
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
		return nil, fmt.Errorf("can't find SIG image for distro %s in region %s", distro, region)
	}
	return sigImageConfig, nil
}
