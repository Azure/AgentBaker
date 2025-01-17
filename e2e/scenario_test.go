package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/toolkit"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
)

func Test_AzureLinuxV2(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using a AzureLinuxV2 (CgroupV2) VHD can be properly bootstrapped",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateInstalledPackageVersion(ctx, s, "moby-containerd", getExpectedPackageVersions("containerd", "mariner", "current")[0])
			},
		},
	})
}

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
						ContainerRegistryServer: fmt.Sprintf("%s.azurecr.io", config.PrivateACRName),
					},
				}
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
				nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.CustomKubeBinaryURL = "https://acs-mirror.azureedge.net/kubernetes/v1.24.9/binaries/kubernetes-node-linux-arm64.tar.gz"
				nbc.AgentPoolProfile.VMSize = "Standard_D2pds_V5"
				nbc.IsARM64 = true

				nbc.OutboundType = datamodel.OutboundTypeBlock
				nbc.ContainerService.Properties.SecurityProfile = &datamodel.SecurityProfile{
					PrivateEgress: &datamodel.PrivateEgress{
						Enabled:                 true,
						ContainerRegistryServer: fmt.Sprintf("%s.azurecr.io", config.PrivateACRName),
					},
				}
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_D2pds_V5")
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

func Test_AzureLinuxV2_WASM(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "tests that a new AzureLinuxV2 (CgroupV2) node using krustlet can be properly bootstrapped",
		Tags: Tags{
			WASM: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.WorkloadRuntime = datamodel.WasmWasi
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateContainerdWASMShims(ctx, s)
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
						ContainerRegistryServer: fmt.Sprintf("%s.azurecr.io", config.PrivateACRName),
					},
				}
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
				nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.CustomKubeBinaryURL = "https://acs-mirror.azureedge.net/kubernetes/v1.24.9/binaries/kubernetes-node-linux-arm64.tar.gz"
				nbc.AgentPoolProfile.VMSize = "Standard_D2pds_V5"
				nbc.IsARM64 = true

				nbc.OutboundType = datamodel.OutboundTypeBlock
				nbc.ContainerService.Properties.SecurityProfile = &datamodel.SecurityProfile{
					PrivateEgress: &datamodel.PrivateEgress{
						Enabled:                 true,
						ContainerRegistryServer: fmt.Sprintf("%s.azurecr.io", config.PrivateACRName),
					},
				}

			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_D2pds_V5")
			},
		},
	})
}

func Test_MarinerV2_AzureCNI(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "marinerv2 scenario on a cluster configured with Azure CNI",
		Config: Config{
			Cluster: ClusterAzureNetwork,
			VHD:     config.VHDCBLMarinerV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = string(armcontainerservice.NetworkPluginAzure)
				nbc.AgentPoolProfile.KubernetesConfig.NetworkPlugin = string(armcontainerservice.NetworkPluginAzure)
			},
		},
	})
}

func Test_MarinerV2_ChronyRestarts(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that the chrony service restarts if it is killed",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDCBLMarinerV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
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

func Test_MarinerV2_WASM(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "tests that a new marinerv2 node using krustlet can be properly bootstrapped",
		Tags: Tags{
			WASM: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDCBLMarinerV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.WorkloadRuntime = datamodel.WasmWasi
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateContainerdWASMShims(ctx, s)
			},
		},
	})
}

// Returns config for the 'base' E2E scenario
func Test_Ubuntu1804(t *testing.T) {
	// for ubuntu1804 containerd version is frozen and its using outdated versioning style, hence this modification
	expected1804ContainredVersion := strings.Replace(getExpectedPackageVersions("containerd", "ubuntu", "r1804")[0], "-", "+azure-ubuntu18.04u", 1)
	RunScenario(t, &Scenario{
		Description: "Tests that a node using an Ubuntu 1804 VHD can be properly bootstrapped",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu1804Gen2Containerd,
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateInstalledPackageVersion(ctx, s, "moby-containerd", expected1804ContainredVersion)
				ValidateInstalledPackageVersion(ctx, s, "moby-runc", getExpectedPackageVersions("runc", "ubuntu", "r1804")[0])
			},

			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {},
		},
	})
}

