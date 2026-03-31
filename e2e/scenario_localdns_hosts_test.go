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
// using the legacy (bash CSE) bootstrap path. This simulates a production rollback where a
// customer disables the hosts plugin on an existing agentpool and AKS-RP re-runs CSE.
//
// Phase 1 (automatic via ValidateCommonLinux): VM boots with EnableHostsPlugin=true, so the
// full hosts-plugin validation suite runs automatically — hosts file populated, service healthy,
// localdns restarted, AA flag proves authoritative response. This confirms the hosts plugin
// is fully working before we disable it.
//
// Phase 2: RerunCSE regenerates the CSE with EnableHostsPlugin=false and pushes it to the
// existing VMSS. The CSE re-runs enableLocalDNS() which hits the new else branch —
// disableAKSHostsSetup() stops the timer and removes the hosts file.
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
						// Phase 2: Re-run CSE with EnableHostsPlugin=false (production disable path)
						nbcCopy := *s.Runtime.NBC
						appCopy := *nbcCopy.AgentPoolProfile
						nbcCopy.AgentPoolProfile = &appCopy
						localDNSCopy := *appCopy.LocalDNSProfile
						appCopy.LocalDNSProfile = &localDNSCopy
						localDNSCopy.EnableHostsPlugin = false
						RerunCSE(ctx, s, &nbcCopy)
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
// Same three-phase flow as Test_LocalDNSHostsPlugin_Rollback. RerunCSE uses the legacy
// CSE generation path (ab.GetNodeBootstrapping) since it embeds env vars directly in the
// CSE command string, avoiding the need to update the on-disk AKSNodeConfig JSON.
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
						// Phase 2: Re-run CSE with EnableHostsPlugin=false (production disable path)
						// Scriptless path doesn't store NBC, so regenerate one for CSE generation
						nbc, err := getBaseNBC(s.T, s.Runtime.Cluster, s.VHD)
						require.NoError(s.T, err)
						nbc.AgentPoolProfile.LocalDNSProfile.EnableHostsPlugin = false
						RerunCSE(ctx, s, nbc)
						// Phase 3: Validate hosts plugin is fully disabled
						validateHostsPluginDisabled(ctx, s)
					},
				},
			})
		})
	}
}

// Test_LocalDNSHostsPlugin_DisableNeverEnabled tests that the disable path is idempotent —
// calling disableAKSHostsSetup() on a node where the hosts plugin was never enabled
// does not cause errors. This is the common case: most nodes boot with
// EnableHostsPlugin=false and the else branch in enableLocalDNS() runs harmlessly.
//
// The test boots with EnableHostsPlugin=false, then validates:
// 1. LocalDNS is running and DNS resolves through 169.254.10.10
// 2. Hosts plugin is NOT active (no hosts file, no timer, base corefile)
// 3. All validators pass without errors
func Test_LocalDNSHostsPlugin_DisableNeverEnabled(t *testing.T) {
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
				Description: "Tests disable path is idempotent (never enabled) on " + tt.name,
				Config: Config{
					Cluster: ClusterKubenet,
					VHD:     tt.vhd,
					BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
						nbc.AgentPoolProfile.LocalDNSProfile.EnableHostsPlugin = false
					},
					Validator: func(ctx context.Context, s *Scenario) {
						// ValidateCommonLinux already ran — LocalDNS enabled, hosts plugin skipped
						// Explicitly verify hosts plugin is not active
						validateNoHostsPlugin(ctx, s)
					},
				},
			})
		})
	}
}

// Test_LocalDNSHostsPlugin_BackwardCompat_NewCSEOldServiceUnit tests backward compatibility
// for the cloud-env consolidation. After this change, new CSE no longer writes
// /etc/localdns/cloud-env. On old VHDs where aks-hosts-setup.service still reads from
// cloud-env, the timer fires but aks-hosts-setup.sh sees empty LOCALDNS_CRITICAL_FQDNS
// and exits gracefully. We verify this by enabling hosts plugin but clearing the FQDNs.
func Test_LocalDNSHostsPlugin_BackwardCompat_NewCSEOldServiceUnit(t *testing.T) {
	tests := []struct {
		name string
		vhd  *config.Image
	}{
		{name: "Ubuntu2204", vhd: config.VHDUbuntu2204Gen2Containerd},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RunScenario(t, &Scenario{
				Description: "Tests backward compat (new CSE, old service unit) on " + tt.name,
				Config: Config{
					Cluster: ClusterKubenet,
					VHD:     tt.vhd,
					BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
						// Enable hosts plugin but clear CriticalFQDNs to simulate
						// the old-VHD scenario where FQDNs aren't available
						nbc.AgentPoolProfile.LocalDNSProfile.EnableHostsPlugin = true
						nbc.AgentPoolProfile.LocalDNSProfile.CriticalFQDNs = nil
					},
					Validator: func(ctx context.Context, s *Scenario) {
						// With empty CriticalFQDNs, enableAKSHostsSetup() should skip
						// starting the timer (LOCALDNS_CRITICAL_FQDNS guard).
						// LocalDNS should still be running with the base corefile.
						ValidateLocalDNSService(ctx, s, "enabled")
						ValidateLocalDNSResolution(ctx, s, "169.254.10.10")

						// Verify hosts plugin did NOT activate (no FQDNs = graceful skip)
						script := `set -euo pipefail
errors=0

# hosts file should not have IP mappings (no FQDNs were provided)
hosts_file="/etc/localdns/hosts"
if [ -f "$hosts_file" ] && grep -qE '^[0-9a-fA-F.:]+[[:space:]]+[a-zA-Z]' "$hosts_file"; then
    echo "ERROR: hosts file has IP mappings but no CriticalFQDNs were provided"
    errors=$((errors + 1))
else
    echo "OK: hosts file has no IP mappings (expected — no CriticalFQDNs)"
fi

# Active corefile should NOT include hosts plugin
corefile="/opt/azure/containers/localdns/localdns.corefile"
if grep -q "hosts /etc/localdns/hosts" "$corefile"; then
    echo "ERROR: active corefile contains hosts plugin directive"
    errors=$((errors + 1))
else
    echo "OK: active corefile does not contain hosts plugin"
fi

exit $errors`
						execScriptOnVMForScenarioValidateExitCode(ctx, s, script, 0,
							"backward compat validation failed")
					},
				},
			})
		})
	}
}
