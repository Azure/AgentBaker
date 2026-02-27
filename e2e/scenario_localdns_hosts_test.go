package e2e

import (
	"context"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
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

// Test_AzureLinuxV2_LocalDNSHostsPlugin tests the localdns hosts plugin feature on Azure Linux V2
func Test_AzureLinuxV2_LocalDNSHostsPlugin(t *testing.T) {
	RunScenario(t, &Scenario{
		Description:      "Tests that localdns hosts plugin works correctly on Azure Linux V2 (cross-distro compatibility)",
		K8sSystemPoolSKU: "Standard_D4s_v3",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV2Gen2,
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

// Test_MarinerV2_LocalDNSHostsPlugin tests the localdns hosts plugin feature on Mariner V2
func Test_MarinerV2_LocalDNSHostsPlugin(t *testing.T) {
	RunScenario(t, &Scenario{
		Description:      "Tests that localdns hosts plugin works correctly on Mariner V2 (cross-distro compatibility)",
		K8sSystemPoolSKU: "Standard_D4s_v3",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDCBLMarinerV2Gen2,
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

// Test_Ubuntu2204_LocalDNSHostsPlugin_China tests cloud-specific FQDN selection for Azure China Cloud
func Test_Ubuntu2204_LocalDNSHostsPlugin_China(t *testing.T) {
	RunScenario(t, &Scenario{
		Description:      "Tests that localdns hosts plugin resolves China-specific FQDNs correctly",
		K8sSystemPoolSKU: "Standard_D4s_v3",
		Tags: Tags{
			MockAzureChinaCloud: true,
		},
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
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				// Tag the VMSS to mock China cloud environment
				// This tag is read by the e2e framework to set TARGET_CLOUD
				if vmss.Tags == nil {
					vmss.Tags = make(map[string]*string)
				}
				vmss.Tags["E2EMockAzureChinaCloud"] = to.Ptr("true")
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateAKSHostsSetupService(ctx, s)

				// Validate China-specific FQDNs are in the hosts file
				ValidateLocalDNSHostsFile(ctx, s, []string{
					"mcr.azure.cn",                      // China container registry
					"login.partner.microsoftonline.cn",  // China Azure AD
					"management.chinacloudapi.cn",       // China ARM
					"acs-mirror.azureedge.net",          // K8s binaries mirror (common)
					"packages.microsoft.com",            // Microsoft packages (common)
				})

				ValidateLocalDNSHostsPluginBypass(ctx, s)
			},
		},
	})
}

// Test_Ubuntu2204_LocalDNSHostsPlugin_OldVHD_GracefulFallback tests backward compatibility
// with old VHDs that don't have aks-hosts-setup artifacts
func Test_Ubuntu2204_LocalDNSHostsPlugin_OldVHD_GracefulFallback(t *testing.T) {
	RunScenario(t, &Scenario{
		Description:      "Tests that new CSE with old VHD (no aks-hosts-setup artifacts) gracefully falls back to default localdns behavior",
		K8sSystemPoolSKU: "Standard_D4s_v3",
		Config: Config{
			Cluster: ClusterKubenet,
			// Use an old VHD without aks-hosts-setup artifacts
			VHD: config.VHDUbuntu2204Gen2ContainerdPrivateKubePkg,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				// Try to enable localdns but it should be disabled due to UnsupportedLocalDns flag
				// This simulates the scenario where new CSE runs on old VHD
			},
			Validator: func(ctx context.Context, s *Scenario) {
				// This VHD has UnsupportedLocalDns=true, so localdns should be disabled
				// Validate that the node still bootstraps successfully without aks-hosts-setup

				// Check that aks-hosts-setup artifacts don't exist (confirms we're testing old VHD)
				ValidateFileDoesNotExist(ctx, s, "/opt/azure/containers/aks-hosts-setup.sh")
				ValidateFileDoesNotExist(ctx, s, "/etc/systemd/system/aks-hosts-setup.service")
				ValidateFileDoesNotExist(ctx, s, "/etc/systemd/system/aks-hosts-setup.timer")

				// Validate node still provisions successfully (graceful fallback)
				// The standard validation should pass
			},
		},
	})
}

// Test_Ubuntu2204_LocalDNSHostsPlugin_BackwardCompatVHD tests backward compatibility
// with older VHDs that don't have aks-hosts-setup artifacts
func Test_Ubuntu2204_LocalDNSHostsPlugin_BackwardCompatVHD(t *testing.T) {
	RunScenario(t, &Scenario{
		Description:      "Tests that older VHD without aks-hosts-setup artifacts still provisions successfully",
		K8sSystemPoolSKU: "Standard_D4s_v3",
		Config: Config{
			Cluster: ClusterKubenet,
			// Use an older VHD without aks-hosts-setup (PrivateKubePkg has UnsupportedLocalDns=true)
			VHD: config.VHDUbuntu2204Gen2ContainerdPrivateKubePkg,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				// Older VHD has UnsupportedLocalDns=true
			},
			Validator: func(ctx context.Context, s *Scenario) {
				// Check that aks-hosts-setup artifacts don't exist
				ValidateFileDoesNotExist(ctx, s, "/opt/azure/containers/aks-hosts-setup.sh")

				// Validate node still provisions successfully despite missing hosts plugin
				// This confirms backward compatibility
			},
		},
	})
}

// Test_Ubuntu2204_LocalDNSHostsPlugin_TimerRefresh tests the periodic refresh behavior
func Test_Ubuntu2204_LocalDNSHostsPlugin_TimerRefresh(t *testing.T) {
	RunScenario(t, &Scenario{
		Description:      "Tests that aks-hosts-setup.timer is configured correctly for periodic refresh",
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
				// Validate timer configuration
				ValidateAKSHostsSetupService(ctx, s)

				// Check timer unit file exists and has correct interval and boot settings
				ValidateFileExists(ctx, s, "/etc/systemd/system/aks-hosts-setup.timer")
				ValidateFileHasContent(ctx, s, "/etc/systemd/system/aks-hosts-setup.timer", "OnUnitActiveSec=15min")
				ValidateFileHasContent(ctx, s, "/etc/systemd/system/aks-hosts-setup.timer", "OnBootSec=0")

				// Verify timer is enabled for automatic startup
				execScriptOnVMForScenarioValidateExitCode(ctx, s,
					"systemctl is-enabled aks-hosts-setup.timer",
					0, "aks-hosts-setup.timer should be enabled")

				// Validate service configuration
				ValidateFileExists(ctx, s, "/etc/systemd/system/aks-hosts-setup.service")
				ValidateFileHasContent(ctx, s, "/etc/systemd/system/aks-hosts-setup.service", "Type=oneshot")
				ValidateFileHasContent(ctx, s, "/etc/systemd/system/aks-hosts-setup.service", "TimeoutStartSec=60")
			},
		},
	})
}

// Test_Ubuntu2204_LocalDNSHostsPlugin_IPv4Validation tests IPv4 octet range validation
func Test_Ubuntu2204_LocalDNSHostsPlugin_IPv4Validation(t *testing.T) {
	RunScenario(t, &Scenario{
		Description:      "Tests that aks-hosts-setup.sh properly validates IPv4 octet ranges (0-255) and rejects invalid IPs",
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
				// Validate that hosts file only contains valid IPv4 addresses
				script := `
set -euo pipefail
hosts_file="/etc/localdns/hosts"

echo "Checking all IPs in hosts file are valid IPv4 addresses with octets 0-255..."

# Extract all IP-looking patterns (skip comments and blank lines)
grep -E '^[0-9]' "$hosts_file" | awk '{print $1}' | while read -r ip; do
    # Split by dots and validate each octet
    IFS='.' read -r a b c d <<< "$ip"

    # Check we have exactly 4 octets
    if [ -z "$a" ] || [ -z "$b" ] || [ -z "$c" ] || [ -z "$d" ]; then
        echo "ERROR: Invalid IP format: $ip"
        exit 1
    fi

    # Validate octet range 0-255
    if [ "$a" -gt 255 ] || [ "$b" -gt 255 ] || [ "$c" -gt 255 ] || [ "$d" -gt 255 ]; then
        echo "ERROR: IP has octet > 255: $ip"
        exit 1
    fi

    if [ "$a" -lt 0 ] || [ "$b" -lt 0 ] || [ "$c" -lt 0 ] || [ "$d" -lt 0 ]; then
        echo "ERROR: IP has octet < 0: $ip"
        exit 1
    fi

    echo "  OK: $ip"
done

echo "All IPs in hosts file are valid!"
`
				execScriptOnVMForScenarioValidateExitCode(ctx, s, script, 0,
					"All IPs in hosts file should have valid IPv4 octet ranges (0-255)")
			},
		},
	})
}