func Test_Ubuntu1804_AzureCNI(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "ubuntu1804 scenario on cluster configured with Azure CNI",
		Config: Config{
			Cluster: ClusterAzureNetwork,
			VHD:     config.VHDUbuntu1804Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = string(armcontainerservice.NetworkPluginAzure)
				nbc.AgentPoolProfile.KubernetesConfig.NetworkPlugin = string(armcontainerservice.NetworkPluginAzure)
			},
		},
	})
}

func Test_Ubuntu1804_ChronyRestarts(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that the chrony service restarts if it is killed",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu1804Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ServiceCanRestartValidator(ctx, s, "chronyd", 10)
				ValidateFileHasContent(ctx, s, "/etc/systemd/system/chrony.service.d/10-chrony-restarts.conf", "Restart=always")
				ValidateFileHasContent(ctx, s, "/etc/systemd/system/chrony.service.d/10-chrony-restarts.conf", "RestartSec=5")
			},
		},
	})
}

func Test_Ubuntu2204_ScriptlessInstaller(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "tests that a new ubuntu 2204 node using self contained installer can be properly bootstrapped",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/var/log/azure/aks-node-controller.log", "aks-node-controller finished successfully")
			},
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {},
		},
	})
}

func Test_Ubuntu2404_ScriptlessInstaller(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "tests that a new ubuntu 2404 node using self contained installer can be properly bootstrapped",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2404Gen2Containerd,
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/var/log/azure/aks-node-controller.log", "aks-node-controller finished successfully")
			},
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {},
		},
	})
}

// Returns config for the 'gpu' E2E scenario
func Test_Ubuntu1804_GPU(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a GPU-enabled node using an Ubuntu 1804 VHD can be properly bootstrapped",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu1804Gen2Containerd,
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

func Test_Ubuntu1804_GPUAzureCNI(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Ubuntu1804 gpu scenario on cluster configured with Azure CNI",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterAzureNetwork,
			VHD:     config.VHDUbuntu1804Gen2Containerd,
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
			},
		},
	})
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
						ContainerRegistryServer: fmt.Sprintf("%s.azurecr.io", config.PrivateACRName),
					},
				}
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
						ContainerRegistryServer: fmt.Sprintf("%s.azurecr.io", config.PrivateACRName),
					},
				}
			},
		},
	})
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
		},
	})
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
			},
		},
	})
}

