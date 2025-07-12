package e2e

import (
	"context"
	"fmt"
	"testing"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/toolkit"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
)

func Test_AzureLinuxV2_AirGap(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using a AzureLinuxV2 (CgroupV2) VHD can be properly bootstrapped",
		Tags: Tags{
			Airgap: true,
		},
		Config: Config{
			Cluster: ClusterKubenetAirgap,
			VHD:     config.VHDAzureLinuxV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.OutboundType = datamodel.OutboundTypeBlock
				nbc.ContainerService.Properties.SecurityProfile = &datamodel.SecurityProfile{
					PrivateEgress: &datamodel.PrivateEgress{
						Enabled:                 true,
						ContainerRegistryServer: fmt.Sprintf("%s.azurecr.io", config.PrivateACRName(config.Config.DefaultLocation)),
					},
				}
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateDirectoryContent(ctx, s, "/run", []string{"outbound-check-skipped"})
			},
		},
	})
}

func Test_AzureLinuxV2_SecureTLSBootstrapping_BootstrapToken_Fallback(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using a AzureLinuxV2 (CgroupV2) VHD can be properly bootstrapped even if secure TLS bootstrapping fails",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				// secure TLS bootstrapping is not yet enabled in e2e regions, thus this will test the bootstrap token fallback case
				nbc.EnableSecureTLSBootstrapping = true
			},
		},
	})
}

func Test_AzureLinuxV2_ARM64(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using a AzureLinuxV2 (CgroupV2) VHD on ARM64 architecture can be properly bootstrapped",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV2Gen2Arm64,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.CustomKubeBinaryURL = "https://acs-mirror.azureedge.net/kubernetes/v1.24.9/binaries/kubernetes-node-linux-arm64.tar.gz"
				nbc.AgentPoolProfile.VMSize = "Standard_D2pds_V5"
				nbc.IsARM64 = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_D2pds_V5")
			},
		},
	})
}

func Test_AzureLinuxV2_ARM64_Scriptless(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using a AzureLinuxV2 (CgroupV2) VHD on ARM64 architecture can be properly bootstrapped",
		Tags: Tags{
			Scriptless: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV2Gen2Arm64,
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
				config.KubeBinaryConfig.CustomKubeBinaryUrl = "https://acs-mirror.azureedge.net/kubernetes/v1.24.9/binaries/kubernetes-node-linux-arm64.tar.gz"
				config.VmSize = "Standard_D2pds_V5"
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_D2pds_V5")
			},
		},
	})
}

func Test_AzureLinuxV2_ARM64AirGap(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using a AzureLinuxV2 (CgroupV2) VHD on ARM64 architecture can be properly bootstrapped",
		Tags: Tags{
			Airgap: true,
		},
		Config: Config{
			Cluster: ClusterKubenetAirgap,
			VHD:     config.VHDAzureLinuxV2Gen2Arm64,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = "Standard_D2pds_V5"
				nbc.IsARM64 = true

				nbc.OutboundType = datamodel.OutboundTypeBlock
				nbc.ContainerService.Properties.SecurityProfile = &datamodel.SecurityProfile{
					PrivateEgress: &datamodel.PrivateEgress{
						Enabled:                 true,
						ContainerRegistryServer: fmt.Sprintf("%s.azurecr.io", config.PrivateACRName(config.Config.DefaultLocation)),
					},
				}
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_D2pds_V5")
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateDirectoryContent(ctx, s, "/run", []string{"outbound-check-skipped"})
			},
		},
	})
}

func Test_AzureLinuxV2_AzureCNI(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "azurelinuxv2 scenario on a cluster configured with Azure CNI",
		Config: Config{
			Cluster: ClusterAzureNetwork,
			VHD:     config.VHDAzureLinuxV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = string(armcontainerservice.NetworkPluginAzure)
				nbc.AgentPoolProfile.KubernetesConfig.NetworkPlugin = string(armcontainerservice.NetworkPluginAzure)
			},
		},
	})
}

func Test_AzureLinuxV2_ChronyRestarts(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that the chrony service restarts if it is killed",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/etc/systemd/system/chronyd.service.d/10-chrony-restarts.conf", "Restart=always")
				ValidateFileHasContent(ctx, s, "/etc/systemd/system/chronyd.service.d/10-chrony-restarts.conf", "RestartSec=5")
				ServiceCanRestartValidator(ctx, s, "chronyd", 10)
			},
		},
	})
}

func Test_AzureLinuxV2_CustomSysctls(t *testing.T) {
	customSysctls := map[string]string{
		"net.ipv4.ip_local_port_range":       "32768 62535",
		"net.netfilter.nf_conntrack_max":     "2097152",
		"net.netfilter.nf_conntrack_buckets": "524288",
		"net.ipv4.tcp_keepalive_intvl":       "90",
		"net.ipv4.ip_local_reserved_ports":   "",
	}
	customContainerdUlimits := map[string]string{
		"LimitMEMLOCK": "75000",
		"LimitNOFILE":  "1048",
	}
	RunScenario(t, &Scenario{
		Description: "tests that a AzureLinuxV2 (CgroupV2) VHD can be properly bootstrapped when supplied custom node config that contains custom sysctl settings",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				customLinuxConfig := &datamodel.CustomLinuxOSConfig{
					Sysctls: &datamodel.SysctlConfig{
						NetNetfilterNfConntrackMax:     to.Ptr(toolkit.StrToInt32(customSysctls["net.netfilter.nf_conntrack_max"])),
						NetNetfilterNfConntrackBuckets: to.Ptr(toolkit.StrToInt32(customSysctls["net.netfilter.nf_conntrack_buckets"])),
						NetIpv4IpLocalPortRange:        customSysctls["net.ipv4.ip_local_port_range"],
						NetIpv4TcpkeepaliveIntvl:       to.Ptr(toolkit.StrToInt32(customSysctls["net.ipv4.tcp_keepalive_intvl"])),
					},
					UlimitConfig: &datamodel.UlimitConfig{
						MaxLockedMemory: customContainerdUlimits["LimitMEMLOCK"],
						NoFile:          customContainerdUlimits["LimitNOFILE"],
					},
				}
				nbc.AgentPoolProfile.CustomLinuxOSConfig = customLinuxConfig
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateUlimitSettings(ctx, s, customContainerdUlimits)
				ValidateSysctlConfig(ctx, s, customSysctls)
			},
		},
	})
}

// Returns config for the 'gpu' E2E scenario
func Test_AzureLinuxV2_GPU(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a GPU-enabled node using a AzureLinuxV2 (CgroupV2) VHD can be properly bootstrapped",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = "Standard_NC6s_v3"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = false
				nbc.EnableNvidia = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NC6s_v3")
			},
			Validator: func(ctx context.Context, s *Scenario) {
			},
		},
	})
}

func Test_AzureLinuxV2_GPUAzureCNI(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "AzureLinux V2 (CgroupV2) gpu scenario on cluster configured with Azure CNI",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterAzureNetwork,
			VHD:     config.VHDAzureLinuxV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = string(armcontainerservice.NetworkPluginAzure)
				nbc.AgentPoolProfile.KubernetesConfig.NetworkPlugin = string(armcontainerservice.NetworkPluginAzure)
				nbc.AgentPoolProfile.VMSize = "Standard_NC6s_v3"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = false
				nbc.EnableNvidia = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NC6s_v3")
			},
			Validator: func(ctx context.Context, s *Scenario) {
			},
		},
	})
}

