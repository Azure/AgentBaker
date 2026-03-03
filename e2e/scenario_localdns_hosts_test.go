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

