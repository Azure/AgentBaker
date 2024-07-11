package e2e

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbakere2e/cluster"
	"github.com/Azure/agentbakere2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/stretchr/testify/require"
)

type Tags struct {
	Name     string
	OS       string
	Platform string
	Airgap   bool
	GPU      bool
	WASM     bool
}

// MatchesFilters checks if the Tags struct matches all given filters.
// Filters are comma-separated "key=value" pairs (e.g., "gpu=true,os=x64").
// Returns true if all filters match, false otherwise. Errors on invalid input.
func (t Tags) MatchesFilters(filters string) (bool, error) {
	return t.matchFilters(filters, true)
}

// MatchesAnyFilter checks if the Tags struct matches at least one of the given filters.
// Filters are comma-separated "key=value" pairs (e.g., "gpu=true,os=x64").
// Returns true if any filter matches, false if none match. Errors on invalid input.
func (t Tags) MatchesAnyFilter(filters string) (bool, error) {
	return t.matchFilters(filters, false)
}

// matchFilters is a helper function used by both MatchesFilters and MatchesAnyFilter.
// The 'all' parameter determines whether all filters must match (true) or just any filter (false).
func (t Tags) matchFilters(filters string, all bool) (bool, error) {
	if filters == "" {
		return true, nil
	}

	v := reflect.ValueOf(t)
	filterPairs := strings.Split(filters, ",")

	for _, pair := range filterPairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			return false, fmt.Errorf("invalid filter format: %s", pair)
		}

		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])

		// Case-insensitive field lookup
		field := reflect.Value{}
		for i := 0; i < v.NumField(); i++ {
			if strings.EqualFold(v.Type().Field(i).Name, key) {
				field = v.Field(i)
				break
			}
		}

		if !field.IsValid() {
			return false, fmt.Errorf("unknown filter key: %s", key)
		}

		match := false
		switch field.Kind() {
		case reflect.String:
			match = strings.EqualFold(field.String(), value)
		case reflect.Bool:
			boolValue, err := strconv.ParseBool(value)
			if err != nil {
				return false, fmt.Errorf("invalid boolean value for %s: %s", key, value)
			}
			match = field.Bool() == boolValue
		default:
			return false, fmt.Errorf("unsupported field type for %s", key)
		}

		if all && !match {
			return false, nil
		}
		if !all && match {
			return true, nil
		}
	}

	return all, nil
}

// Scenario represents an AgentBaker E2E scenario
type Scenario struct {
	// Description is a short description of what the scenario does and tests for
	Description string

	// Tags are used for filtering scenarios to run based on the tags provided
	Tags Tags

	// Config contains the configuration of the scenario
	Config
}

// Config represents the configuration of an AgentBaker E2E scenario
type Config struct {
	Cluster func(ctx context.Context) (*cluster.Cluster, error)

	// VHD is the function called by the e2e suite on the given scenario to get its VHD selection
	VHDSelector func() (config.VHDResourceID, error)

	// BootstrapConfigMutator is a function which mutates the base NodeBootstrappingConfig according to the scenario's requirements
	BootstrapConfigMutator func(*datamodel.NodeBootstrappingConfiguration)

	// VMConfigMutator is a function which mutates the base VMSS model according to the scenario's requirements
	VMConfigMutator func(*armcompute.VirtualMachineScaleSet)

	// LiveVMValidators is a slice of LiveVMValidator objects for performing any live VM validation
	// specific to the scenario that isn't covered in the set of common validators run with all scenarios
	LiveVMValidators []*LiveVMValidator

	// Airgap is a boolean flag which indicates whether or not the scenario will have airgap network confiigurations
	Airgap bool
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
func (s *Scenario) PrepareVMSSModel(t *testing.T, vmss *armcompute.VirtualMachineScaleSet) error {
	resourceID, err := s.VHDSelector()
	require.NotNil(t, "test")
	require.NoError(t, err)
	require.NotEmpty(t, resourceID, "VHDSelector.ResourceID")
	require.NotNil(t, vmss, "input VirtualMachineScaleSet")
	require.NotNil(t, vmss.Properties, "input VirtualMachineScaleSet.Properties")

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
		ID: to.Ptr(string(resourceID)),
	}

	return nil
}