func Test_AzureLinuxV2_GPUAzureCNI_Scriptless(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "AzureLinux V2 (CgroupV2) gpu scenario on cluster configured with Azure CNI",
		Tags: Tags{
			Scriptless: true,
			GPU:        true,
		},
		Config: Config{
			Cluster: ClusterAzureNetwork,
			VHD:     config.VHDAzureLinuxV2Gen2,
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
				config.NetworkConfig.NetworkPlugin = aksnodeconfigv1.NetworkPlugin_NETWORK_PLUGIN_AZURE
				config.VmSize = "Standard_NC6s_v3"
				config.GpuConfig.ConfigGpuDriver = true
				config.GpuConfig.GpuDevicePlugin = false
				config.GpuConfig.EnableNvidia = to.Ptr(true)

			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NC6s_v3")
			},
			Validator: func(ctx context.Context, s *Scenario) {
			},
		},
	})
}

func Test_MarinerV2(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using a MarinerV2 VHD can be properly bootstrapped",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDCBLMarinerV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateInstalledPackageVersion(ctx, s, "moby-containerd", getExpectedPackageVersions("containerd", "mariner", "current")[0])
			},
		},
	})
}

func Test_MarinerV2_AirGap(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using a MarinerV2 VHD can be properly bootstrapped",
		Tags: Tags{
			Airgap: true,
		},
		Config: Config{
			Cluster: ClusterKubenetAirgap,
			VHD:     config.VHDCBLMarinerV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.OutboundType = datamodel.OutboundTypeBlock
				nbc.ContainerService.Properties.SecurityProfile = &datamodel.SecurityProfile{
					PrivateEgress: &datamodel.PrivateEgress{
						Enabled:                 true,
						ContainerRegistryServer: fmt.Sprintf("%s.azurecr.io", config.PrivateACRName(config.Config.DefaultLocation)),
					},
				}
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateDirectoryContent(ctx, s, "/run", []string{"outbound-check-skipped"})
			},
		},
	})
}

func Test_MarinerV2_ARM64(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using a MarinerV2 VHD on ARM64 architecture can be properly bootstrapped",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDCBLMarinerV2Gen2Arm64,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.CustomKubeBinaryURL = "https://acs-mirror.azureedge.net/kubernetes/v1.24.9/binaries/kubernetes-node-linux-arm64.tar.gz"
				nbc.AgentPoolProfile.VMSize = "Standard_D2pds_V5"
				nbc.IsARM64 = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_D2pds_V5")
			},
		},
	})
}

func Test_MarinerV2_ARM64AirGap(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using a MarinerV2 VHD on ARM64 architecture can be properly bootstrapped",
		Tags: Tags{
			Airgap: true,
		},
		Config: Config{
			Cluster: ClusterKubenetAirgap,
			VHD:     config.VHDCBLMarinerV2Gen2Arm64,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = "Standard_D2pds_V5"
				nbc.IsARM64 = true

				nbc.OutboundType = datamodel.OutboundTypeBlock
				nbc.ContainerService.Properties.SecurityProfile = &datamodel.SecurityProfile{
					PrivateEgress: &datamodel.PrivateEgress{
						Enabled:                 true,
						ContainerRegistryServer: fmt.Sprintf("%s.azurecr.io", config.PrivateACRName(config.Config.DefaultLocation)),
					},
				}

			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_D2pds_V5")
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateDirectoryContent(ctx, s, "/run", []string{"outbound-check-skipped"})
			},
		},
	})
}

// Merge test case MarinerV2 AzureCNI with MarinerV2 ChronyRestarts
func Test_MarinerV2_AzureCNI_ChronyRestarts(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Test marinerv2 scenario on a cluster configured with Azure CNI and the chrony service restarts if it is killed",
		Config: Config{
			Cluster: ClusterAzureNetwork,
			VHD:     config.VHDCBLMarinerV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = string(armcontainerservice.NetworkPluginAzure)
				nbc.AgentPoolProfile.KubernetesConfig.NetworkPlugin = string(armcontainerservice.NetworkPluginAzure)
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ServiceCanRestartValidator(ctx, s, "chronyd", 10)
				ValidateFileHasContent(ctx, s, "/etc/systemd/system/chronyd.service.d/10-chrony-restarts.conf", "Restart=always")
				ValidateFileHasContent(ctx, s, "/etc/systemd/system/chronyd.service.d/10-chrony-restarts.conf", "RestartSec=5")
			},
		},
	})
}

// Merge scriptless test case MarinerV2 AzureCNI with MarinerV2 ChronyRestarts
func Test_MarinerV2_AzureCNI_ChronyRestarts_Scriptless(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Test marinerv2 scenario on a cluster configured with Azure CNI and the chrony service restarts if it is killed",
		Tags: Tags{
			Scriptless: true,
		},
		Config: Config{
			Cluster: ClusterAzureNetwork,
			VHD:     config.VHDCBLMarinerV2Gen2,
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
				config.NetworkConfig.NetworkPlugin = aksnodeconfigv1.NetworkPlugin_NETWORK_PLUGIN_AZURE
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ServiceCanRestartValidator(ctx, s, "chronyd", 10)
				ValidateFileHasContent(ctx, s, "/etc/systemd/system/chronyd.service.d/10-chrony-restarts.conf", "Restart=always")
				ValidateFileHasContent(ctx, s, "/etc/systemd/system/chronyd.service.d/10-chrony-restarts.conf", "RestartSec=5")
			},
		},
	})
}

func Test_MarinerV2_CustomSysctls(t *testing.T) {
	customSysctls := map[string]string{
		"net.ipv4.ip_local_port_range":       "32768 62535",
		"net.netfilter.nf_conntrack_max":     "2097152",
		"net.netfilter.nf_conntrack_buckets": "524288",
		"net.ipv4.tcp_keepalive_intvl":       "90",
		"net.ipv4.ip_local_reserved_ports":   "",
	}
	customContainerdUlimits := map[string]string{
		"LimitMEMLOCK": "75000",
		"LimitNOFILE":  "1048",
	}
	RunScenario(t, &Scenario{
		Description: "tests that a MarinerV2 VHD can be properly bootstrapped when supplied custom node config that contains custom sysctl settings",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDCBLMarinerV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				customLinuxConfig := &datamodel.CustomLinuxOSConfig{
					Sysctls: &datamodel.SysctlConfig{
						NetNetfilterNfConntrackMax:     to.Ptr(toolkit.StrToInt32(customSysctls["net.netfilter.nf_conntrack_max"])),
						NetNetfilterNfConntrackBuckets: to.Ptr(toolkit.StrToInt32(customSysctls["net.netfilter.nf_conntrack_buckets"])),
						NetIpv4IpLocalPortRange:        customSysctls["net.ipv4.ip_local_port_range"],
						NetIpv4TcpkeepaliveIntvl:       to.Ptr(toolkit.StrToInt32(customSysctls["net.ipv4.tcp_keepalive_intvl"])),
					},
					UlimitConfig: &datamodel.UlimitConfig{
						MaxLockedMemory: customContainerdUlimits["LimitMEMLOCK"],
						NoFile:          customContainerdUlimits["LimitNOFILE"],
					},
				}
				nbc.AgentPoolProfile.CustomLinuxOSConfig = customLinuxConfig
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateUlimitSettings(ctx, s, customContainerdUlimits)
				ValidateSysctlConfig(ctx, s, customSysctls)
			},
		},
	})
}

