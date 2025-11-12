package e2e

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"os/user"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/stretchr/testify/require"
)

type Tags struct {
	Name                string
	ImageName           string
	OS                  string
	Arch                string
	Airgap              bool
	NonAnonymousACR     bool
	GPU                 bool
	WASM                bool
	KubeletCustomConfig bool
	Scriptless          bool
	VHDCaching          bool
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

		var match bool
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

// Scenario represents an AgentBaker E2E scenario.
type Scenario struct {
	// Description is a short description of what the scenario does and tests for
	Description string

	// Tags are used for filtering scenarios to run based on the tags provided
	Tags Tags

	// Config contains the configuration of the scenario
	Config

	// Location is the Azure location where the scenario will run. This can be
	// used to override the default location.
	Location string

	// K8sSystemPoolSKU is the VM size to use for the system nodepool. If empty,
	// a default size will be used.
	K8sSystemPoolSKU string

	// Runtime contains the runtime state of the scenario. It's populated in the beginning of the test run
	Runtime *ScenarioRuntime
	T       testing.TB
}

type ScenarioRuntime struct {
	NBC           *datamodel.NodeBootstrappingConfiguration
	AKSNodeConfig *aksnodeconfigv1.Configuration
	Cluster       *Cluster
	VM            *ScenarioVM
	VMSSName      string
}

type ScenarioVM struct {
	KubeName  string
	VMSS      *armcompute.VirtualMachineScaleSet
	VM        *armcompute.VirtualMachineScaleSetVM
	PrivateIP string
}

// Config represents the configuration of an AgentBaker E2E scenario.
type Config struct {
	// Cluster creates, updates or re-uses an AKS cluster for the scenario
	Cluster func(ctx context.Context, request ClusterRequest) (*Cluster, error)

	// VHD is the node image used by the scenario.
	VHD *config.Image

	// BootstrapConfigMutator is a function which mutates the base NodeBootstrappingConfig according to the scenario's requirements
	BootstrapConfigMutator func(*datamodel.NodeBootstrappingConfiguration)

	// AKSNodeConfigMutator if defined then aks-node-controller will be used to provision nodes
	AKSNodeConfigMutator func(*aksnodeconfigv1.Configuration)

	// VMConfigMutator is a function which mutates the base VMSS model according to the scenario's requirements
	VMConfigMutator func(*armcompute.VirtualMachineScaleSet)

	// Validator is a function where the scenario can perform any extra validation checks
	Validator func(ctx context.Context, s *Scenario)

	// SkipDefaultValidation is a flag to indicate whether the common validation (like spawning a pod) should be skipped.
	// It shouldn't be used for majority of scenarios, currently only used for preparing VHD in a two-stage scenario
	SkipDefaultValidation bool

	// SkipSSHConnectivityValidation is a flag to indicate whether the ssh connectivity validation should be skipped.
	// It shouldn't be used for majority of scenarios, currently only used for scenarios where the node is not expected to be reachable via ssh
	SkipSSHConnectivityValidation bool

	// WaitForSSHAfterReboot if set to non-zero duration, SSH connectivity validation will retry with exponential backoff
	// for up to this duration when encountering reboot-related errors. This is useful for scenarios where the node
	// reboots during provisioning (e.g., MIG-enabled GPU nodes). Default (zero value) means no retry.
	WaitForSSHAfterReboot time.Duration

	// if VHDCaching is set then a VHD will be created first for the test scenario and then a VM will be created from that VHD.
	// The main purpose is to validate VHD Caching logic and ensure a reboot step between basePrep and nodePrep doesn't break anything.
	VHDCaching bool
}

func (s *Scenario) PrepareAKSNodeConfig() {

}

// PrepareVMSSModel mutates the input VirtualMachineScaleSet based on the scenario's VMConfigMutator, if configured.
// This method will also use the scenario's configured VHD selector to modify the input VMSS to reference the correct VHD resource.
func (s *Scenario) PrepareVMSSModel(ctx context.Context, t testing.TB, vmss *armcompute.VirtualMachineScaleSet) {
	resourceID, err := CachedPrepareVHD(ctx, GetVHDRequest{
		Image:    *s.VHD,
		Location: s.Location,
	})
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

	s.updateTags(ctx, vmss)
}

func (s *Scenario) updateTags(ctx context.Context, vmss *armcompute.VirtualMachineScaleSet) {
	if vmss.Tags == nil {
		vmss.Tags = map[string]*string{}
	}

	// don't clean up VMSS in other tests
	if config.Config.KeepVMSS {
		vmss.Tags["KEEP_VMSS"] = to.Ptr("true")
	}

	if config.Config.BuildID != "" {
		vmss.Tags[buildIDTagKey] = &config.Config.BuildID
	}

	owner, err := getLoggedInAzUser()
	if err != nil {
		owner, err = getLocalUsername()
		if err != nil {
			owner = "unknown"
		}
	}
	vmss.Tags["owner"] = to.Ptr(owner)
}

func getLoggedInAzUser() (string, error) {
	// Define the command and arguments
	cmd := exec.Command("az", "account", "show", "--query", "user.name", "-o", "tsv")

	// Create a buffer to capture stdout
	var out bytes.Buffer
	cmd.Stdout = &out

	// Run the command
	err := cmd.Run()
	if err != nil {
		return "", err
	}

	return out.String(), nil
}

func getLocalUsername() (string, error) {
	currentUser, err := user.Current()
	if err == nil {
		return currentUser.Username, nil
	}

	return "", err
}

func (s *Scenario) IsWindows() bool {
	return s.VHD.OS == config.OSWindows
}

func (s *Scenario) IsLinux() bool {
	return !s.IsWindows()
}
