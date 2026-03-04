package e2e

import (
	"context"
	"testing"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// Test_AzureLinuxV3_LocalDNSHostsPlugin_China tests cloud-specific FQDN selection for Azure China Cloud on Azure Linux V3
func Test_AzureLinuxV3_LocalDNSHostsPlugin_China(t *testing.T) {
	RunScenario(t, &Scenario{
		Description:      "Tests that localdns hosts plugin resolves China-specific FQDNs correctly on Azure Linux V3 (cross-distro × cross-cloud)",
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

				// Override location to chinaeast to trigger TARGET_CLOUD=AzureChinaCloud
				nbc.ContainerService.Location = "chinaeast"
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateAKSHostsSetupService(ctx, s)

				// Validate China-specific FQDNs are in the hosts file
				ValidateLocalDNSHostsFile(ctx, s, []string{
					"mcr.azure.cn",                     // China container registry
					"login.partner.microsoftonline.cn", // China Azure AD
					"management.chinacloudapi.cn",      // China ARM
					"acs-mirror.azureedge.net",         // K8s binaries mirror (common)
					"packages.microsoft.com",           // Microsoft packages (common)
				})

				ValidateLocalDNSHostsPluginBypass(ctx, s)
			},
		},
	})
}

// Test_AzureLinuxV3_LocalDNSHostsPlugin_Fairfax tests cloud-specific FQDN selection for Azure US Government Cloud on Azure Linux V3
func Test_AzureLinuxV3_LocalDNSHostsPlugin_Fairfax(t *testing.T) {
	RunScenario(t, &Scenario{
		Description:      "Tests that localdns hosts plugin resolves Fairfax-specific FQDNs correctly on Azure Linux V3 (cross-distro × cross-cloud)",
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

				// Override location to usgovvirginia to trigger TARGET_CLOUD=AzureUSGovernmentCloud
				nbc.ContainerService.Location = "usgovvirginia"
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateAKSHostsSetupService(ctx, s)

				// Validate Fairfax-specific FQDNs are in the hosts file
				ValidateLocalDNSHostsFile(ctx, s, []string{
					"mcr.microsoft.com",            // Container registry
					"login.microsoftonline.us",     // Azure AD (US Gov)
					"management.usgovcloudapi.net", // ARM (US Gov)
					"acs-mirror.azureedge.net",     // K8s binaries mirror (common)
					"packages.microsoft.com",       // Microsoft packages (common)
					"packages.aks.azure.com",       // AKS packages
				})

				ValidateLocalDNSHostsPluginBypass(ctx, s)
			},
		},
	})
}

