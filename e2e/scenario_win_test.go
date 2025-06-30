package e2e

import (
	"context"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
)

func EmptyBootstrapConfigMutator(configuration *datamodel.NodeBootstrappingConfiguration) {}
func EmptyVMConfigMutator(vmss *armcompute.VirtualMachineScaleSet)                        {}

func DualStackConfigMutator(configuration *datamodel.NodeBootstrappingConfiguration) {
	properties := configuration.ContainerService.Properties
	properties.FeatureFlags.EnableIPv6DualStack = true
	kubernetesConfig := properties.OrchestratorProfile.KubernetesConfig
	kubernetesConfig.ServiceCIDRs = []string{
		kubernetesConfig.ServiceCIDR,
		"fd12::/108",
	}
	kubernetesConfig.ClusterSubnets = []string{
		kubernetesConfig.ClusterSubnet,
		"fd13::/64",
	}

	allClusterCidrs := getAllClusterCidrs(kubernetesConfig)
	kubernetesConfig.ClusterSubnet = strings.Join(allClusterCidrs, ",")

}

func getAllClusterCidrs(k *datamodel.KubernetesConfig) []string {
	return k.ClusterSubnets
}

// WS2019 doesn't support IPv6, so we don't test it with dual-stack.
func Test_Windows2019AzureNetwork(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2019 Azure Network",
		Config: Config{
			Cluster:                ClusterAzureNetwork,
			VHD:                    config.VHDWindows2019Containerd,
			VMConfigMutator:        EmptyVMConfigMutator,
			BootstrapConfigMutator: EmptyBootstrapConfigMutator,
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateWindowsVersionFromWindowsSettings(ctx, s, "2019-containerd")
				ValidateWindowsProductName(ctx, s, "Windows Server 2019 Datacenter")
				ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote")
				ValidateWindowsProcessHasCliArguments(ctx, s, "kubelet.exe", []string{"--rotate-certificates=true", "--client-ca-file=c:\\k\\ca.crt"})
				ValidateCiliumIsNotRunningWindows(ctx, s)
				ValidateFileHasContent(ctx, s, "/AzureData/CustomDataSetupScript.log", "CSEScriptsPackageUrl used for provision is https://packages.aks.azure.com/aks/windows/cse/aks-windows-cse-scripts-current.zip")
			},
		},
	})
}

func Test_Windows2022_AzureNetwork(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2022 Azure Network",
		Config: Config{
			Cluster:                ClusterAzureNetwork,
			VHD:                    config.VHDWindows2022Containerd,
			VMConfigMutator:        EmptyVMConfigMutator,
			BootstrapConfigMutator: EmptyBootstrapConfigMutator,
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateWindowsVersionFromWindowsSettings(ctx, s, "2022-containerd")
				ValidateWindowsProductName(ctx, s, "Windows Server 2022 Datacenter")
				ValidateWindowsDisplayVersion(ctx, s, "21H2")
				ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote")
				ValidateWindowsProcessHasCliArguments(ctx, s, "kubelet.exe", []string{"--rotate-certificates=true", "--client-ca-file=c:\\k\\ca.crt"})
				ValidateCiliumIsNotRunningWindows(ctx, s)
			},
		},
	})
}

func Test_Windows2022AzureOverlayNetworkDualStack(t *testing.T) {
	//t.Skip("Dual stack tests are not working yet")
	RunScenario(t, &Scenario{
		Description: "Windows Server 2022 Azure Overlay Network Dual Stack",
		Config: Config{
			Cluster: ClusterAzureOverlayNetworkDualStack,
			VHD:     config.VHDWindows2022Containerd,
			VMConfigMutator: func(set *armcompute.VirtualMachineScaleSet) {
				ip4Config := set.Properties.VirtualMachineProfile.NetworkProfile.NetworkInterfaceConfigurations[0].Properties.IPConfigurations[0]

				ip6Config := &armcompute.VirtualMachineScaleSetIPConfiguration{
					Name: to.Ptr(fmt.Sprintf("%s_1", *ip4Config.Name)),
					Properties: &armcompute.VirtualMachineScaleSetIPConfigurationProperties{
						Primary:                 to.Ptr(false),
						PrivateIPAddressVersion: to.Ptr(armcompute.IPVersionIPv6),
						Subnet: &armcompute.APIEntityReference{
							ID: ip4Config.Properties.Subnet.ID,
						},
					},
				}

				set.Properties.VirtualMachineProfile.NetworkProfile.NetworkInterfaceConfigurations[0].Properties.IPConfigurations = []*armcompute.VirtualMachineScaleSetIPConfiguration{
					ip4Config,
					ip6Config,
				}
			},
			BootstrapConfigMutator: DualStackConfigMutator,
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateWindowsVersionFromWindowsSettings(ctx, s, "2022-containerd")
				ValidateWindowsProductName(ctx, s, "Windows Server 2022 Datacenter")
				ValidateWindowsDisplayVersion(ctx, s, "21H2")
				ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote")
				ValidateWindowsProcessHasCliArguments(ctx, s, "kubelet.exe", []string{"--rotate-certificates=true", "--client-ca-file=c:\\k\\ca.crt"})
				ValidateCiliumIsNotRunningWindows(ctx, s)
			},
		},
	})
}