func Test_MarinerV2_GPU(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a GPU-enabled node using a MarinerV2 VHD can be properly bootstrapped",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDCBLMarinerV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = "Standard_NC6s_v3"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = false
				nbc.EnableNvidia = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NC6s_v3")
			},
			Validator: func(ctx context.Context, s *Scenario) {
			},
		},
	})
}

func Test_MarinerV2_GPUAzureCNI(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "MarinerV2 gpu scenario on cluster configured with Azure CNI",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterAzureNetwork,
			VHD:     config.VHDCBLMarinerV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = string(armcontainerservice.NetworkPluginAzure)
				nbc.AgentPoolProfile.KubernetesConfig.NetworkPlugin = string(armcontainerservice.NetworkPluginAzure)
				nbc.AgentPoolProfile.VMSize = "Standard_NC6s_v3"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = false
				nbc.EnableNvidia = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NC6s_v3")
			},
			Validator: func(ctx context.Context, s *Scenario) {
			},
		},
	})
}

// Returns config for the 'base' E2E scenario

func Test_Ubuntu2204_Scriptless(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "tests that a new ubuntu 2204 node using self contained installer can be properly bootstrapped",
		Tags: Tags{
			Scriptless: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/var/log/azure/aks-node-controller.log", "aks-node-controller finished successfully")
			},
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
			},
		}})
}

func Test_Ubuntu2404_Scriptless(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "testing that a new ubuntu 2404 node using self contained installer can be properly bootstrapped",
		Tags: Tags{
			Scriptless: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2404Gen2Containerd,
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/var/log/azure/aks-node-controller.log", "aks-node-controller finished successfully")
			},
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
			},
		}})
}

// Returns config for the 'gpu' E2E scenario
func Test_Ubuntu2204(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using the Ubuntu 2204 VHD can be properly bootstrapped",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				// Check that we don't leak these secrets if they're
				// set (which they mostly aren't in these scenarios).
				nbc.ContainerService.Properties.CertificateProfile.ClientPrivateKey = "client cert private key"
				nbc.ContainerService.Properties.ServicePrincipalProfile.Secret = "SP secret"
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateInstalledPackageVersion(ctx, s, "moby-containerd", getExpectedPackageVersions("containerd", "ubuntu", "r2204")[0])
				ValidateInstalledPackageVersion(ctx, s, "moby-runc", getExpectedPackageVersions("runc", "ubuntu", "r2204")[0])
				ValidateSSHServiceEnabled(ctx, s)
			},
		}})
}

func Test_Ubuntu2204_AirGap(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using the Ubuntu 2204 VHD and is airgap can be properly bootstrapped",
		Tags: Tags{
			Airgap: true,
		},
		Config: Config{
			Cluster: ClusterKubenetAirgap,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.OutboundType = datamodel.OutboundTypeBlock
				nbc.ContainerService.Properties.SecurityProfile = &datamodel.SecurityProfile{
					PrivateEgress: &datamodel.PrivateEgress{
						Enabled:                 true,
						ContainerRegistryServer: fmt.Sprintf("%s.azurecr.io", config.PrivateACRName(config.Config.DefaultLocation)),
					},
				}
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateDirectoryContent(ctx, s, "/run", []string{"outbound-check-skipped"})
			},
		},
	})
}

// TODO: refactor NonAnonymous tests to use the same cluster as Anonymous airgap
// or deprecate anonymous ACR airgap tests once it is unsupported
func Test_Ubuntu2204_AirGap_NonAnonymousACR(t *testing.T) {
	location := config.Config.DefaultLocation

	ctx := newTestCtx(t, location)
	identity, err := config.Azure.UserAssignedIdentities.Get(ctx, config.ResourceGroupName(location), config.VMIdentityName, nil)
	if err != nil {
		t.Fatalf("failed to get identity: %v", err)
	}

	RunScenario(t, &Scenario{
		Description: "Tests that a node using the Ubuntu 2204 VHD and is airgap can be properly bootstrapped",
		Tags: Tags{
			Airgap:          true,
			NonAnonymousACR: true,
		},
		Config: Config{
			Cluster: ClusterKubenetAirgapNonAnon,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.TenantID = *identity.Properties.TenantID
				nbc.UserAssignedIdentityClientID = *identity.Properties.ClientID

				nbc.OutboundType = datamodel.OutboundTypeBlock
				nbc.ContainerService.Properties.SecurityProfile = &datamodel.SecurityProfile{
					PrivateEgress: &datamodel.PrivateEgress{
						Enabled:                 true,
						ContainerRegistryServer: fmt.Sprintf("%s.azurecr.io", config.PrivateACRNameNotAnon(config.Config.DefaultLocation)),
					},
				}
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateDirectoryContent(ctx, s, "/run", []string{"outbound-check-skipped"})
			},
		},
	})
}

func Test_Ubuntu2204Gen2_ContainerdAirgappedK8sNotCached(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using the Ubuntu 2204 VHD without k8s binary and is airgap can be properly bootstrapped",
		Tags: Tags{
			Airgap: true,
		},
		Config: Config{
			Cluster: ClusterKubenetAirgap,
			VHD:     config.VHDUbuntu2204Gen2ContainerdAirgappedK8sNotCached,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.OutboundType = datamodel.OutboundTypeBlock
				nbc.ContainerService.Properties.SecurityProfile = &datamodel.SecurityProfile{
					PrivateEgress: &datamodel.PrivateEgress{
						Enabled:                 true,
						ContainerRegistryServer: fmt.Sprintf("%s.azurecr.io", config.PrivateACRName(config.Config.DefaultLocation)),
					},
				}
				nbc.AgentPoolProfile.LocalDNSProfile = nil
				// intentionally using private acr url to get kube binaries
				nbc.AgentPoolProfile.KubernetesConfig.CustomKubeBinaryURL = fmt.Sprintf(
					"%s.azurecr.io/oss/binaries/kubernetes/kubernetes-node:v%s-linux-amd64",
					config.PrivateACRName(config.Config.DefaultLocation),
					nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion)
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateDirectoryContent(ctx, s, "/run", []string{"outbound-check-skipped"})
			},
		}})
}

func Test_Ubuntu2204ARM64(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that an Ubuntu 2204 Node using ARM64 architecture can be properly bootstrapped",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Arm64Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				// This needs to be set based on current CSE implementation...
				nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.CustomKubeBinaryURL = "https://acs-mirror.azureedge.net/kubernetes/v1.24.9/binaries/kubernetes-node-linux-arm64.tar.gz"
				nbc.AgentPoolProfile.VMSize = "Standard_D2pds_V5"
				nbc.IsARM64 = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_D2pds_V5")
			},
		}})
}

func Test_Ubuntu2204_ArtifactStreaming(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "tests that a new ubuntu 2204 node using artifact streaming can be properly bootstrapepd",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.EnableArtifactStreaming = true
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateNonEmptyDirectory(ctx, s, "/etc/overlaybd")
				ValidateSystemdUnitIsRunning(ctx, s, "overlaybd-snapshotter.service")
				ValidateSystemdUnitIsRunning(ctx, s, "overlaybd-tcmu.service")
				ValidateSystemdUnitIsRunning(ctx, s, "acr-mirror.service")
				ValidateSystemdUnitIsRunning(ctx, s, "acr-nodemon.service")
				ValidateSystemdUnitIsRunning(ctx, s, "containerd.service")
			},
		}})
}

