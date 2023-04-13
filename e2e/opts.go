package e2e_test

import (
	"fmt"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbakere2e/scenario"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
)

var (
	errNilClusterModelProperties = fmt.Errorf("cluster model or cluster model properties were nil")
)

type scenarioRunOpts struct {
	cloud         *azureClient
	kube          *kubeclient
	suiteConfig   *suiteConfig
	scenario      *scenario.Scenario
	chosenCluster *armcontainerservice.ManagedCluster
	nbc           *datamodel.NodeBootstrappingConfiguration
	subnetID      string
	loggingDir    string
}

// Returns true if the scenario's chosen cluster is configured with Azure CNI
func (opts *scenarioRunOpts) isChosenClusterAzureCNI() (bool, error) {
	cluster := opts.chosenCluster
	if cluster != nil && cluster.Properties != nil && cluster.Properties.NetworkProfile != nil {
		return *cluster.Properties.NetworkProfile.NetworkPlugin == armcontainerservice.NetworkPluginAzure, nil
	}
	return false, errNilClusterModelProperties
}

// Returns the maximum number of pods per node of the chosen cluster's agentpool
func (opts *scenarioRunOpts) chosenClusterMaxPodsPerNode() (int, error) {
	cluster := opts.chosenCluster
	if cluster != nil && cluster.Properties != nil && cluster.Properties.AgentPoolProfiles != nil && len(cluster.Properties.AgentPoolProfiles) > 0 {
		return int(*cluster.Properties.AgentPoolProfiles[0].MaxPods), nil
	}
	return 0, errNilClusterModelProperties
}
