package e2e_test

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbakere2e/scenario"
)

type scenarioRunOpts struct {
	clusterConfig clusterConfig
	cloud         *azureClient
	suiteConfig   *suiteConfig
	scenario      *scenario.Scenario
	nbc           *datamodel.NodeBootstrappingConfiguration
	loggingDir    string
}