func Test_Windows2022Gen2AzureNetwork(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2022 with Azure Network - hyperv gen2",
		Config: Config{
			Cluster:                ClusterAzureNetwork,
			VHD:                    config.VHDWindows2022ContainerdGen2,
			VMConfigMutator:        EmptyVMConfigMutator,
			BootstrapConfigMutator: EmptyBootstrapConfigMutator,
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateWindowsVersionFromWindowsSettings(ctx, s, "2022-containerd-gen2")
				ValidateWindowsProductName(ctx, s, "Windows Server 2022 Datacenter")
				ValidateWindowsDisplayVersion(ctx, s, "21H2")
				ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote")
				ValidateWindowsProcessHasCliArguments(ctx, s, "kubelet.exe", []string{"--rotate-certificates=true", "--client-ca-file=c:\\k\\ca.crt"})
				ValidateCiliumIsNotRunningWindows(ctx, s)
				ValidateFileHasContent(ctx, s, "/AzureData/CustomDataSetupScript.log", "CSEScriptsPackageUrl used for provision is https://packages.aks.azure.com/aks/windows/cse/aks-windows-cse-scripts-current.zip")
			},
		},
	})
}

func Test_Windows2022Gen2AzureOverlayNetworkDualStack(t *testing.T) {
	t.Skip("Dual stack tests are not working yet")
	RunScenario(t, &Scenario{
		Description: "Windows Server 2022 with Azure Overlay Network Dual Stack - hyperv gen 2",
		Config: Config{
			Cluster:                ClusterAzureOverlayNetworkDualStack,
			VHD:                    config.VHDWindows2022ContainerdGen2,
			VMConfigMutator:        EmptyVMConfigMutator,
			BootstrapConfigMutator: DualStackConfigMutator,
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateWindowsVersionFromWindowsSettings(ctx, s, "2022-containerd-gen2")
				ValidateWindowsProductName(ctx, s, "Windows Server 2022 Datacenter")
				ValidateWindowsDisplayVersion(ctx, s, "21H2")
				ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote")
				ValidateWindowsProcessHasCliArguments(ctx, s, "kubelet.exe", []string{"--rotate-certificates=true", "--client-ca-file=c:\\k\\ca.crt"})
				ValidateCiliumIsNotRunningWindows(ctx, s)
				ValidateFileHasContent(ctx, s, "/AzureData/CustomDataSetupScript.log", "CSEScriptsPackageUrl used for provision is https://packages.aks.azure.com/aks/windows/cse/aks-windows-cse-scripts-current.zip")
			},
		},
	})
}

func Test_Windows23H2AzureNetwork(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 23H2 with Azure Network",
		Config: Config{
			Cluster:                ClusterAzureNetwork,
			VHD:                    config.VHDWindows23H2,
			VMConfigMutator:        EmptyVMConfigMutator,
			BootstrapConfigMutator: EmptyBootstrapConfigMutator,
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateWindowsVersionFromWindowsSettings(ctx, s, "23H2")
				ValidateWindowsProductName(ctx, s, "Windows Server 2022 Datacenter")
				ValidateWindowsDisplayVersion(ctx, s, "23H2")
				ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote")
				ValidateWindowsProcessHasCliArguments(ctx, s, "kubelet.exe", []string{"--rotate-certificates=true", "--client-ca-file=c:\\k\\ca.crt"})
				ValidateCiliumIsNotRunningWindows(ctx, s)
			},
		},
	})
}

