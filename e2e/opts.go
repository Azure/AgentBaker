package e2e

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

type scenarioRunOpts struct {
	clusterConfig      *Cluster
	scenario           *Scenario
	nbc                *datamodel.NodeBootstrappingConfiguration
	isSelfContainedVHD bool
}
