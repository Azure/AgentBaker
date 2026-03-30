package e2e

import (
	"context"
	"testing"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
	"github.com/stretchr/testify/require"
)

// Test_LocalDNSHostsPlugin tests the localdns hosts plugin across all supported distros
// on the legacy (bash CSE) bootstrap path.
// Hosts plugin validators (AA flag, IP match, Corefile, hosts file) run automatically
// via ValidateCommonLinux when EnableHostsPlugin is set.
//
// Run a single distro with: go test -run "Test_LocalDNSHostsPlugin/AzureLinuxV3" -v
func Test_LocalDNSHostsPlugin(t *testing.T) {
	tests := []struct {
		name            string
		vhd             *config.Image
		vmConfigMutator func(*armcompute.VirtualMachineScaleSet)
	}{
		{name: "Ubuntu2204", vhd: config.VHDUbuntu2204Gen2Containerd},
		{name: "Ubuntu2404", vhd: config.VHDUbuntu2404Gen2Containerd},
		{name: "AzureLinuxV3", vhd: config.VHDAzureLinuxV3Gen2},
		{name: "ACL", vhd: config.VHDACLGen2TL, vmConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
			vmss.Properties = addTrustedLaunchToVMSS(vmss.Properties)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RunScenario(t, &Scenario{
				Description: "Tests that localdns hosts plugin works correctly on " + tt.name,
				Config: Config{
					Cluster: ClusterKubenet,
					VHD:     tt.vhd,
					BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
						nbc.AgentPoolProfile.LocalDNSProfile.EnableHostsPlugin = true
					},
					VMConfigMutator: tt.vmConfigMutator,
				},
			})
		})
	}
}

// Test_LocalDNSHostsPlugin_Scriptless tests the localdns hosts plugin across all supported distros
// on the scriptless (aks-node-controller) bootstrap path.
// The base AKSNodeConfig from nbcToAKSNodeConfigV1 already includes a full LocalDnsProfile with
// DNS overrides, so the mutator only needs to enable the hosts plugin.
//
// Run a single distro with: go test -run "Test_LocalDNSHostsPlugin_Scriptless/Ubuntu2204" -v
func Test_LocalDNSHostsPlugin_Scriptless(t *testing.T) {
	tests := []struct {
		name            string
		vhd             *config.Image
		vmConfigMutator func(*armcompute.VirtualMachineScaleSet)
	}{
		{name: "Ubuntu2204", vhd: config.VHDUbuntu2204Gen2Containerd},
		{name: "Ubuntu2404", vhd: config.VHDUbuntu2404Gen2Containerd},
		{name: "AzureLinuxV3", vhd: config.VHDAzureLinuxV3Gen2},
		{name: "ACL", vhd: config.VHDACLGen2TL, vmConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
			vmss.Properties = addTrustedLaunchToVMSS(vmss.Properties)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RunScenario(t, &Scenario{
				Description: "Tests that localdns hosts plugin works correctly on " + tt.name + " (scriptless)",
				Config: Config{
					Cluster:         ClusterKubenet,
					VHD:             tt.vhd,
					VMConfigMutator: tt.vmConfigMutator,
					AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
						config.LocalDnsProfile.EnableHostsPlugin = true
					},
				},
			})
		})
	}
}