func Test_Ubuntu2204_ArtifactStreaming_Scriptless(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "tests that a new ubuntu 2204 node using artifact streaming can be properly bootstrapepd",
		Tags: Tags{
			Scriptless: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
				config.EnableArtifactStreaming = true
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateNonEmptyDirectory(ctx, s, "/etc/overlaybd")
				ValidateSystemdUnitIsRunning(ctx, s, "overlaybd-snapshotter.service")
				ValidateSystemdUnitIsRunning(ctx, s, "overlaybd-tcmu.service")
				ValidateSystemdUnitIsRunning(ctx, s, "acr-mirror.service")
				ValidateSystemdUnitIsRunning(ctx, s, "acr-nodemon.service")
				ValidateSystemdUnitIsRunning(ctx, s, "containerd.service")
			},
		}})
}

func Test_Ubuntu2204_ChronyRestarts_Taints_And_Tolerations(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that the chrony service restarts if it is killed. Also tests taints and tolerations",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.KubeletConfig["--register-with-taints"] = "testkey1=value1:NoSchedule,testkey2=value2:NoSchedule"
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/etc/systemd/system/chronyd.service.d/10-chrony-restarts.conf", "Restart=always")
				ValidateFileHasContent(ctx, s, "/etc/systemd/system/chronyd.service.d/10-chrony-restarts.conf", "RestartSec=5")
				ServiceCanRestartValidator(ctx, s, "chronyd", 10)
				ValidateTaints(ctx, s, s.Runtime.NBC.KubeletConfig["--register-with-taints"])
			},
		}})
}

func Test_Ubuntu2204_ChronyRestarts_Taints_And_Tolerations_Scriptless(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that the chrony service restarts if it is killed. Also tests taints and tolerations",
		Tags: Tags{
			Scriptless: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
				config.KubeletConfig.KubeletFlags["--register-with-taints"] = "testkey1=value1:NoSchedule,testkey2=value2:NoSchedule"
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/etc/systemd/system/chronyd.service.d/10-chrony-restarts.conf", "Restart=always")
				ValidateFileHasContent(ctx, s, "/etc/systemd/system/chronyd.service.d/10-chrony-restarts.conf", "RestartSec=5")
				ServiceCanRestartValidator(ctx, s, "chronyd", 10)
				ValidateTaints(ctx, s, s.Runtime.AKSNodeConfig.KubeletConfig.KubeletFlags["--register-with-taints"])
			},
		}})
}

func Test_AzureLinuxV2_CustomCATrust(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using the Azure Linux 2204 VHD can be properly bootstrapped and custom CA was correctly added",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.CustomCATrustConfig = &datamodel.CustomCATrustConfig{
					CustomCATrustCerts: []string{
						encodedTestCert,
					},
				}
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateNonEmptyDirectory(ctx, s, "/usr/share/pki/ca-trust-source/anchors")
			},
		}})
}

func Test_Ubuntu2204_CustomCATrust(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using the Ubuntu 2204 VHD can be properly bootstrapped and custom CA was correctly added",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.CustomCATrustConfig = &datamodel.CustomCATrustConfig{
					CustomCATrustCerts: []string{
						encodedTestCert,
					},
				}
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateNonEmptyDirectory(ctx, s, "/usr/local/share/ca-certificates/certs")
			},
		}})
}

func Test_Ubuntu2204_CustomCATrust_Scriptless(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using the Ubuntu 2204 VHD can be properly bootstrapped and custom CA was correctly added",
		Tags: Tags{
			Scriptless: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
				config.CustomCaCerts = []string{encodedTestCert}
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateNonEmptyDirectory(ctx, s, "/usr/local/share/ca-certificates/certs")
			},
		},
	})
}

func Test_Ubuntu2204_CustomSysctls(t *testing.T) {
	customSysctls := map[string]string{
		"net.ipv4.ip_local_port_range":       "32768 65535",
		"net.netfilter.nf_conntrack_max":     "2097152",
		"net.netfilter.nf_conntrack_buckets": "524288",
		"net.ipv4.tcp_keepalive_intvl":       "90",
		"net.ipv4.ip_local_reserved_ports":   "65330",
	}
	customContainerdUlimits := map[string]string{
		"LimitMEMLOCK": "75000",
		"LimitNOFILE":  "1048",
	}
	RunScenario(t, &Scenario{
		Description: "tests that an ubuntu 2204 VHD can be properly bootstrapped when supplied custom node config that contains custom sysctl settings",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				customLinuxConfig := &datamodel.CustomLinuxOSConfig{
					Sysctls: &datamodel.SysctlConfig{
						NetNetfilterNfConntrackMax:     to.Ptr(toolkit.StrToInt32(customSysctls["net.netfilter.nf_conntrack_max"])),
						NetNetfilterNfConntrackBuckets: to.Ptr(toolkit.StrToInt32(customSysctls["net.netfilter.nf_conntrack_buckets"])),
						NetIpv4IpLocalPortRange:        customSysctls["net.ipv4.ip_local_port_range"],
						NetIpv4TcpkeepaliveIntvl:       to.Ptr(toolkit.StrToInt32(customSysctls["net.ipv4.tcp_keepalive_intvl"])),
					},
					UlimitConfig: &datamodel.UlimitConfig{
						MaxLockedMemory: "75000",
						NoFile:          "1048",
					},
				}
				nbc.AgentPoolProfile.CustomLinuxOSConfig = customLinuxConfig
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateUlimitSettings(ctx, s, customContainerdUlimits)
				ValidateSysctlConfig(ctx, s, customSysctls)
			},
		},
	})
}

func Test_Ubuntu2204_CustomSysctls_Scriptless(t *testing.T) {
	customSysctls := map[string]string{
		"net.ipv4.ip_local_port_range":       "32768 65535",
		"net.netfilter.nf_conntrack_max":     "2097152",
		"net.netfilter.nf_conntrack_buckets": "524288",
		"net.ipv4.tcp_keepalive_intvl":       "90",
		"net.ipv4.ip_local_reserved_ports":   "65330",
	}
	customContainerdUlimits := map[string]string{
		"LimitMEMLOCK": "75000",
		"LimitNOFILE":  "1048",
	}
	RunScenario(t, &Scenario{
		Description: "tests that an ubuntu 2204 VHD can be properly bootstrapped when supplied custom node config that contains custom sysctl settings",
		Tags: Tags{
			Scriptless: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
				customLinuxOsConfig := &aksnodeconfigv1.CustomLinuxOsConfig{
					SysctlConfig: &aksnodeconfigv1.SysctlConfig{
						NetNetfilterNfConntrackMax:     to.Ptr(toolkit.StrToInt32(customSysctls["net.netfilter.nf_conntrack_max"])),
						NetNetfilterNfConntrackBuckets: to.Ptr(toolkit.StrToInt32(customSysctls["net.netfilter.nf_conntrack_buckets"])),
						NetIpv4IpLocalPortRange:        to.Ptr(customSysctls["net.ipv4.ip_local_port_range"]),
						NetIpv4TcpkeepaliveIntvl:       to.Ptr(toolkit.StrToInt32(customSysctls["net.ipv4.tcp_keepalive_intvl"])),
					},
					UlimitConfig: &aksnodeconfigv1.UlimitConfig{
						MaxLockedMemory: to.Ptr(customContainerdUlimits["LimitMEMLOCK"]),
						NoFile:          to.Ptr(customContainerdUlimits["LimitNOFILE"]),
					},
				}
				config.CustomLinuxOsConfig = customLinuxOsConfig
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateUlimitSettings(ctx, s, customContainerdUlimits)
				ValidateSysctlConfig(ctx, s, customSysctls)
			},
		}})
}

