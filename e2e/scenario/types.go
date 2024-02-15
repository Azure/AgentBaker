package scenario

import (
	"fmt"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
)

// Template represents a 'scenario template' which contains common config used
// across all scenarios, such as the VHD catalog for selecting VHDs.
type Template struct {
	VHDCatalog
}

// NewTemplate constructs a new template using the base VHD catalog.
func NewTemplate() *Template {
	return &Template{
		VHDCatalog: BaseVHDCatalog,
	}
}

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

	// VHDSelector is the function called by the e2e suite on the given scenario to get its VHD selection
	VHDSelector func() VHD

	// BootstrapConfigMutator is a function which mutates the base NodeBootstrappingConfig according to the scenario's requirements
	BootstrapConfigMutator func(*datamodel.NodeBootstrappingConfiguration)

	// VMConfigMutator is a function which mutates the base VMSS model according to the scenario's requirements
	VMConfigMutator func(*armcompute.VirtualMachineScaleSet)

	// LiveVMValidators is a slice of LiveVMValidator objects for performing any live VM validation
	// specific to the scenario that isn't covered in the set of common validators run with all scenarios
	LiveVMValidators []*LiveVMValidator
}

// VMCommandOutputAsserterFn is a function which takes in stdout and stderr stream content
// as strings and performs arbitrary assertions on them, returning an error in the case where the assertion fails
type VMCommandOutputAsserterFn func(code, stdout, stderr string) error

// LiveVMValidator represents a command to be run on a live VM after
// node bootstrapping has succeeded that generates output which can be asserted against
// to make sure that the live VM itself is in the correct state
type LiveVMValidator struct {
	// Description is the description of the validator and what it actually validates on the VM
	Description string

	// Command is the command string to be run on the live VM after node bootstrapping has succeeed
	Command string

	// Asserter is the validator's VMCommandOutputAsserterFn which will be run against command output
	Asserter VMCommandOutputAsserterFn

	// IsShellBuiltIn is a boolean flag which indicates whether or not the command is a shell built-in
	// that will fail when executed with sudo - requires separate command to avoid command not found error on node
	IsShellBuiltIn bool
}

// PrepareNodeBootstrappingConfiguration mutates the input NodeBootstrappingConfiguration by calling the
// scenario's BootstrapConfigMutator on it, if configured.
func (s *Scenario) PrepareNodeBootstrappingConfiguration(nbc *datamodel.NodeBootstrappingConfiguration) {
	if s.BootstrapConfigMutator != nil {
		s.BootstrapConfigMutator(nbc)
	}
}

// PrepareVMSSModel mutates the input VirtualMachineScaleSet based on the scenario's VMConfigMutator, if configured.
// This method will also use the scenario's configured VHD selector to modify the input VMSS to reference the correct VHD resource.
func (s *Scenario) PrepareVMSSModel(vmss *armcompute.VirtualMachineScaleSet) error {
	if s.VHDSelector == nil {
		return fmt.Errorf("VHD selector configured for scenario %q is nil", s.Name)
	}
	if vmss == nil || vmss.Properties == nil {
		return fmt.Errorf("unable to prepare VMSS model for scenario %q: input VirtualMachineScaleSet or properties are nil", s.Name)
	}

	if s.VMConfigMutator != nil {
		s.VMConfigMutator(vmss)
	}

	if vmss.Properties.VirtualMachineProfile == nil {
		vmss.Properties.VirtualMachineProfile = &armcompute.VirtualMachineScaleSetVMProfile{}
	}
	if vmss.Properties.VirtualMachineProfile.StorageProfile == nil {
		vmss.Properties.VirtualMachineProfile.StorageProfile = &armcompute.VirtualMachineScaleSetStorageProfile{}
	}
	vmss.Properties.VirtualMachineProfile.StorageProfile.ImageReference = &armcompute.ImageReference{
		ID: to.Ptr(string(s.VHDSelector().ResourceID)),
	}

	return nil
}
