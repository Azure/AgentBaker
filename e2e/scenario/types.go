package scenario

import (
	"context"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
)

type ScenarioTable map[string]*Scenario

type Scenario struct {
	Name                string
	Description         string
	ClusterConfigurator ClusterConfigurator
	ScenarioConfig
}

type ScenarioConfig struct {
	// BootstrapConfig          *datamodel.NodeBootstrappingConfiguration
	BootstrapConfigMutator func(*testing.T, *datamodel.NodeBootstrappingConfiguration)
	VMConfigMutator        func(*armcompute.VirtualMachineScaleSet)
	Validator              func(context.Context, *testing.T, *ScenarioValidationInput) error
}

type ScenarioValidationInput struct {
	PrivateIP     string
	SSHPrivateKey string
}

type ClusterConfigurator func(*armcontainerservice.ManagedCluster, bool) bool
