package e2e

import (
	"context"
	"testing"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
)

// Test_Ubuntu2204_LocalDNSHostsPlugin tests the localdns hosts plugin feature on Ubuntu 22.04
func Test_Ubuntu2204_LocalDNSHostsPlugin(t *testing.T) {
	RunScenario(t, &Scenario{
		Description:      "Tests that localdns hosts plugin works correctly on Ubuntu 22.04 with dynamic IP resolution",
		K8sSystemPoolSKU: "Standard_D4s_v3",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				// Enable localdns and hosts plugin explicitly
				if nbc.AgentPoolProfile.LocalDNSProfile == nil {
					nbc.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{}
				}
				nbc.AgentPoolProfile.LocalDNSProfile.EnableLocalDNS = true
				nbc.AgentPoolProfile.LocalDNSProfile.EnableHostsPlugin = true
			},
			Validator: func(ctx context.Context, s *Scenario) {
				// Validate aks-hosts-setup service ran successfully and timer is active
				ValidateAKSHostsSetupService(ctx, s)

				// Validate hosts file contains resolved IPs for public cloud FQDNs
				ValidateLocalDNSHostsFile(ctx, s, []string{
					"mcr.microsoft.com",
					"login.microsoftonline.com",
					"acs-mirror.azureedge.net",
					"management.azure.com",
					"packages.aks.azure.com",
					"packages.microsoft.com",
				})

				// Validate localdns resolves fake FQDN from hosts file (proves hosts plugin bypass)
				ValidateLocalDNSHostsPluginBypass(ctx, s)
			},
		},
	})
}

// Test_Ubuntu2404_LocalDNSHostsPlugin tests the localdns hosts plugin feature on Ubuntu 24.04
func Test_Ubuntu2404_LocalDNSHostsPlugin(t *testing.T) {
	RunScenario(t, &Scenario{
		Description:      "Tests that localdns hosts plugin works correctly on Ubuntu 24.04",
		K8sSystemPoolSKU: "Standard_D4s_v3",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2404Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				// Enable localdns and hosts plugin explicitly
				if nbc.AgentPoolProfile.LocalDNSProfile == nil {
					nbc.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{}
				}
				nbc.AgentPoolProfile.LocalDNSProfile.EnableLocalDNS = true
				nbc.AgentPoolProfile.LocalDNSProfile.EnableHostsPlugin = true
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateAKSHostsSetupService(ctx, s)
				ValidateLocalDNSHostsFile(ctx, s, []string{
					"mcr.microsoft.com",
					"login.microsoftonline.com",
					"acs-mirror.azureedge.net",
				})
				ValidateLocalDNSHostsPluginBypass(ctx, s)
			},
		},
	})
}

// Test_AzureLinuxV3_LocalDNSHostsPlugin tests the localdns hosts plugin feature on Azure Linux V3
func Test_AzureLinuxV3_LocalDNSHostsPlugin(t *testing.T) {
	RunScenario(t, &Scenario{
		Description:      "Tests that localdns hosts plugin works correctly on Azure Linux V3",
		K8sSystemPoolSKU: "Standard_D4s_v3",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV3Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				// Enable localdns and hosts plugin explicitly
				if nbc.AgentPoolProfile.LocalDNSProfile == nil {
					nbc.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{}
				}
				nbc.AgentPoolProfile.LocalDNSProfile.EnableLocalDNS = true
				nbc.AgentPoolProfile.LocalDNSProfile.EnableHostsPlugin = true
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateAKSHostsSetupService(ctx, s)
				ValidateLocalDNSHostsFile(ctx, s, []string{
					"mcr.microsoft.com",
					"login.microsoftonline.com",
					"acs-mirror.azureedge.net",
				})
				ValidateLocalDNSHostsPluginBypass(ctx, s)
			},
		},
	})
}

// NOTE: UnknownCloud E2E tests have been removed because they fail during API server connectivity
// checks (exit code 52) before aks-hosts-setup runs. UnknownCloud scenarios are now covered by
// unit tests in spec/parts/linux/cloud-init/artifacts/aks_hosts_setup_spec.sh which test the
// script behavior directly without requiring full VM provisioning.