// Test_LocalDNSHostsPlugin_Brownfield tests enabling the hosts plugin on an already-running VM
// using the legacy (bash CSE) bootstrap path.
//
// Phase 1: VM boots with EnableHostsPlugin=false — ValidateCommonLinux validates LocalDNS service
// is active and DNS resolves via 169.254.10.10, but skips hosts plugin validators.
//
// Phase 1b: validateNoHostsPlugin confirms SHOULD_ENABLE_HOSTS_PLUGIN=false, no cloud-env,
// no hosts directive in active corefile.
//
// Phase 2: enableHostsPluginOnRunningVM mutates the VM via SSH — patches environment file,
// creates cloud-env, starts aks-hosts-setup timer/service.
//
// Phase 3: Full hosts plugin validation — hosts file populated, service healthy, localdns restarted,
// AA flag proves authoritative response from hosts plugin.
//
// Run a single distro with: go test -run "Test_LocalDNSHostsPlugin_Brownfield/Ubuntu2204" -v
func Test_LocalDNSHostsPlugin_Brownfield(t *testing.T) {
	tests := []struct {
		name string
		vhd  *config.Image
	}{
		{name: "Ubuntu2204", vhd: config.VHDUbuntu2204Gen2Containerd},
		{name: "AzureLinuxV3", vhd: config.VHDAzureLinuxV3Gen2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RunScenario(t, &Scenario{
				Description: "Tests brownfield hosts plugin enable on " + tt.name,
				Config: Config{
					Cluster: ClusterKubenet,
					VHD:     tt.vhd,
					BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
						nbc.AgentPoolProfile.LocalDNSProfile.EnableHostsPlugin = false
					},
					Validator: func(ctx context.Context, s *Scenario) {
						// Phase 1b: Verify hosts plugin is NOT active (baseline)
						validateNoHostsPlugin(ctx, s)
						// Phase 2: Enable hosts plugin on running VM via SSH
						enableHostsPluginOnRunningVM(ctx, s, s.Runtime.NBC)
						// Phase 3: Validate hosts plugin works after brownfield enablement
						ValidateLocalDNSHostsFile(ctx, s, s.GetDefaultFQDNsForValidation())
						ValidateAKSHostsSetupService(ctx, s)
						execScriptOnVMForScenarioValidateExitCode(ctx, s,
							"sudo systemctl restart localdns", 0, "failed to restart localdns")
						ValidateLocalDNSHostsPluginBypass(ctx, s)
					},
				},
			})
		})
	}
}

// Test_LocalDNSHostsPlugin_Brownfield_Scriptless tests enabling the hosts plugin on an
// already-running VM using the scriptless (aks-node-controller) bootstrap path.
// Same three-phase flow as Test_LocalDNSHostsPlugin_Brownfield, but the scriptless path
// doesn't store NBC on ScenarioRuntime. Instead, the Validator calls getBaseNBC() to
// regenerate an NBC for experimental corefile generation.
//
// Run a single distro with: go test -run "Test_LocalDNSHostsPlugin_Brownfield_Scriptless/Ubuntu2204" -v
func Test_LocalDNSHostsPlugin_Brownfield_Scriptless(t *testing.T) {
	tests := []struct {
		name string
		vhd  *config.Image
	}{
		{name: "Ubuntu2204", vhd: config.VHDUbuntu2204Gen2Containerd},
		{name: "AzureLinuxV3", vhd: config.VHDAzureLinuxV3Gen2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RunScenario(t, &Scenario{
				Description: "Tests brownfield hosts plugin enable (scriptless) on " + tt.name,
				Config: Config{
					Cluster: ClusterKubenet,
					VHD:     tt.vhd,
					AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
						config.LocalDnsProfile.EnableHostsPlugin = false
					},
					Validator: func(ctx context.Context, s *Scenario) {
						// Phase 1b: Verify hosts plugin is NOT active (baseline)
						validateNoHostsPlugin(ctx, s)
						// Phase 2: Scriptless path doesn't store NBC, so regenerate it
						// for experimental corefile generation
						nbc, err := getBaseNBC(s.T, s.Runtime.Cluster, s.VHD)
						require.NoError(s.T, err)
						enableHostsPluginOnRunningVM(ctx, s, nbc)
						// Phase 3: Validate hosts plugin works after brownfield enablement
						ValidateLocalDNSHostsFile(ctx, s, s.GetDefaultFQDNsForValidation())
						ValidateAKSHostsSetupService(ctx, s)
						execScriptOnVMForScenarioValidateExitCode(ctx, s,
							"sudo systemctl restart localdns", 0, "failed to restart localdns")
						ValidateLocalDNSHostsPluginBypass(ctx, s)
					},
				},
			})
		})
	}
}

