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

	if osImageConfig, hasImage := osImageConfigMap[config.AgentPoolProfile.Distro]; hasImage {
		nodeBootstrapping.OSImageConfig = osImageConfig
	}

	return nodeBootstrapping, nil
}