func Test_Ubuntu2204_GPUNC(t *testing.T) {
	runScenarioUbuntu2204GPU(t, "Standard_NC6s_v3")
}

func Test_Ubuntu2204_GPUA100(t *testing.T) {
	runScenarioUbuntu2204GPU(t, "Standard_NC24ads_A100_v4")
}

func Test_Ubuntu2204_GPUA10(t *testing.T) {
	runScenarioUbuntuGRID(t, "Standard_NV6ads_A10_v5")
}

// Returns config for the 'gpu' E2E scenario
func runScenarioUbuntu2204GPU(t *testing.T, vmSize string) {
	RunScenario(t, &Scenario{
		Description: fmt.Sprintf("Tests that a GPU-enabled node with VM size %s using an Ubuntu 2204 VHD can be properly bootstrapped", vmSize),
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = vmSize
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = false
				nbc.EnableNvidia = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr(vmSize)
			},
			Validator: func(ctx context.Context, s *Scenario) {
				// Ensure nvidia-modprobe install does not restart kubelet and temporarily cause node to be unschedulable
				ValidateNvidiaModProbeInstalled(ctx, s)
				ValidateKubeletHasNotStopped(ctx, s)
				ValidateServicesDoNotRestartKubelet(ctx, s)
			},
		}})
}

func runScenarioUbuntuGRID(t *testing.T, vmSize string) {
	RunScenario(t, &Scenario{
		Description: fmt.Sprintf("Tests that a GPU-enabled node with VM size %s using an Ubuntu 2204 VHD can be properly bootstrapped, and that the GRID license is valid", vmSize),
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = vmSize
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = false
				nbc.EnableNvidia = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr(vmSize)
			},
			Validator: func(ctx context.Context, s *Scenario) {
				// Ensure nvidia-modprobe install does not restart kubelet and temporarily cause node to be unschedulable
				ValidateNvidiaModProbeInstalled(ctx, s)
				ValidateNvidiaGRIDLicenseValid(ctx, s)
				ValidateKubeletHasNotStopped(ctx, s)
				ValidateServicesDoNotRestartKubelet(ctx, s)
				ValidateNvidiaPersistencedRunning(ctx, s)
			},
		}})
}

func Test_Ubuntu2204_GPUA10_Scriptless(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests scriptless installer that a GPU-enabled node using the Ubuntu 2204 VHD with grid driver can be properly bootstrapped",
		Tags: Tags{
			Scriptless: true,
			GPU:        true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NV6ads_A10_v5")
			},
			Validator: func(ctx context.Context, s *Scenario) {
				// Ensure nvidia-modprobe install does not restart kubelet and temporarily cause node to be unschedulable
				ValidateNvidiaModProbeInstalled(ctx, s)
				ValidateKubeletHasNotStopped(ctx, s)
				ValidateServicesDoNotRestartKubelet(ctx, s)
			},
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
				config.VmSize = "Standard_NV6ads_A10_v5"
				config.GpuConfig.ConfigGpuDriver = true
				config.GpuConfig.GpuDevicePlugin = false
				config.GpuConfig.EnableNvidia = to.Ptr(true)
			},
		}})
}

func Test_Ubuntu2204_GPUGridDriver(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a GPU-enabled node using the Ubuntu 2204 VHD with grid driver can be properly bootstrapped",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = "Standard_NV6ads_A10_v5"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = false
				nbc.EnableNvidia = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NV6ads_A10_v5")
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateNvidiaModProbeInstalled(ctx, s)
				ValidateKubeletHasNotStopped(ctx, s)
				ValidateNvidiaSMIInstalled(ctx, s)
			},
		}})
}

func Test_Ubuntu2204_GPUNoDriver(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a GPU-enabled node using the Ubuntu 2204 VHD opting for skipping gpu driver installation can be properly bootstrapped",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = "Standard_NC6s_v3"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = false
				nbc.EnableNvidia = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.Tags = map[string]*string{
					// deliberately case mismatched to agentbaker logic to check case insensitivity
					"SkipGPUDriverInstall": to.Ptr("true"),
				}
				vmss.SKU.Name = to.Ptr("Standard_NC6s_v3")
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateNvidiaSMINotInstalled(ctx, s)
			},
		}})
}

func Test_Ubuntu2204_GPUNoDriver_Scriptless(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a GPU-enabled node using the Ubuntu 2204 VHD opting for skipping gpu driver installation can be properly bootstrapped",
		Tags: Tags{
			Scriptless: true,
			GPU:        true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
				config.VmSize = "Standard_NC6s_v3"
				config.GpuConfig.ConfigGpuDriver = true
				config.GpuConfig.GpuDevicePlugin = false
				config.GpuConfig.EnableNvidia = to.Ptr(true)
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				// this vmss tag is needed since there is a logic in cse_main.sh otherwise the test will fail
				vmss.Tags = map[string]*string{
					// deliberately case mismatched to agentbaker logic to check case insensitivity
					"SkipGPUDriverInstall": to.Ptr("true"),
				}
				vmss.SKU.Name = to.Ptr("Standard_NC6s_v3")
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateNvidiaSMINotInstalled(ctx, s)
			},
		}})
}

func Test_Ubuntu2204_PrivateKubePkg(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using the Ubuntu 2204 VHD that was built with private kube packages can be properly bootstrapped with the specified kube version",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2ContainerdPrivateKubePkg,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = "1.25.6"
				nbc.K8sComponents.LinuxPrivatePackageURL = "https://privatekube.blob.core.windows.net/kubernetes/v1.25.6-hotfix.20230612/binaries/v1.25.6-hotfix.20230612.tar.gz"
				nbc.AgentPoolProfile.LocalDNSProfile = nil
			},
		}})
}

// These tests were created to verify that the apt-get call in downloadContainerdFromVersion is not executed.
// The code path is not hit in either of these tests. In the future, testing with some kind of firewall to ensure no egress
// calls are made would be beneficial for airgap testing.

// Combine old e2e tests for scenario Ubuntu2204_ContainerdURL and Ubuntu2204_IMDSRestrictionFilterTable
func Test_Ubuntu2204_ContainerdURL_IMDSRestrictionFilterTable(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: `tests that a node using the Ubuntu 2204 VHD with the ContainerdPackageURL override bootstraps with the provided URL and not the components.json containerd version,
		              tests that the imds restriction filter table is properly set`,
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerdPackageURL = "https://packages.microsoft.com/ubuntu/22.04/prod/pool/main/m/moby-containerd/moby-containerd_1.6.9+azure-ubuntu22.04u1_amd64.deb"
				nbc.EnableIMDSRestriction = true
				nbc.InsertIMDSRestrictionRuleToMangleTable = false
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateInstalledPackageVersion(ctx, s, "containerd", "1.6.9")
			},
		}})
}

