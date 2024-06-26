package e2e

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbakere2e/scenario"
)

type scenarioRunOpts struct {
	clusterConfig clusterConfig
	scenario      *scenario.Scenario
	nbc           *datamodel.NodeBootstrappingConfiguration
	loggingDir    string
}