// Test_Ubuntu2204_LocalDNSHostsPlugin_Scriptless tests the localdns hosts plugin on scriptless path
func Test_Ubuntu2204_LocalDNSHostsPlugin_Scriptless(t *testing.T) {
	RunScenario(t, &Scenario{
		Description:      "Tests that localdns hosts plugin works correctly on Ubuntu 22.04 scriptless path (aks-node-controller)",
		K8sSystemPoolSKU: "Standard_D4s_v3",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			AKSNodeConfigMutator: func(aksNodeConfig *aksnodeconfigv1.Configuration) {
				// Enable localdns and hosts plugin via AKSNodeConfig (scriptless path)
				// Include DNS overrides to ensure corefile has health endpoint on port 8181
				aksNodeConfig.LocalDnsProfile = &aksnodeconfigv1.LocalDnsProfile{
					EnableLocalDns:       true,
					EnableHostsPlugin:    true,
					CpuLimitInMilliCores: to.Ptr(int32(2008)),
					MemoryLimitInMb:      to.Ptr(int32(128)),
					VnetDnsOverrides: map[string]*aksnodeconfigv1.LocalDnsOverrides{
						".": {
							QueryLogging:                "Log",
							Protocol:                    "PreferUDP",
							ForwardDestination:          "VnetDNS",
							ForwardPolicy:               "Sequential",
							MaxConcurrent:               to.Ptr(int32(1000)),
							CacheDurationInSeconds:      to.Ptr(int32(3600)),
							ServeStaleDurationInSeconds: to.Ptr(int32(3600)),
							ServeStale:                  "Verify",
						},
						"cluster.local": {
							QueryLogging:                "Error",
							Protocol:                    "ForceTCP",
							ForwardDestination:          "ClusterCoreDNS",
							ForwardPolicy:               "Sequential",
							MaxConcurrent:               to.Ptr(int32(1000)),
							CacheDurationInSeconds:      to.Ptr(int32(3600)),
							ServeStaleDurationInSeconds: to.Ptr(int32(3600)),
							ServeStale:                  "Disable",
						},
						"testdomain456.com": {
							QueryLogging:                "Log",
							Protocol:                    "PreferUDP",
							ForwardDestination:          "ClusterCoreDNS",
							ForwardPolicy:               "Sequential",
							MaxConcurrent:               to.Ptr(int32(1000)),
							CacheDurationInSeconds:      to.Ptr(int32(3600)),
							ServeStaleDurationInSeconds: to.Ptr(int32(3600)),
							ServeStale:                  "Verify",
						},
					},
					KubeDnsOverrides: map[string]*aksnodeconfigv1.LocalDnsOverrides{
						".": {
							QueryLogging:                "Error",
							Protocol:                    "PreferUDP",
							ForwardDestination:          "ClusterCoreDNS",
							ForwardPolicy:               "Sequential",
							MaxConcurrent:               to.Ptr(int32(1000)),
							CacheDurationInSeconds:      to.Ptr(int32(3600)),
							ServeStaleDurationInSeconds: to.Ptr(int32(3600)),
							ServeStale:                  "Verify",
						},
						"cluster.local": {
							QueryLogging:                "Log",
							Protocol:                    "ForceTCP",
							ForwardDestination:          "ClusterCoreDNS",
							ForwardPolicy:               "RoundRobin",
							MaxConcurrent:               to.Ptr(int32(1000)),
							CacheDurationInSeconds:      to.Ptr(int32(3600)),
							ServeStaleDurationInSeconds: to.Ptr(int32(3600)),
							ServeStale:                  "Disable",
						},
						"testdomain567.com": {
							QueryLogging:                "Error",
							Protocol:                    "PreferUDP",
							ForwardDestination:          "VnetDNS",
							ForwardPolicy:               "Random",
							MaxConcurrent:               to.Ptr(int32(1000)),
							CacheDurationInSeconds:      to.Ptr(int32(3600)),
							ServeStaleDurationInSeconds: to.Ptr(int32(3600)),
							ServeStale:                  "Immediate",
						},
					},
				}
			},
			Validator: func(ctx context.Context, s *Scenario) {
				// Validate aks-hosts-setup service ran successfully and timer is active
				ValidateAKSHostsSetupService(ctx, s)

				// Validate hosts file contains resolved IPs for public cloud FQDNs
				ValidateLocalDNSHostsFile(ctx, s, []string{
					"mcr.microsoft.com",
					"login.microsoftonline.com",
					"acs-mirror.azureedge.net",
					"management.azure.com",
					"packages.aks.azure.com",
					"packages.microsoft.com",
				})

				// Validate localdns resolves fake FQDN from hosts file (proves hosts plugin bypass)
				ValidateLocalDNSHostsPluginBypass(ctx, s)
			},
		},
	})
}