// Test_LocalDNSHostsPlugin_Rollback tests disabling the hosts plugin on an already-running VM
// using the legacy (bash CSE) bootstrap path. This simulates a production rollback where an
// operator needs to turn off the hosts plugin without reprovisioning nodes.
//
// Phase 1 (automatic via ValidateCommonLinux): VM boots with EnableHostsPlugin=true, so the
// full hosts-plugin validation suite runs automatically — hosts file populated, service healthy,
// localdns restarted, AA flag proves authoritative response. This confirms the hosts plugin
// is fully working before we disable it.
//
// Phase 2: disableHostsPluginOnRunningVM mutates the VM via SSH — patches environment file
// with SHOULD_ENABLE_HOSTS_PLUGIN=false, clears experimental corefile, removes cloud-env
// and hosts files, stops aks-hosts-setup timer, restarts localdns.
//
// Phase 3: validateHostsPluginDisabled runs comprehensive checks — environment file state,
// removed files, inactive timer, corefile without hosts directive, AA flag absent from dig,
// and DNS still resolves through localdns.
//
// Run a single distro with: go test -run "Test_LocalDNSHostsPlugin_Rollback/Ubuntu2204" -v
func Test_LocalDNSHostsPlugin_Rollback(t *testing.T) {
	tests := []struct {
		name string
		vhd  *config.Image
	}{
		{name: "Ubuntu2204", vhd: config.VHDUbuntu2204Gen2Containerd},
		{name: "AzureLinuxV3", vhd: config.VHDAzureLinuxV3Gen2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RunScenario(t, &Scenario{
				Description: "Tests hosts plugin rollback (disable on running VM) on " + tt.name,
				Config: Config{
					Cluster: ClusterKubenet,
					VHD:     tt.vhd,
					BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
						nbc.AgentPoolProfile.LocalDNSProfile.EnableHostsPlugin = true
					},
					Validator: func(ctx context.Context, s *Scenario) {
						// Phase 1 already ran via ValidateCommonLinux (IsHostsPluginEnabled=true)
						// Phase 2: Disable hosts plugin on running VM
						disableHostsPluginOnRunningVM(ctx, s)
						// Phase 3: Validate hosts plugin is fully disabled
						validateHostsPluginDisabled(ctx, s)
					},
				},
			})
		})
	}
}

// Test_LocalDNSHostsPlugin_Rollback_Scriptless tests disabling the hosts plugin on an
// already-running VM using the scriptless (aks-node-controller) bootstrap path.
// Same three-phase flow as Test_LocalDNSHostsPlugin_Rollback — the disable path is
// identical for both legacy and scriptless since it only patches files and stops services
// (no corefile generation needed).
//
// Run a single distro with: go test -run "Test_LocalDNSHostsPlugin_Rollback_Scriptless/Ubuntu2204" -v
func Test_LocalDNSHostsPlugin_Rollback_Scriptless(t *testing.T) {
	tests := []struct {
		name string
		vhd  *config.Image
	}{
		{name: "Ubuntu2204", vhd: config.VHDUbuntu2204Gen2Containerd},
		{name: "AzureLinuxV3", vhd: config.VHDAzureLinuxV3Gen2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RunScenario(t, &Scenario{
				Description: "Tests hosts plugin rollback (scriptless) on " + tt.name,
				Config: Config{
					Cluster:         ClusterKubenet,
					VHD:             tt.vhd,
					AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
						config.LocalDnsProfile.EnableHostsPlugin = true
					},
					Validator: func(ctx context.Context, s *Scenario) {
						// Phase 1 already ran via ValidateCommonLinux (IsHostsPluginEnabled=true)
						// Phase 2: Disable hosts plugin on running VM
						disableHostsPluginOnRunningVM(ctx, s)
						// Phase 3: Validate hosts plugin is fully disabled
						validateHostsPluginDisabled(ctx, s)
					},
				},
			})
		})
	}
}
