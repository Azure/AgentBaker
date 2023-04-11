package e2e_test

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbakere2e/scenario"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
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