// Combine e2e scriptless tests for scenario Ubuntu2204_ContainerdURL and Ubuntu2204_IMDSRestrictionFilterTable
func Test_Ubuntu2204_ContainerdURL_IMDSRestrictionFilterTable_Scriptless(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: `tests that a node using the Ubuntu 2204 VHD with the ContainerdPackageURL override the provided URL and not the components.json containerd version,
		              tests that the imds restriction filter table is properly set`,
		Tags: Tags{
			Scriptless: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
				config.ContainerdConfig.ContainerdPackageUrl = "https://packages.microsoft.com/ubuntu/22.04/prod/pool/main/m/moby-containerd/moby-containerd_1.6.9+azure-ubuntu22.04u1_amd64.deb"
				config.ImdsRestrictionConfig = &aksnodeconfigv1.ImdsRestrictionConfig{
					EnableImdsRestriction:                  true,
					InsertImdsRestrictionRuleToMangleTable: false,
				}
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateInstalledPackageVersion(ctx, s, "containerd", "1.6.9")
			},
		}})
}

func Test_Ubuntu2204_ContainerdHasCurrentVersion(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "tests that a node using an Ubuntu2204 VHD and the ContainerdVersion override bootstraps with the correct components.json containerd version and ignores the override",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateInstalledPackageVersion(ctx, s, "moby-containerd", getExpectedPackageVersions("containerd", "ubuntu", "r2204")[0])
			},
		}})
}

func Test_AzureLinux_Skip_Binary_Cleanup(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "tests that an AzureLinux node will skip binary cleanup and can be properly bootstrapped",
		Config: Config{
			Cluster:                ClusterKubenet,
			VHD:                    config.VHDAzureLinuxV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				if vmss.Tags == nil {
					vmss.Tags = map[string]*string{}
				}
				vmss.Tags["SkipBinaryCleanup"] = to.Ptr("true")
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateMultipleKubeProxyVersionsExist(ctx, s)
			},
		}})
}

func Test_Ubuntu2204_DisableKubeletServingCertificateRotationWithTags(t *testing.T) {
	RunScenario(t, &Scenario{
		Tags: Tags{
			ServerTLSBootstrapping: true,
		},
		Description: "tests that a node on ubuntu 2204 bootstrapped with kubelet serving certificate rotation enabled will disable certificate rotation due to nodepool tags",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				if nbc.KubeletConfig == nil {
					nbc.KubeletConfig = map[string]string{}
				}
				nbc.KubeletConfig["--rotate-server-certificates"] = "true"
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				if vmss.Tags == nil {
					vmss.Tags = map[string]*string{}
				}
				vmss.Tags["aks-disable-kubelet-serving-certificate-rotation"] = to.Ptr("true")
			},

			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/etc/default/kubelet", "--tls-cert-file=/etc/kubernetes/certs/kubeletserver.crt")
				ValidateFileHasContent(ctx, s, "/etc/default/kubelet", "--tls-private-key-file=/etc/kubernetes/certs/kubeletserver.key")
				ValidateFileExcludesContent(ctx, s, "/etc/default/kubelet", "--rotate-server-certificates=true")
				ValidateFileExcludesContent(ctx, s, "/etc/default/kubelet", "kubernetes.azure.com/kubelet-serving-ca=cluster")
				ValidateDirectoryContent(ctx, s, "/etc/kubernetes/certs", []string{"kubeletserver.crt", "kubeletserver.key"})
			},
		}})
}

func Test_Ubuntu2204_DisableKubeletServingCertificateRotationWithTags_CustomKubeletConfig(t *testing.T) {
	RunScenario(t, &Scenario{
		Tags: Tags{
			ServerTLSBootstrapping: true,
		},
		Description: "tests that a node on ubuntu 2204 bootstrapped with custom kubelet config and kubelet serving certificate rotation enabled will disable certificate rotation due to nodepool tags",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {

				// to force kubelet config file
				customKubeletConfig := &datamodel.CustomKubeletConfig{
					FailSwapOn:           to.Ptr(true),
					AllowedUnsafeSysctls: &[]string{"kernel.msg*", "net.ipv4.route.min_pmtu"},
				}
				nbc.AgentPoolProfile.CustomKubeletConfig = customKubeletConfig

				if nbc.KubeletConfig == nil {
					nbc.KubeletConfig = map[string]string{}
				}
				nbc.KubeletConfig["--rotate-server-certificates"] = "true"
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				if vmss.Tags == nil {
					vmss.Tags = map[string]*string{}
				}
				vmss.Tags["aks-disable-kubelet-serving-certificate-rotation"] = to.Ptr("true")
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/etc/default/kubeletconfig.json", "\"tlsCertFile\": \"/etc/kubernetes/certs/kubeletserver.crt\"")
				ValidateFileHasContent(ctx, s, "/etc/default/kubeletconfig.json", "\"tlsPrivateKeyFile\": \"/etc/kubernetes/certs/kubeletserver.key\"")
				ValidateFileExcludesContent(ctx, s, "/etc/default/kubelet", "--rotate-server-certificates=true")
				ValidateFileExcludesContent(ctx, s, "/etc/default/kubelet", "kubernetes.azure.com/kubelet-serving-ca=cluster")
				ValidateFileExcludesContent(ctx, s, "/etc/default/kubeletconfig.json", "\"serverTLSBootstrap\": true")
				ValidateDirectoryContent(ctx, s, "/etc/kubernetes/certs", []string{"kubeletserver.crt", "kubeletserver.key"})
			},
		}})
}

func Test_Ubuntu2204_DisableKubeletServingCertificateRotationWithTags_CustomKubeletConfig_Scriptless(t *testing.T) {
	RunScenario(t, &Scenario{
		Tags: Tags{
			ServerTLSBootstrapping: true,
			Scriptless:             true,
		},
		Description: "tests that a node on ubuntu 2204 bootstrapped with custom kubelet config and kubelet serving certificate rotation enabled will disable certificate rotation due to nodepool tags",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
				config.KubeletConfig.EnableKubeletConfigFile = true
				config.KubeletConfig.KubeletConfigFileConfig.FailSwapOn = to.Ptr(true)
				config.KubeletConfig.KubeletConfigFileConfig.AllowedUnsafeSysctls = []string{"kernel.msg*", "net.ipv4.route.min_pmtu"}
				config.KubeletConfig.KubeletConfigFileConfig.ServerTlsBootstrap = true
				config.KubeletConfig.KubeletConfigFileConfig.FeatureGates = map[string]bool{"RotateKubeletServerCertificate": true}
				config.EnableUnattendedUpgrade = false
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				if vmss.Tags == nil {
					vmss.Tags = map[string]*string{}
				}
				vmss.Tags["aks-disable-kubelet-serving-certificate-rotation"] = to.Ptr("true")
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/etc/default/kubeletconfig.json", "\"tlsCertFile\": \"/etc/kubernetes/certs/kubeletserver.crt\"")
				ValidateFileHasContent(ctx, s, "/etc/default/kubeletconfig.json", "\"tlsPrivateKeyFile\": \"/etc/kubernetes/certs/kubeletserver.key\"")
				ValidateFileExcludesContent(ctx, s, "/etc/default/kubelet", "--rotate-server-certificates=true")
				ValidateFileExcludesContent(ctx, s, "/etc/default/kubelet", "kubernetes.azure.com/kubelet-serving-ca=cluster")
				ValidateFileExcludesContent(ctx, s, "/etc/default/kubeletconfig.json", "\"serverTLSBootstrap\": true")
				ValidateDirectoryContent(ctx, s, "/etc/kubernetes/certs", []string{"kubeletserver.crt", "kubeletserver.key"})
			},
		}})
}