func Test_Windows23H2AzureOverlayNetworkDualStack(t *testing.T) {
	t.Skip("Dual stack tests are not working yet")
	RunScenario(t, &Scenario{
		Description: "Windows Server 23H2 with Azure Overlay Network Dual Stack",
		Config: Config{
			Cluster:                ClusterAzureOverlayNetworkDualStack,
			VHD:                    config.VHDWindows23H2,
			VMConfigMutator:        EmptyVMConfigMutator,
			BootstrapConfigMutator: DualStackConfigMutator,
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateWindowsVersionFromWindowsSettings(ctx, s, "23H2")
				ValidateWindowsProductName(ctx, s, "Windows Server 2022 Datacenter")
				ValidateWindowsDisplayVersion(ctx, s, "23H2")
				ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote")
				ValidateWindowsProcessHasCliArguments(ctx, s, "kubelet.exe", []string{"--rotate-certificates=true", "--client-ca-file=c:\\k\\ca.crt"})
				ValidateCiliumIsNotRunningWindows(ctx, s)
			},
		},
	})
}

func Test_Windows23H2Gen2AzureNetwork(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 23H2 with Azure Network - hyperv gen2",
		Config: Config{
			Cluster:                ClusterAzureNetwork,
			VHD:                    config.VHDWindows23H2Gen2,
			VMConfigMutator:        EmptyVMConfigMutator,
			BootstrapConfigMutator: EmptyBootstrapConfigMutator,
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateWindowsVersionFromWindowsSettings(ctx, s, "23H2-gen2")
				ValidateWindowsProductName(ctx, s, "Windows Server 2022 Datacenter")
				ValidateWindowsDisplayVersion(ctx, s, "23H2")
				ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote")
				ValidateWindowsProcessHasCliArguments(ctx, s, "kubelet.exe", []string{"--rotate-certificates=true", "--client-ca-file=c:\\k\\ca.crt"})
				ValidateCiliumIsNotRunningWindows(ctx, s)
				ValidateFileHasContent(ctx, s, "/AzureData/CustomDataSetupScript.log", "CSEScriptsPackageUrl used for provision is https://packages.aks.azure.com/aks/windows/cse/aks-windows-cse-scripts-current.zip")
			},
		},
	})
}

func Test_Windows23H2Gen2AzureOverlayDualStack(t *testing.T) {
	t.Skip("Dual stack tests are not working yet")
	RunScenario(t, &Scenario{
		Description: "Windows Server 23H2 with Azure Overlay Network Dual Stack - hyperv gen2",
		Config: Config{
			Cluster:                ClusterAzureOverlayNetworkDualStack,
			VHD:                    config.VHDWindows23H2Gen2,
			VMConfigMutator:        EmptyVMConfigMutator,
			BootstrapConfigMutator: DualStackConfigMutator,
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateWindowsVersionFromWindowsSettings(ctx, s, "23H2-gen2")
				ValidateWindowsProductName(ctx, s, "Windows Server 2022 Datacenter")
				ValidateWindowsDisplayVersion(ctx, s, "23H2")
				ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote")
				ValidateWindowsProcessHasCliArguments(ctx, s, "kubelet.exe", []string{"--rotate-certificates=true", "--client-ca-file=c:\\k\\ca.crt"})
				ValidateCiliumIsNotRunningWindows(ctx, s)
				ValidateFileHasContent(ctx, s, "/AzureData/CustomDataSetupScript.log", "CSEScriptsPackageUrl used for provision is https://packages.aks.azure.com/aks/windows/cse/aks-windows-cse-scripts-current.zip")
			},
		},
	})
}

func Test_Windows23H2Gen2CachingRegression(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows 23H2 VHD built before local cache enabled should still work - overwrite the CSE scripts package URL",
		Config: Config{
			Cluster:         ClusterAzureNetwork,
			VHD:             config.VHDWindows23H2Gen2,
			VMConfigMutator: EmptyVMConfigMutator,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.WindowsProfile.CseScriptsPackageURL = "https://packages.aks.azure.com/aks/windows/cse/aks-windows-cse-scripts-v0.0.52.zip"
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/AzureData/CustomDataSetupScript.log", "CSEScriptsPackageUrl used for provision is https://packages.aks.azure.com/aks/windows/cse/aks-windows-cse-scripts-v0.0.52.zip")
			},
		},
	})
}

func Test_Windows2022CachingRegression(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows 2022 VHD built before local cache enabled should still work - overwrite the CSE scripts package URL",
		Config: Config{
			Cluster:         ClusterAzureNetwork,
			VHD:             config.VHDWindows2022ContainerdGen2,
			VMConfigMutator: EmptyVMConfigMutator,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.WindowsProfile.CseScriptsPackageURL = "https://packages.aks.azure.com/aks/windows/cse/aks-windows-cse-scripts-v0.0.52.zip"
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/AzureData/CustomDataSetupScript.log", "CSEScriptsPackageUrl used for provision is https://packages.aks.azure.com/aks/windows/cse/aks-windows-cse-scripts-v0.0.52.zip")
			},
		},
	})
}