func Test_Ubuntu2204_ChronyRestarts(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that the chrony service restarts if it is killed",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
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

func Test_Ubuntu2204_CustomCATrust(t *testing.T) {
	const encodedTestCert = "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUgvVENDQmVXZ0F3SUJBZ0lRYUJZRTMvTTA4WEhZQ25OVm1jRkJjakFOQmdrcWhraUc5dzBCQVFzRkFEQnkKTVFzd0NRWURWUVFHRXdKVlV6RU9NQXdHQTFVRUNBd0ZWR1Y0WVhNeEVEQU9CZ05WQkFjTUIwaHZkWE4wYjI0eApFVEFQQmdOVkJBb01DRk5UVENCRGIzSndNUzR3TEFZRFZRUUREQ1ZUVTB3dVkyOXRJRVZXSUZOVFRDQkpiblJsCmNtMWxaR2xoZEdVZ1EwRWdVbE5CSUZJek1CNFhEVEl3TURRd01UQXdOVGd6TTFvWERUSXhNRGN4TmpBd05UZ3oKTTFvd2diMHhDekFKQmdOVkJBWVRBbFZUTVE0d0RBWURWUVFJREFWVVpYaGhjekVRTUE0R0ExVUVCd3dIU0c5MQpjM1J2YmpFUk1BOEdBMVVFQ2d3SVUxTk1JRU52Y25BeEZqQVVCZ05WQkFVVERVNVdNakF3T0RFMk1UUXlORE14CkZEQVNCZ05WQkFNTUMzZDNkeTV6YzJ3dVkyOXRNUjB3R3dZRFZRUVBEQlJRY21sMllYUmxJRTl5WjJGdWFYcGgKZEdsdmJqRVhNQlVHQ3lzR0FRUUJnamM4QWdFQ0RBWk9aWFpoWkdFeEV6QVJCZ3NyQmdFRUFZSTNQQUlCQXhNQwpWVk13Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRREhoZVJrYmIxRkNjN3hSS3N0CndLMEpJR2FLWTh0N0piUzJiUTJiNllJSkRnbkh1SVlIcUJyQ1VWNzlvZWxpa2tva1JrRnZjdnBhS2luRkhEUUgKVXBXRUk2UlVFUlltU0NnM084V2k0MnVPY1YyQjVaYWJtWENrd2R4WTVFY2w1MUJiTThVbkdkb0FHYmRObWlSbQpTbVRqY3MrbGhNeGc0ZkZZNmxCcGlFVkZpR1VqR1JSKzYxUjY3THo2VTRLSmVMTmNDbTA3UXdGWUtCbXBpMDhnCmR5Z1N2UmRVdzU1Sm9wcmVkaitWR3RqVWtCNGhGVDRHUVgvZ2h0NjlSbHF6Lys4dTBkRVFraHVVdXVjcnFhbG0KU0d5NDNIUndCZkRLRndZZVdNN0NQTWQ1ZS9kTyt0MDh0OFBianpWVFR2NWhRRENzRVlJVjJUN0FGSTlTY054TQpraDcvQWdNQkFBR2pnZ05CTUlJRFBUQWZCZ05WSFNNRUdEQVdnQlMvd1ZxSC95ajZRVDM5dDAva0hhK2dZVmdwCnZUQi9CZ2dyQmdFRkJRY0JBUVJ6TUhFd1RRWUlLd1lCQlFVSE1BS0dRV2gwZEhBNkx5OTNkM2N1YzNOc0xtTnYKYlM5eVpYQnZjMmwwYjNKNUwxTlRUR052YlMxVGRXSkRRUzFGVmkxVFUwd3RVbE5CTFRRd09UWXRVak11WTNKMApNQ0FHQ0NzR0FRVUZCekFCaGhSb2RIUndPaTh2YjJOemNITXVjM05zTG1OdmJUQWZCZ05WSFJFRUdEQVdnZ3QzCmQzY3VjM05zTG1OdmJZSUhjM05zTG1OdmJUQmZCZ05WSFNBRVdEQldNQWNHQldlQkRBRUJNQTBHQ3lxRWFBR0cKOW5jQ0JRRUJNRHdHRENzR0FRUUJncWt3QVFNQkJEQXNNQ29HQ0NzR0FRVUZCd0lCRmg1b2RIUndjem92TDNkMwpkeTV6YzJ3dVkyOXRMM0psY0c5emFYUnZjbmt3SFFZRFZSMGxCQll3RkFZSUt3WUJCUVVIQXdJR0NDc0dBUVVGCkJ3TUJNRWdHQTFVZEh3UkJNRDh3UGFBN29EbUdOMmgwZEhBNkx5OWpjbXh6TG5OemJDNWpiMjB2VTFOTVkyOXQKTFZOMVlrTkJMVVZXTFZOVFRDMVNVMEV0TkRBNU5pMVNNeTVqY213d0hRWURWUjBPQkJZRUZBREFGVUlhenc1cgpaSUhhcG5SeElVbnB3K0dMTUE0R0ExVWREd0VCL3dRRUF3SUZvRENDQVgwR0Npc0dBUVFCMW5rQ0JBSUVnZ0Z0CkJJSUJhUUZuQUhjQTlseVVMOUYzTUNJVVZCZ0lNSlJXanVOTkV4a3p2OThNTHlBTHpFN3haT01BQUFGeE0waG8KYndBQUJBTUFTREJHQWlFQTZ4ZWxpTlI4R2svNjNwWWRuUy92T3gvQ2pwdEVNRXY4OVdXaDEvdXJXSUVDSVFEeQpCcmVIVTI1RHp3dWtRYVJRandXNjU1WkxrcUNueGJ4UVdSaU9lbWo5SkFCMUFKUWd2QjZPMVkxc2lITWZnb3NpCkxBM1IyazFlYkUrVVBXSGJUaTlZVGFMQ0FBQUJjVE5JYU53QUFBUURBRVl3UkFJZ0dSRTR3emFiTlJkRDhrcS8KdkZQM3RRZTJobTB4NW5YdWxvd2g0SWJ3M2xrQ0lGWWIvM2xTRHBsUzdBY1I0citYcFd0RUtTVEZXSm1OQ1JiYwpYSnVyMlJHQkFIVUE3c0NWN28xeVpBK1M0OE81RzhjU28ybHFDWHRMYWhvVU9PWkhzc3Z0eGZrQUFBRnhNMGhvCjh3QUFCQU1BUmpCRUFpQjZJdmJvV3NzM1I0SXRWd2plYmw3RDN5b0ZhWDBORGgyZFdoaGd3Q3hySHdJZ0NmcTcKb2NNQzV0KzFqaTVNNXhhTG1QQzRJK1dYM0kvQVJrV1N5aU83SVFjd0RRWUpLb1pJaHZjTkFRRUxCUUFEZ2dJQgpBQ2V1dXI0UW51anFtZ3VTckhVM21oZitjSm9kelRRTnFvNHRkZStQRDEvZUZkWUFFTHU4eEYrMEF0N3hKaVBZCmk1Ukt3aWx5UDU2diszaVkyVDlsdzdTOFRKMDQxVkxoYUlLcDE0TXpTVXpSeWVvT0FzSjdRQURNQ2xIS1VEbEgKVVUycE51bzg4WTZpZ292VDNic253Sk5pRVFOcXltU1NZaGt0dzB0YWR1b3FqcVhuMDZnc1Zpb1dUVkRYeXNkNQpxRXg0dDZzSWdJY01tMjZZSDF2SnBDUUVoS3BjMnkwN2dSa2tsQlpSdE1qVGh2NGNYeXlNWDd1VGNkVDdBSkJQCnVlaWZDb1YyNUp4WHVvOGQ1MTM5Z3dQMUJBZTdJQlZQeDJ1N0tOL1V5T1hkWm13TWYvVG1GR3dEZENmc3lIZi8KWnNCMndMSG96VFlvQVZtUTlGb1UxSkxnY1ZpdnFKK3ZObEJoSFhobHhNZE4wajgwUjlOejZFSWdsUWplSzNPOApJL2NGR20vQjgrNDJoT2xDSWQ5WmR0bmRKY1JKVmppMHdEMHF3ZXZDYWZBOWpKbEh2L2pzRStJOVV6NmNwQ3loCnN3K2xyRmR4VWdxVTU4YXhxZUs4OUZSK05vNHEwSUlPK0ppMXJKS3I5bmtTQjBCcVhvelZuRTFZQi9LTHZkSXMKdVlaSnVxYjJwS2t1K3p6VDZnVXdIVVRadkJpTk90WEw0Tnh3Yy9LVDdXek9TZDJ3UDEwUUk4REtnNHZmaU5EcwpIV21CMWM0S2ppNmdPZ0E1dVNVemFHbXEvdjRWbmNLNVVyK245TGJmbmZMYzI4SjVmdC9Hb3Rpbk15RGszaWFyCkYxMFlscWNPbWVYMXVGbUtiZGkvWG9yR2xrQ29NRjNURHg4cm1wOURCaUIvCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0=" //nolint:lll
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

func Test_Ubuntu2204_GPUNC(t *testing.T) {
	runScenarioUbuntu2204GPU(t, "Standard_NC6s_v3")
}

func Test_Ubuntu2204_GPUA100(t *testing.T) {
	runScenarioUbuntu2204GPU(t, "Standard_NC24ads_A100_v4")
}

func Test_Ubuntu2204_GPUA10(t *testing.T) {
	runScenarioUbuntu2204GPU(t, "Standard_NV6ads_A10_v5")
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
		},
	})
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
		},
	})
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
		},
	})
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
			},
		},
	})
}