func Test_Ubuntu2204_DisableKubeletServingCertificateRotationWithTags_AlreadyDisabled(t *testing.T) {
	RunScenario(t, &Scenario{
		Tags: Tags{
			ServerTLSBootstrapping: true,
		},
		Description: "tests that a node on ubuntu 2204 bootstrapped with kubelet serving certificate rotation disabled will disable certificate rotation regardless of nodepool tags",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				if vmss.Tags == nil {
					vmss.Tags = map[string]*string{}
				}
				vmss.Tags["aks-disable-kubelet-serving-certificate-rotation"] = to.Ptr("true")
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/etc/default/kubelet", "--tls-cert-file=/etc/kubernetes/certs/kubeletserver.crt")
				ValidateFileHasContent(ctx, s, "/etc/default/kubelet", "--tls-private-key-file=/etc/kubernetes/certs/kubeletserver.key")
				ValidateFileExcludesContent(ctx, s, "/etc/default/kubelet", "--rotate-server-certificates=true")
				ValidateFileExcludesContent(ctx, s, "/etc/default/kubelet", "kubernetes.azure.com/kubelet-serving-ca=cluster")
				ValidateDirectoryContent(ctx, s, "/etc/kubernetes/certs", []string{"kubeletserver.crt", "kubeletserver.key"})
			},
		}})
}

func Test_Ubuntu2204_DisableKubeletServingCertificateRotationWithTags_AlreadyDisabled_CustomKubeletConfig(t *testing.T) {
	RunScenario(t, &Scenario{
		Tags: Tags{
			ServerTLSBootstrapping: true,
		},
		Description: "tests that a node on ubuntu 2204 bootstrapped with kubelet serving certificate rotation disabled and custom kubelet config will disable certificate rotation regardless of nodepool tags",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {

				// to force kubelet config file
				customKubeletConfig := &datamodel.CustomKubeletConfig{
					FailSwapOn:           to.Ptr(true),
					AllowedUnsafeSysctls: &[]string{"kernel.msg*", "net.ipv4.route.min_pmtu"},
				}
				nbc.AgentPoolProfile.CustomKubeletConfig = customKubeletConfig
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				if vmss.Tags == nil {
					vmss.Tags = map[string]*string{}
				}
				vmss.Tags["aks-disable-kubelet-serving-certificate-rotation"] = to.Ptr("true")
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/etc/default/kubeletconfig.json", "\"tlsCertFile\": \"/etc/kubernetes/certs/kubeletserver.crt\"")
				ValidateFileHasContent(ctx, s, "/etc/default/kubeletconfig.json", "\"tlsPrivateKeyFile\": \"/etc/kubernetes/certs/kubeletserver.key\"")
				ValidateFileExcludesContent(ctx, s, "/etc/default/kubelet", "--rotate-server-certificates=true")
				ValidateFileExcludesContent(ctx, s, "/etc/default/kubelet", "kubernetes.azure.com/kubelet-serving-ca=cluster")
				ValidateFileExcludesContent(ctx, s, "/etc/default/kubeletconfig.json", "\"serverTLSBootstrap\": true")
				ValidateDirectoryContent(ctx, s, "/etc/kubernetes/certs", []string{"kubeletserver.crt", "kubeletserver.key"})
			},
		}})
}

func Test_Ubuntu2204_MessageOfTheDay(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "tests that a node on ubuntu 2204 bootstrapped and message of the day is properly added to the node",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.MessageOfTheDay = "Zm9vYmFyDQo=" // base64 for foobar
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/etc/motd", "foobar")
				ValidateFileHasContent(ctx, s, "/etc/update-motd.d/99-aks-custom-motd", "cat /etc/motd")
			},
		}})
}

func Test_AzureLinuxV2_MessageOfTheDay(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using a AzureLinuxV2 can be bootstrapped and message of the day is added to the node",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.MessageOfTheDay = "Zm9vYmFyDQo=" // base64 for foobar
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/etc/motd", "foobar")
				ValidateFileHasContent(ctx, s, "/etc/dnf/automatic.conf", "emit_via = stdio")
			},
		}})
}

func Test_AzureLinuxV2_MessageOfTheDay_Scriptless(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using a AzureLinuxV2 can be bootstrapped and message of the day is added to the node",
		Tags: Tags{
			Scriptless: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV2Gen2,
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
				config.MessageOfTheDay = "Zm9vYmFyDQo=" // base64 for foobar
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/etc/motd", "foobar")
				ValidateFileHasContent(ctx, s, "/etc/dnf/automatic.conf", "emit_via = stdio")
			},
		}})
}

func Test_Ubuntu2204_KubeletCustomConfig(t *testing.T) {
	RunScenario(t, &Scenario{
		Tags: Tags{
			KubeletCustomConfig: true,
		},
		Description: "tests that a node on ubuntu 2204 bootstrapped with kubelet custom config for seccomp set to non default values",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.AgentPoolProfile.Distro = "aks-ubuntu-containerd-22.04-gen2"
				customKubeletConfig := &datamodel.CustomKubeletConfig{
					SeccompDefault: to.Ptr(true),
				}
				nbc.AgentPoolProfile.CustomKubeletConfig = customKubeletConfig
				nbc.ContainerService.Properties.AgentPoolProfiles[0].CustomKubeletConfig = customKubeletConfig
			},
			Validator: func(ctx context.Context, s *Scenario) {
				kubeletConfigFilePath := "/etc/default/kubeletconfig.json"
				ValidateFileHasContent(ctx, s, kubeletConfigFilePath, `"seccompDefault": true`)
				ValidateKubeletHasFlags(ctx, s, kubeletConfigFilePath)
			},
		}})
}

func Test_AzureLinuxV2_KubeletCustomConfig(t *testing.T) {
	RunScenario(t, &Scenario{
		Tags: Tags{
			KubeletCustomConfig: true,
		},
		Description: "tests that a node on azure linux v2 bootstrapped with kubelet custom config for seccomp set to non default values",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-azurelinux-v2-gen2"
				nbc.AgentPoolProfile.Distro = "aks-azurelinux-v2-gen2"
				customKubeletConfig := &datamodel.CustomKubeletConfig{
					SeccompDefault: to.Ptr(true),
				}
				nbc.AgentPoolProfile.CustomKubeletConfig = customKubeletConfig
				nbc.ContainerService.Properties.AgentPoolProfiles[0].CustomKubeletConfig = customKubeletConfig
			},
			Validator: func(ctx context.Context, s *Scenario) {
				kubeletConfigFilePath := "/etc/default/kubeletconfig.json"
				ValidateFileHasContent(ctx, s, kubeletConfigFilePath, `"seccompDefault": true`)
				ValidateKubeletHasFlags(ctx, s, kubeletConfigFilePath)
				ValidateInstalledPackageVersion(ctx, s, "moby-containerd", getExpectedPackageVersions("containerd", "mariner", "current")[0])
			},
		}})
}

func Test_AzureLinuxV2_KubeletCustomConfig_Scriptless(t *testing.T) {
	RunScenario(t, &Scenario{
		Tags: Tags{
			KubeletCustomConfig: true,
			Scriptless:          true,
		},
		Description: "tests that a node on azure linux v2 bootstrapped with kubelet custom config for seccomp set to non default values",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV2Gen2,
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
				config.KubeletConfig.KubeletConfigFileConfig.SeccompDefault = true
			},
			Validator: func(ctx context.Context, s *Scenario) {
				kubeletConfigFilePath := "/etc/default/kubeletconfig.json"
				ValidateFileHasContent(ctx, s, kubeletConfigFilePath, `"seccompDefault": true`)
				ValidateKubeletHasFlags(ctx, s, kubeletConfigFilePath)
				ValidateInstalledPackageVersion(ctx, s, "moby-containerd", getExpectedPackageVersions("containerd", "mariner", "current")[0])
			},
		}})
}

