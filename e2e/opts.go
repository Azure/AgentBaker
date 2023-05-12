package e2e_test

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbakere2e/clients"
	"github.com/Azure/agentbakere2e/scenario"
)

type runOpts struct {
	clusterConfig clusterConfig
	cloud         *clients.AzureClient
	suiteConfig   *suiteConfig
	scenario      *scenario.Scenario
	nbc           *datamodel.NodeBootstrappingConfiguration
	loggingDir    string
}
