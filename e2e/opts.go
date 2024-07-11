package e2e

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbakere2e/cluster"
)

type scenarioRunOpts struct {
	clusterConfig *cluster.Cluster
	scenario      *Scenario
	nbc           *datamodel.NodeBootstrappingConfiguration
	loggingDir    string
}