// These tests were created to verify that the apt-get call in downloadContainerdFromVersion is not executed.
// The code path is not hit in either of these tests. In the future, testing with some kind of firewall to ensure no egress
// calls are made would be beneficial for airgap testing.

func Test_Ubuntu2204_ContainerdURL(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "tests that a node using the Ubuntu 2204 VHD with the ContainerdPackageURL override bootstraps with the provided URL and not the components.json containerd version",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerdPackageURL = "https://packages.microsoft.com/ubuntu/22.04/prod/pool/main/m/moby-containerd/moby-containerd_1.6.9+azure-ubuntu22.04u1_amd64.deb"
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateInstalledPackageVersion(ctx, s, "containerd", "1.6.9")
			},
		},
	})
}

func Test_Ubuntu2204_ContainerdHasCurrentVersion(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "tests that a node using an Ubuntu2204 VHD and the ContainerdVersion override bootstraps with the correct components.json containerd version and ignores the override",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerdVersion = "1.6.9"
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateInstalledPackageVersion(ctx, s, "moby-containerd", getExpectedPackageVersions("containerd", "ubuntu", "r2204")[0])
			},
		},
	})
}

func Test_Ubuntu2204_WASM(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "tests that a new ubuntu 2204 node using krustlet can be properly bootstrapepd",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.WorkloadRuntime = datamodel.WasmWasi
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateContainerdWASMShims(ctx, s)
			},
		},
	})
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
		},
	})
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
		},
	})
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
		},
	})
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
		},
	})
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
		},
	})
}

