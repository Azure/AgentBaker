package scenario

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbakere2e/validation"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
)

// Table represents a set of mappings from scenario name -> Scenario to
// be run as a part of the test suite
type Table map[string]*Scenario

// Scenario represents an AgentBaker E2E scenario
type Scenario struct {
	// Name is the name of the scenario
	Name string

	// Description is a short description of what the scenario does and tests for
	Description string

	// Config contains the configuration of the scenario
	Config
}

// Config represents the configuration of an AgentBaker E2E scenario
type Config struct {
	// ClusterSelector is a function which determines whether or not (by returning true/false) the
	// supplied cluster model represents a cluster which is capable of running the scenario
	ClusterSelector func(*armcontainerservice.ManagedCluster) bool

	// ClusterMutator is a function which mutates a supplied cluster model such that it represents a
	// cluster which is capable of running the scenario
	ClusterMutator func(*armcontainerservice.ManagedCluster)

	// BootstrapConfigMutator is a function which mutates the base NodeBootstrappingConfig according to the scenario's requirements
	BootstrapConfigMutator func(*datamodel.NodeBootstrappingConfiguration)

	// VMConfigMutator is a function which mutates the base VMSS model according to the scenario's requirements
	VMConfigMutator func(*armcompute.VirtualMachineScaleSet)

	// LiveVMValidators is a slice of LiveVMValidator objects for performing any live VM validation
	// specific to the scenario that isn't covered in the set of common validators run with all scenarios
	LiveVMValidators []*validation.LiveVMValidator

	// K8sValidators is a slice of K8sValidator objects for performing any k8s-level validation specific to the given scenario
	K8sValidators []*validation.K8sValidator
}
