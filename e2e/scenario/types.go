package scenario

import (
	"context"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
)

type Table map[string]*Scenario

type Scenario struct {
	Name        string
	Description string
	Config
}

type Config struct {
	// BootstrapConfig          *datamodel.NodeBootstrappingConfiguration
	ClusterSelector        func(*armcontainerservice.ManagedCluster) bool
	ClusterMutator         func(*armcontainerservice.ManagedCluster)
	BootstrapConfigMutator func(*testing.T, *datamodel.NodeBootstrappingConfiguration)
	VMConfigMutator        func(*armcompute.VirtualMachineScaleSet)
	Validator              func(context.Context, *testing.T, *ValidationInput) error
}

type ValidationInput struct {
	PrivateIP     string
	SSHPrivateKey string
}