func Test_Ubuntu2204ARM64_KubeletCustomConfig(t *testing.T) {
	RunScenario(t, &Scenario{
		Tags: Tags{
			KubeletCustomConfig: true,
		},
		Description: "tests that a node on ubuntu 2204 ARM64 bootstrapped with kubelet custom config",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Arm64Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.IsARM64 = true
				nbc.AgentPoolProfile.Distro = "aks-ubuntu-arm64-containerd-22.04-gen2"
				nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_D2pds_V5"
				nbc.AgentPoolProfile.VMSize = "Standard_D2pds_V5"

				customKubeletConfig := &datamodel.CustomKubeletConfig{
					SeccompDefault: to.Ptr(true),
				}
				nbc.AgentPoolProfile.CustomKubeletConfig = customKubeletConfig
				nbc.ContainerService.Properties.AgentPoolProfiles[0].CustomKubeletConfig = customKubeletConfig
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_D2pds_V5")
			},

			Validator: func(ctx context.Context, s *Scenario) {
				kubeletConfigFilePath := "/etc/default/kubeletconfig.json"
				ValidateFileHasContent(ctx, s, kubeletConfigFilePath, `"seccompDefault": true`)
				ValidateKubeletHasFlags(ctx, s, kubeletConfigFilePath)
			},
		}})
}

func Test_Ubuntu2404Gen2(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using the Ubuntu 2404 VHD can be properly bootstrapped with containerd v2",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2404Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
			},
			Validator: func(ctx context.Context, s *Scenario) {
				containerdVersions := getExpectedPackageVersions("containerd", "ubuntu", "r2404")
				runcVersions := getExpectedPackageVersions("runc", "ubuntu", "r2404")
				ValidateContainerd2Properties(ctx, s, containerdVersions)
				ValidateRunc12Properties(ctx, s, runcVersions)
				ValidateContainerRuntimePlugins(ctx, s)
				ValidateSSHServiceEnabled(ctx, s)
			},
		}})
}

func Test_Ubuntu2404Gen2_SecureTLSBootstrapping_BootstrapToken_Fallback(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using an Ubuntu 2404 Gen2 VHD can be properly bootstrapped even if secure TLS bootstrapping fails",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2404Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				// secure TLS bootstrapping is not yet enabled in e2e regions, thus this will test the bootstrap token fallback case
				nbc.EnableSecureTLSBootstrapping = true
			},
		},
	})
}

func Test_Ubuntu2404Gen2_GPUNoDriver(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a GPU-enabled node using the Ubuntu 2404 VHD opting for skipping gpu driver installation can be properly bootstrapped",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2404Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = "Standard_NC6s_v3"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = false
				nbc.EnableNvidia = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.Tags = map[string]*string{
					// deliberately case mismatched to agentbaker logic to check case insensitivity
					"SkipGPUDriverInstall": to.Ptr("true"),
				}
				vmss.SKU.Name = to.Ptr("Standard_NC6s_v3")
			},
			Validator: func(ctx context.Context, s *Scenario) {
				containerdVersions := getExpectedPackageVersions("containerd", "ubuntu", "r2404")
				runcVersions := getExpectedPackageVersions("runc", "ubuntu", "r2404")

				ValidateNvidiaSMINotInstalled(ctx, s)
				ValidateContainerd2Properties(ctx, s, containerdVersions)
				ValidateRunc12Properties(ctx, s, runcVersions)
			},
		}})
}

func Test_Ubuntu2404Gen1(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using the Ubuntu 2404 VHD can be properly bootstrapped with containerd v2",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2404Gen1Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
			},
			Validator: func(ctx context.Context, s *Scenario) {
				containerdVersions := getExpectedPackageVersions("containerd", "ubuntu", "r2404")
				runcVersions := getExpectedPackageVersions("runc", "ubuntu", "r2404")
				ValidateContainerd2Properties(ctx, s, containerdVersions)
				ValidateRunc12Properties(ctx, s, runcVersions)
			},
		}})
}

func Test_Ubuntu2404ARM(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using the Ubuntu 2404 VHD can be properly bootstrapped with containerd v2",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2404ArmContainerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_D2pds_V5")
			},
			Validator: func(ctx context.Context, s *Scenario) {
				containerdVersions := getExpectedPackageVersions("containerd", "ubuntu", "r2404")
				runcVersions := getExpectedPackageVersions("runc", "ubuntu", "r2404")
				ValidateContainerd2Properties(ctx, s, containerdVersions)
				ValidateRunc12Properties(ctx, s, runcVersions)
			},
		}})
}

func Test_Random_VHD_With_Latest_Kubernetes_Version(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using a Random VHD can be properly bootstrapped with the latest kubernetes version",
		Config: Config{
			Cluster: ClusterLatestKubernetesVersion,
			VHD:     config.GetRandomLinuxAMD64VHD(),
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
			},
		},
	})
}

func runScenarioUbuntu2404GRID(t *testing.T, vmSize string) {
	RunScenario(t, &Scenario{
		Description: fmt.Sprintf("Tests that a GPU-enabled node with VM size %s using an Ubuntu 2404 VHD can be properly bootstrapped, and that the GRID license is valid", vmSize),
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2404Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = vmSize
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = false
				nbc.EnableNvidia = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr(vmSize)
			},
			Validator: func(ctx context.Context, s *Scenario) {
				// Ensure nvidia-modprobe install does not restart kubelet and temporarily cause node to be unschedulable
				ValidateNvidiaModProbeInstalled(ctx, s)
				ValidateNvidiaGRIDLicenseValid(ctx, s)
				ValidateKubeletHasNotStopped(ctx, s)
				ValidateServicesDoNotRestartKubelet(ctx, s)
				ValidateNvidiaPersistencedRunning(ctx, s)
			},
		},
	})
}

func Test_Ubuntu2404_GPUA10(t *testing.T) {
	runScenarioUbuntu2404GRID(t, "Standard_NV6ads_A10_v5")
}

func Test_Ubuntu2404_NPD_Basic(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Test that a node with AKS VM Extension enabled can report simulated node problem detector events",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2404Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				extension, err := createVMExtensionLinuxAKSNode(vmss.Location)
				if err != nil {
					t.Fatalf("creating AKS VM extension: %v", err)
				}
				vmss.Properties = addVMExtensionToVMSS(vmss.Properties, extension)
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateNodeProblemDetector(ctx, s)
				ValidateNPDFilesystemCorruption(ctx, s)
			},
		}})
}

func Test_AlternateLocation_BasicDeployment(t *testing.T) {
	location := "southafricanorth" // Set the alternate location for the test

	RunScenario(t, &Scenario{
		Description: "Tests basic node deployment in configured location",
		Location:    location, // Override location for this test
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				// Add your custom configuration here
			},
			Validator: func(ctx context.Context, s *Scenario) {
				// Validate kubelet is running
				ValidateSystemdUnitIsRunning(ctx, s, "kubelet")
				// Add your custom validations here
				t.Logf("Successfully validated deployment in location: %s", location)
			},
		},
	})
}