// Test_Ubuntu2204_LocalDNSHostsPlugin_DynamicSelection tests the self-healing dynamic corefile selection
// This test simulates the scenario where:
// 1. Node provisions with hosts plugin enabled but hosts file doesn't exist yet (aks-hosts-setup timed out)
// 2. localdns starts with NO_HOSTS corefile variant
// 3. aks-hosts-setup timer fires later and populates /etc/localdns/hosts
// 4. localdns restarts and dynamically switches to WITH_HOSTS corefile variant
func Test_Ubuntu2204_LocalDNSHostsPlugin_DynamicSelection(t *testing.T) {
	RunScenario(t, &Scenario{
		Description:      "Tests dynamic corefile selection and self-healing behavior when hosts file appears after initial provisioning",
		K8sSystemPoolSKU: "Standard_D4s_v3",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				// Enable localdns and hosts plugin explicitly
				if nbc.AgentPoolProfile.LocalDNSProfile == nil {
					nbc.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{}
				}
				nbc.AgentPoolProfile.LocalDNSProfile.EnableLocalDNS = true
				nbc.AgentPoolProfile.LocalDNSProfile.EnableHostsPlugin = true
			},
			Validator: func(ctx context.Context, s *Scenario) {
				// Step 1: Verify initial state - localdns should be running
				ValidateLocalDNSIsRunning(ctx, s)

				// Step 2: Check which corefile variant is currently in use
				initialCorefileHasHostsPlugin := ValidateLocalDNSCorefileVariant(ctx, s)
				t.Logf("Initial corefile has hosts plugin: %v", initialCorefileHasHostsPlugin)

				// Step 3: Verify hosts file exists (aks-hosts-setup should have run by now)
				ValidateLocalDNSHostsFile(ctx, s, []string{
					"mcr.microsoft.com",
					"login.microsoftonline.com",
				})

				// Step 4: Verify environment file has all required variables for dynamic selection
				ValidateLocalDNSEnvironmentFile(ctx, s)

				// Step 5: Simulate localdns restart and verify it picks the correct corefile
				// Delete the hosts file temporarily
				ExecOnVMSS(ctx, t, s.Cluster, s.AzureClient, s.Runtime, "sudo rm -f /etc/localdns/hosts", s.K8sUserPoolName)

				// Restart localdns - should use NO_HOSTS variant
				ExecOnVMSS(ctx, t, s.Cluster, s.AzureClient, s.Runtime, "sudo systemctl restart localdns", s.K8sUserPoolName)

				// Wait for localdns to come back up
				WaitForLocalDNSToBeReady(ctx, s)

				// Verify it's using NO_HOSTS variant now
				corefileAfterDelete := ValidateLocalDNSCorefileVariant(ctx, s)
				if corefileAfterDelete {
					t.Fatalf("Expected corefile WITHOUT hosts plugin after deleting hosts file, but found WITH hosts plugin")
				}
				t.Logf("✅ After deleting hosts file: corefile correctly uses NO_HOSTS variant")

				// Step 6: Trigger aks-hosts-setup to repopulate the hosts file
				ExecOnVMSS(ctx, t, s.Cluster, s.AzureClient, s.Runtime, "sudo systemctl start aks-hosts-setup.service", s.K8sUserPoolName)

				// Wait for hosts file to be populated
				WaitForHostsFileToExist(ctx, s)

				// Step 7: Restart localdns again - should now use WITH_HOSTS variant (self-healing!)
				ExecOnVMSS(ctx, t, s.Cluster, s.AzureClient, s.Runtime, "sudo systemctl restart localdns", s.K8sUserPoolName)

				// Wait for localdns to come back up
				WaitForLocalDNSToBeReady(ctx, s)

				// Verify it's now using WITH_HOSTS variant
				corefileAfterRestore := ValidateLocalDNSCorefileVariant(ctx, s)
				if !corefileAfterRestore {
					t.Fatalf("Expected corefile WITH hosts plugin after hosts file was restored, but found WITHOUT hosts plugin")
				}
				t.Logf("✅ After restoring hosts file: corefile correctly switched to WITH_HOSTS variant")

				// Step 8: Final validation - ensure localdns is still functional
				ValidateLocalDNSHostsPluginBypass(ctx, s)
				t.Logf("✅ Self-healing behavior verified: localdns dynamically switched corefile variants based on hosts file state")
			},
		},
	})
}
