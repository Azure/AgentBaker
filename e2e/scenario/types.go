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
	Name        string
	Description string
	ScenarioConfig
}

type ScenarioConfig struct {
	// BootstrapConfig          *datamodel.NodeBootstrappingConfiguration
	ClusterSelector        func(*armcontainerservice.ManagedCluster) bool
	ClusterMutator         func(*armcontainerservice.ManagedCluster)
	BootstrapConfigMutator func(*testing.T, *datamodel.NodeBootstrappingConfiguration)
	VMConfigMutator        func(*armcompute.VirtualMachineScaleSet)
	Validator              func(context.Context, *testing.T, *ScenarioValidationInput) error
}

type ScenarioValidationInput struct {
	PrivateIP     string
	SSHPrivateKey string
}