// Test_AzureLinuxV3_LocalDNSHostsPlugin_UnknownCloud tests that unknown/empty cloud environment
// causes aks-hosts-setup to exit early and not enable hosts plugin on Azure Linux V3
func Test_AzureLinuxV3_LocalDNSHostsPlugin_UnknownCloud(t *testing.T) {
	RunScenario(t, &Scenario{
		Description:      "Tests that unknown cloud environment causes graceful fallback without hosts plugin on Azure Linux V3 (cross-distro)",
		K8sSystemPoolSKU: "Standard_D4s_v3",
		Tags: Tags{
			MockUnknownCloud: true,
		},
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
				// Note: TARGET_CLOUD will be set to "UnsupportedCloudE2ETest" by vmss.go
			},
			Validator: func(ctx context.Context, s *Scenario) {
				// Validate that aks-hosts-setup.service failed gracefully (exit 1)
				serviceScript := `set -euo pipefail
svc="aks-hosts-setup.service"
result=$(systemctl show -p Result "$svc" --value 2>/dev/null || echo "unknown")
echo "aks-hosts-setup.service result: $result"
if [ "$result" != "exit-code" ]; then
    echo "ERROR: Expected aks-hosts-setup.service to fail with exit-code, got: $result"
    systemctl status "$svc" --no-pager || true
    journalctl -u "$svc" --no-pager -n 50 || true
    exit 1
fi

# Verify the service exited with code 1 (graceful failure)
exit_code=$(systemctl show -p ExecMainStatus "$svc" --value 2>/dev/null || echo "0")
echo "aks-hosts-setup.service exit code: $exit_code"
if [ "$exit_code" != "1" ]; then
    echo "ERROR: Expected exit code 1, got: $exit_code"
    journalctl -u "$svc" --no-pager -n 50 || true
    exit 1
fi

# Verify error message in logs mentions unrecognized cloud
if ! journalctl -u "$svc" --no-pager | grep -q "Unrecognized cloud environment"; then
    echo "ERROR: Expected error message about unrecognized cloud environment"
    journalctl -u "$svc" --no-pager -n 50 || true
    exit 1
fi
`
				execScriptOnVMForScenarioValidateExitCode(ctx, s, serviceScript, 0,
					"aks-hosts-setup.service should fail gracefully when cloud is unrecognized")

				// Validate that /etc/localdns/hosts was not created (or is empty)
				hostsCheckScript := `set -euo pipefail
if [ ! -f "/etc/localdns/hosts" ]; then
    echo "/etc/localdns/hosts does not exist (expected)"
    exit 0
fi

# If file exists, it should be empty or have no IP mappings
if grep -qE '^[0-9a-fA-F.:]+[[:space:]]+[a-zA-Z]' /etc/localdns/hosts; then
    echo "ERROR: /etc/localdns/hosts should not contain IP mappings"
    cat /etc/localdns/hosts
    exit 1
fi

echo "/etc/localdns/hosts is empty or has no mappings (expected)"
`
				execScriptOnVMForScenarioValidateExitCode(ctx, s, hostsCheckScript, 0,
					"/etc/localdns/hosts should not be populated when cloud is unrecognized")

				// Validate that the node annotation was NOT set
				s.T.Log("Verifying node does NOT have localdns-hosts-plugin annotation...")
				node, err := s.Runtime.Cluster.Kube.Typed.CoreV1().Nodes().Get(ctx, s.Runtime.VM.KubeName, metav1.GetOptions{})
				require.NoError(s.T, err, "failed to get node %q", s.Runtime.VM.KubeName)

				annotationKey := "kubernetes.azure.com/localdns-hosts-plugin"
				_, exists := node.Annotations[annotationKey]
				require.False(s.T, exists,
					"Node should NOT have annotation %q when cloud is unrecognized", annotationKey)
				s.T.Logf("Confirmed node does not have annotation %q (expected for unrecognized cloud)", annotationKey)

				// Validate that updated.localdns.corefile does NOT have hosts plugin
				corefileCheckScript := `set -euo pipefail
corefile="/opt/azure/containers/localdns/updated.localdns.corefile"
if [ ! -f "$corefile" ]; then
    echo "ERROR: $corefile does not exist"
    exit 1
fi

if grep -q "hosts /etc/localdns/hosts" "$corefile"; then
    echo "ERROR: Corefile should not contain hosts plugin when TARGET_CLOUD is unset"
    cat "$corefile"
    exit 1
fi

echo "Corefile does not contain hosts plugin (expected)"
`
				execScriptOnVMForScenarioValidateExitCode(ctx, s, corefileCheckScript, 0,
					"updated.localdns.corefile should not contain hosts plugin when cloud is unrecognized")
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

				// Override location to chinaeast to trigger TARGET_CLOUD=AzureChinaCloud
				// GetCloudTargetEnv() in pkg/agent/utils.go uses location prefix to determine cloud
				nbc.ContainerService.Location = "chinaeast"
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateAKSHostsSetupService(ctx, s)

				// Validate China-specific FQDNs are in the hosts file
				ValidateLocalDNSHostsFile(ctx, s, []string{
					"mcr.azure.cn",                     // China container registry
					"login.partner.microsoftonline.cn", // China Azure AD
					"management.chinacloudapi.cn",      // China ARM
					"acs-mirror.azureedge.net",         // K8s binaries mirror (common)
					"packages.microsoft.com",           // Microsoft packages (common)
				})

				ValidateLocalDNSHostsPluginBypass(ctx, s)
			},
		},
	})
}

// Test_Ubuntu2204_LocalDNSHostsPlugin_Fairfax tests cloud-specific FQDN selection for Azure US Government Cloud
func Test_Ubuntu2204_LocalDNSHostsPlugin_Fairfax(t *testing.T) {
	RunScenario(t, &Scenario{
		Description:      "Tests that localdns hosts plugin resolves Fairfax-specific FQDNs correctly",
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

				// Override location to usgovvirginia to trigger TARGET_CLOUD=AzureUSGovernmentCloud
				// GetCloudTargetEnv() in pkg/agent/utils.go uses location prefix to determine cloud
				nbc.ContainerService.Location = "usgovvirginia"
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateAKSHostsSetupService(ctx, s)

				// Validate Fairfax-specific FQDNs are in the hosts file
				ValidateLocalDNSHostsFile(ctx, s, []string{
					"mcr.microsoft.com",            // Container registry
					"login.microsoftonline.us",     // Azure AD (US Gov)
					"management.usgovcloudapi.net", // ARM (US Gov)
					"acs-mirror.azureedge.net",     // K8s binaries mirror (common)
					"packages.microsoft.com",       // Microsoft packages (common)
					"packages.aks.azure.com",       // AKS packages
				})

				ValidateLocalDNSHostsPluginBypass(ctx, s)
			},
		},
	})
}