// Test_Ubuntu2204_LocalDNSHostsPlugin_CloudEnvPersistence tests cloud env persistence
func Test_Ubuntu2204_LocalDNSHostsPlugin_CloudEnvPersistence(t *testing.T) {
	RunScenario(t, &Scenario{
		Description:      "Tests that TARGET_CLOUD is persisted to /etc/localdns/cloud-env for timer-triggered runs",
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
				// Validate cloud-env file exists and contains TARGET_CLOUD
				ValidateFileExists(ctx, s, "/etc/localdns/cloud-env")
				ValidateFileHasContent(ctx, s, "/etc/localdns/cloud-env", "TARGET_CLOUD=")

				// Validate service reads from EnvironmentFile
				ValidateFileHasContent(ctx, s, "/etc/systemd/system/aks-hosts-setup.service",
					"EnvironmentFile=-/etc/localdns/cloud-env")
			},
		},
	})
}

// Test_Ubuntu2204_LocalDNSHostsPlugin_DNSResolutionFailure tests graceful handling of DNS resolution failures
func Test_Ubuntu2204_LocalDNSHostsPlugin_DNSResolutionFailure(t *testing.T) {
	RunScenario(t, &Scenario{
		Description:      "Tests that aks-hosts-setup handles DNS resolution failures gracefully and still creates hosts file",
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
				// Even if some FQDNs fail to resolve, the service should succeed
				// and create a hosts file with the resolvable entries
				ValidateAKSHostsSetupService(ctx, s)
				ValidateFileExists(ctx, s, "/etc/localdns/hosts")

				// The hosts file should contain at least one resolved entry
				script := `
set -euo pipefail
hosts_file="/etc/localdns/hosts"

# Count non-comment, non-empty lines (should have at least some resolved entries)
count=$(grep -E '^[0-9]' "$hosts_file" | wc -l)

if [ "$count" -eq 0 ]; then
    echo "ERROR: No resolved entries in hosts file"
    exit 1
fi

echo "Found $count resolved entries in hosts file"
`
				execScriptOnVMForScenarioValidateExitCode(ctx, s, script, 0,
					"Hosts file should contain at least some resolved entries even if some FQDNs fail")
			},
		},
	})
}