func Test_Windows2019CachingRegression(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows 2019 VHD built before local cache enabled should still work - overwrite the CSE scripts package URL",
		Config: Config{
			Cluster:         ClusterAzureNetwork,
			VHD:             config.VHDWindows2019Containerd,
			VMConfigMutator: EmptyVMConfigMutator,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.WindowsProfile.CseScriptsPackageURL = "https://packages.aks.azure.com/aks/windows/cse/aks-windows-cse-scripts-v0.0.52.zip"
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/AzureData/CustomDataSetupScript.log", "CSEScriptsPackageUrl used for provision is https://packages.aks.azure.com/aks/windows/cse/aks-windows-cse-scripts-v0.0.52.zip")
			},
		},
	})
}

func Test_Windows2025(t *testing.T) {
	t.Skip("skipping test for Windows 2025, as we are testing regression issues with k8s 1.31+")
	RunScenario(t, &Scenario{
		Description: "Windows Server 2025 with Containerd",
		Config: Config{
			Cluster:         ClusterAzureNetwork,
			VHD:             config.VHDWindows2025,
			VMConfigMutator: EmptyVMConfigMutator,
			BootstrapConfigMutator: func(configuration *datamodel.NodeBootstrappingConfiguration) {
				// 2025 supported in 1.32+ .
				configuration.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = "1.32.5"
				configuration.K8sComponents.WindowsPackageURL = fmt.Sprintf("https://packages.aks.azure.com/kubernetes/v%s/windowszip/v%s-1int.zip", "1.32.5", "1.32.5")
			},

			Validator: func(ctx context.Context, s *Scenario) {
				ValidateWindowsVersionFromWindowsSettings(ctx, s, "2025")
				ValidateWindowsProductName(ctx, s, "Windows Server 2025 Datacenter")
				ValidateWindowsDisplayVersion(ctx, s, "24H2")
				ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote")
				ValidateWindowsProcessHasCliArguments(ctx, s, "kubelet.exe", []string{"--rotate-certificates=true", "--client-ca-file=c:\\k\\ca.crt"})
				ValidateCiliumIsNotRunningWindows(ctx, s)
			},
		},
	})
}

func Test_Windows2025Gen2(t *testing.T) {
	t.Skip("skipping test for Windows 2025, as we are testing regression issues with k8s 1.31+")
	RunScenario(t, &Scenario{
		Description: "Windows Server 2025 with Containerd - hyperv gen 2",
		Config: Config{
			Cluster:         ClusterAzureNetwork,
			VHD:             config.VHDWindows2025Gen2,
			VMConfigMutator: EmptyVMConfigMutator,
			// BootstrapConfigMutator: EmptyBootstrapConfigMutator,
			BootstrapConfigMutator: func(configuration *datamodel.NodeBootstrappingConfiguration) {
				// 2025 supported in 1.32+ .
				configuration.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = "1.32.5"
				configuration.K8sComponents.WindowsPackageURL = fmt.Sprintf("https://packages.aks.azure.com/kubernetes/v%s/windowszip/v%s-1int.zip", "1.32.5", "1.32.5")
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateWindowsVersionFromWindowsSettings(ctx, s, "2025-gen2")
				ValidateWindowsProductName(ctx, s, "Windows Server 2025 Datacenter")
				ValidateWindowsDisplayVersion(ctx, s, "24H2")
				ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote")
				ValidateWindowsProcessHasCliArguments(ctx, s, "kubelet.exe", []string{"--rotate-certificates=true", "--client-ca-file=c:\\k\\ca.crt"})
				ValidateCiliumIsNotRunningWindows(ctx, s)
			},
		},
	})
}

func Test_Windows23H2_Cilium2(t *testing.T) {
	t.Skip("skipping test for Cilium on Windows 23H2, as it is not supported in production AKS yet")
	RunScenario(t, &Scenario{
		Description: "Windows Server 2022 with Containerd",
		Config: Config{
			Cluster:         ClusterCiliumNetwork,
			VHD:             config.VHDWindows23H2Gen2,
			VMConfigMutator: EmptyVMConfigMutator,
			BootstrapConfigMutator: func(configuration *datamodel.NodeBootstrappingConfiguration) {
				// cilium is only supported in 1.30 or greater.
				configuration.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = "1.30.9"
				configuration.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.EbpfDataplane = datamodel.EbpfDataplane_cilium
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote")
				ValidateWindowsProcessHasCliArguments(ctx, s, "kubelet.exe", []string{"--rotate-certificates=true", "--client-ca-file=c:\\k\\ca.crt"})
				ValidateCiliumIsRunningWindows(ctx, s)
			},
		},
	})
}