// Test_Ubuntu2204_LocalDNSHostsPlugin_UnknownCloud tests that unknown/empty cloud environment
// causes aks-hosts-setup to exit early and not enable hosts plugin
func Test_Ubuntu2204_LocalDNSHostsPlugin_UnknownCloud(t *testing.T) {
	RunScenario(t, &Scenario{
		Description:      "Tests that unknown cloud environment causes graceful fallback without hosts plugin",
		K8sSystemPoolSKU: "Standard_D4s_v3",
		Tags: Tags{
			MockUnknownCloud: true,
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
				// Note: TARGET_CLOUD will be set to "UnsupportedCloudE2ETest" by vmss.go
				// This tests the wildcard (*) case in aks-hosts-setup.sh which now exits with error
			},
			Validator: func(ctx context.Context, s *Scenario) {
				// Validate that aks-hosts-setup.service failed gracefully (exit 1)
				// but the node still provisions successfully
				serviceScript := `set -euo pipefail
svc="aks-hosts-setup.service"
result=$(systemctl show -p Result "$svc" --value 2>/dev/null || echo "unknown")
echo "aks-hosts-setup.service result: $result"
if [ "$result" != "exit-code" ]; then
    echo "ERROR: Expected aks-hosts-setup.service to fail with exit-code, got: $result"
    systemctl status "$svc" --no-pager || true
    journalctl -u "$svc" --no-pager -n 50 || true
    exit 1
fi

# Verify the service exited with code 1 (graceful failure)
exit_code=$(systemctl show -p ExecMainStatus "$svc" --value 2>/dev/null || echo "0")
echo "aks-hosts-setup.service exit code: $exit_code"
if [ "$exit_code" != "1" ]; then
    echo "ERROR: Expected exit code 1, got: $exit_code"
    journalctl -u "$svc" --no-pager -n 50 || true
    exit 1
fi

# Verify error message in logs mentions unrecognized cloud
if ! journalctl -u "$svc" --no-pager | grep -q "Unrecognized cloud environment"; then
    echo "ERROR: Expected error message about unrecognized cloud environment"
    journalctl -u "$svc" --no-pager -n 50 || true
    exit 1
fi
`
				execScriptOnVMForScenarioValidateExitCode(ctx, s, serviceScript, 0,
					"aks-hosts-setup.service should fail gracefully when cloud is unrecognized")

				// Validate that /etc/localdns/hosts was not created (or is empty)
				hostsCheckScript := `set -euo pipefail
if [ ! -f "/etc/localdns/hosts" ]; then
    echo "/etc/localdns/hosts does not exist (expected)"
    exit 0
fi

# If file exists, it should be empty or have no IP mappings
if grep -qE '^[0-9a-fA-F.:]+[[:space:]]+[a-zA-Z]' /etc/localdns/hosts; then
    echo "ERROR: /etc/localdns/hosts should not contain IP mappings"
    cat /etc/localdns/hosts
    exit 1
fi

echo "/etc/localdns/hosts is empty or has no mappings (expected)"
`
				execScriptOnVMForScenarioValidateExitCode(ctx, s, hostsCheckScript, 0,
					"/etc/localdns/hosts should not be populated when cloud is unrecognized")

				// Validate that the node annotation was NOT set
				s.T.Log("Verifying node does NOT have localdns-hosts-plugin annotation...")
				node, err := s.Runtime.Cluster.Kube.Typed.CoreV1().Nodes().Get(ctx, s.Runtime.VM.KubeName, metav1.GetOptions{})
				require.NoError(s.T, err, "failed to get node %q", s.Runtime.VM.KubeName)

				annotationKey := "kubernetes.azure.com/localdns-hosts-plugin"
				_, exists := node.Annotations[annotationKey]
				require.False(s.T, exists,
					"Node should NOT have annotation %q when cloud is unrecognized", annotationKey)
				s.T.Logf("Confirmed node does not have annotation %q (expected for unrecognized cloud)", annotationKey)

				// Validate that updated.localdns.corefile does NOT have hosts plugin
				corefileCheckScript := `set -euo pipefail
corefile="/opt/azure/containers/localdns/updated.localdns.corefile"
if [ ! -f "$corefile" ]; then
    echo "ERROR: $corefile does not exist"
    exit 1
fi

if grep -q "hosts /etc/localdns/hosts" "$corefile"; then
    echo "ERROR: Corefile should not contain hosts plugin when TARGET_CLOUD is unset"
    cat "$corefile"
    exit 1
fi

echo "Corefile does not contain hosts plugin (expected)"
`
				execScriptOnVMForScenarioValidateExitCode(ctx, s, corefileCheckScript, 0,
					"updated.localdns.corefile should not contain hosts plugin when cloud is unrecognized")
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
				aksNodeConfig.LocalDnsProfile = &aksnodeconfigv1.LocalDnsProfile{
					EnableLocalDns:    true,
					EnableHostsPlugin: true,
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