func Test_Ubuntu2204_WASMAirGap(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "tests that a new ubuntu 2204 node using krustlet can be properly bootstrapepd when it is network isolated cluster",
		Tags: Tags{
			Airgap: true,
		},
		Config: Config{
			Cluster: ClusterKubenetAirgap,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.WorkloadRuntime = datamodel.WasmWasi

				nbc.OutboundType = datamodel.OutboundTypeBlock
				nbc.ContainerService.Properties.SecurityProfile = &datamodel.SecurityProfile{
					PrivateEgress: &datamodel.PrivateEgress{
						Enabled:                 true,
						ContainerRegistryServer: fmt.Sprintf("%s.azurecr.io", config.PrivateACRName),
					},
				}
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateContainerdWASMShims(ctx, s)
			},
		},
	})
}

func Test_Ubuntu2204_IMDSRestrictionFilterTable(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "tests that the imds restriction filter table is properly set",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.EnableIMDSRestriction = true
				nbc.InsertIMDSRestrictionRuleToMangleTable = false
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateIMDSRestrictionRule(ctx, s, "filter")
			},
		},
	})
}

func Test_Ubuntu1804IMDS_RestrictionMangleTable(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "tests that the imds restriction mangle table is properly set",
		Config: Config{
			Cluster: ClusterAzureNetwork,
			VHD:     config.VHDUbuntu1804Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = string(armcontainerservice.NetworkPluginAzure)
				nbc.AgentPoolProfile.KubernetesConfig.NetworkPlugin = string(armcontainerservice.NetworkPluginAzure)
				nbc.EnableIMDSRestriction = true
				nbc.InsertIMDSRestrictionRuleToMangleTable = true
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateIMDSRestrictionRule(ctx, s, "mangle")
			},
		},
	})
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
		},
	})
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
		},
	})
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
		},
	})
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
			},
		},
	})
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
		},
	})
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
			},
		},
	})
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
		},
	})
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
		},
	})
}
