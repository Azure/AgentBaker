package e2e

import (
	"context"
	"fmt"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbakere2e/config"
	"github.com/Azure/agentbakere2e/toolkit"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
	"github.com/stretchr/testify/require"
)

func TestAll(t *testing.T) {
	t.Parallel()
	err := ensureResourceGroup(context.TODO())
	require.NoError(t, err)
	// grouping all tests together helps with reporting on parallel tests
	t.Run("azurelinuxv2", Scenario_azurelinuxv2)
	t.Run("azurelinuxv2-airgap", Scenario_azurelinuxv2AirGap)
	t.Run("azurelinuxv2-arm64", Scenario_azurelinuxv2ARM64)
	t.Run("azurelinuxv2-arm64-airgap", Scenario_azurelinuxv2ARM64AirGap)
	t.Run("azurelinuxv2-azurecni", Scenario_azurelinuxv2_azurecni)
	t.Run("azurelinuxv2-containerd-version", Scenario_azurelinuxv2HasRightContainerdVersion)
	t.Run("azurelinuxv2-runc-version", Scenario_azurelinuxv2HasRightRuncVersion)
	t.Run("azurelinuxv2-chrony-restarts", Scenario_azurelinuxv2ChronyRestarts)
	t.Run("azurelinuxv2-custom-sysctls", Scenario_azurelinuxv2CustomSysctls)
	t.Run("azurelinuxv2-gpu", Scenario_azurelinuxv2gpu)
	t.Run("azurelinuxv2-gpu-azurecni", Scenario_azurelinuxv2gpu_azurecni)
	t.Run("azurelinuxv2-wasm", Scenario_azurelinuxv2Wasm)
	t.Run("marinerv2", Scenario_marinerv2)
	t.Run("marinerv2-airgap", Scenario_marinerv2AirGap)
	t.Run("marinerv2-arm64", Scenario_marinerv2ARM64)
	t.Run("marinerv2-arm64-airgap", Scenario_marinerv2ARM64AirGap)
	t.Run("marinerv2-azurecni", Scenario_marinerv2_azurecni)
	t.Run("marinerv2-containerd-version", Scenario_marinerv2HasRightContainerdVersion)
	t.Run("marinerv2-runc-version", Scenario_marinerv2HasRightRuncVersion)
	t.Run("marinerv2-chrony-restarts", Scenario_marinerv2ChronyRestarts)
	t.Run("marinerv2-custom-sysctls", Scenario_marinerv2CustomSysctls)
	t.Run("marinerv2-gpu", Scenario_marinerv2gpu)
	t.Run("marinerv2-gpu-azurecni", Scenario_marinerv2gpu_azurecni)
	t.Run("marinerv2-wasm", Scenario_marinerv2Wasm)
	t.Run("ubuntu1804", Scenario_ubuntu1804)
	t.Run("ubuntu1804-azurecni", Scenario_ubuntu1804_azurecni)
	t.Run("ubuntu1804-chrony-restarts", Scenario_ubuntu1804ChronyRestarts)
	t.Run("ubuntu1804-containerd-version", Scenario_ubuntu1804HasRightContainerdVersion)
	t.Run("ubuntu1804-runc-version", Scenario_ubuntu1804HasRightRuncVersion)
	t.Run("ubuntu1804-gpu", Scenario_ubuntu1804gpu)
	t.Run("ubuntu1804-gpu-azurecni", Scenario_ubuntu1804gpu_azurecni)
	t.Run("ubuntu2204", Scenario_ubuntu2204)
	t.Run("ubuntu2204-airgap", Scenario_ubuntu2204AirGap)
	t.Run("ubuntu2204-arm64", Scenario_ubuntu2204ARM64)
	t.Run("ubuntu2204-artifact-streaming", Scenario_ubuntu2204ArtifactStreaming)
	t.Run("ubuntu2204-chrony-restarts", Scenario_ubuntu2204ChronyRestarts)
	t.Run("ubuntu2204-containerd-version", Scenario_ubuntu2204HasRightContainerdVersion)
	t.Run("ubuntu2204-runc-version", Scenario_ubuntu2204HasRightRuncVersion)
	t.Run("ubuntu2204-containerd-override", Scenario_ubuntu2204ContainerdURL)
	t.Run("ubuntu2204-containerd-cur-ver", Scenario_ubuntu2204ContainerdHasCurrentVersion)
	t.Run("ubuntu2204-custom-ca-trust", Scenario_ubuntu2204CustomCATrust)
	t.Run("ubuntu2204-custom-sysctls", Scenario_ubuntu2204CustomSysctls)
	t.Run("ubuntu2204-gpu-a10", Scenario_ubuntu2204gpua10)
	t.Run("ubuntu2204-gpu-a100", Scenario_ubuntu2204gpua100)
	t.Run("ubuntu2204-gpu-grid-driver", Scenario_ubuntu2204GPUGridDriver)
	t.Run("ubuntu2204-gpu-ncv", Scenario_ubuntu2204gpuncv)
	t.Run("ubuntu2204-gpu-no-driver", Scenario_ubuntu2204gpuNoDriver)
	t.Run("ubuntu2204-wasm", Scenario_ubuntu2204Wasm)
}

func Scenario_azurelinuxv2(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using a AzureLinuxV2 (CgroupV2) VHD can be properly bootstrapped",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-azurelinux-v2-gen2"
				nbc.AgentPoolProfile.Distro = "aks-azurelinux-v2-gen2"
			},
		},
	})
}

func Scenario_azurelinuxv2AirGap(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using a AzureLinuxV2 (CgroupV2) VHD can be properly bootstrapped",
		Tags: Tags{
			Airgap: true,
		},
		Config: Config{
			Cluster: ClusterKubenetAirgap,
			VHD:     config.VHDAzureLinuxV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-azurelinux-v2-gen2"
				nbc.AgentPoolProfile.Distro = "aks-azurelinux-v2-gen2"
			},
		},
	})
}

func Scenario_azurelinuxv2ARM64(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using a AzureLinuxV2 (CgroupV2) VHD on ARM64 architecture can be properly bootstrapped",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV2Gen2Arm64,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_D2pds_V5"
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-azurelinux-v2-arm64-gen2"
				nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.CustomKubeBinaryURL = "https://acs-mirror.azureedge.net/kubernetes/v1.24.9/binaries/kubernetes-node-linux-arm64.tar.gz"
				nbc.AgentPoolProfile.VMSize = "Standard_D2pds_V5"
				nbc.AgentPoolProfile.Distro = "aks-azurelinux-v2-arm64-gen2"
				nbc.IsARM64 = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_D2pds_V5")
			},
		},
	})
}

func Scenario_azurelinuxv2ARM64AirGap(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using a AzureLinuxV2 (CgroupV2) VHD on ARM64 architecture can be properly bootstrapped",
		Tags: Tags{
			Airgap: true,
		},
		Config: Config{
			Cluster: ClusterKubenetAirgap,
			VHD:     config.VHDAzureLinuxV2Gen2Arm64,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_D2pds_V5"
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-azurelinux-v2-arm64-gen2"
				nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.CustomKubeBinaryURL = "https://acs-mirror.azureedge.net/kubernetes/v1.24.9/binaries/kubernetes-node-linux-arm64.tar.gz"
				nbc.AgentPoolProfile.VMSize = "Standard_D2pds_V5"
				nbc.AgentPoolProfile.Distro = "aks-azurelinux-v2-arm64-gen2"
				nbc.IsARM64 = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_D2pds_V5")
			},
		},
	})
}

func Scenario_azurelinuxv2_azurecni(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "azurelinuxv2 scenario on a cluster configured with Azure CNI",
		Config: Config{
			Cluster: ClusterAzureNetwork,
			VHD:     config.VHDAzureLinuxV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = string(armcontainerservice.NetworkPluginAzure)
				nbc.AgentPoolProfile.KubernetesConfig.NetworkPlugin = string(armcontainerservice.NetworkPluginAzure)
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-azurelinux-v2-gen2"
				nbc.AgentPoolProfile.Distro = "aks-azurelinux-v2-gen2"
			},
		},
	})
}

func Scenario_azurelinuxv2ChronyRestarts(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that the chrony service restarts if it is killed",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-azurelinux-v2-gen2"
				nbc.AgentPoolProfile.Distro = "aks-azurelinux-v2-gen2"
			},
			LiveVMValidators: []*LiveVMValidator{
				ServiceCanRestartValidator("chronyd", 10),
				FileHasContentsValidator("/etc/systemd/system/chronyd.service.d/10-chrony-restarts.conf", "Restart=always"),
				FileHasContentsValidator("/etc/systemd/system/chronyd.service.d/10-chrony-restarts.conf", "RestartSec=5"),
			},
		},
	})
}

func Scenario_azurelinuxv2CustomSysctls(t *testing.T) {
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
				nbc.ContainerService.Properties.AgentPoolProfiles[0].CustomLinuxOSConfig = customLinuxConfig
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-azurelinux-v2-gen2"
				nbc.AgentPoolProfile.Distro = "aks-azurelinux-v2-gen2"
			},
			LiveVMValidators: []*LiveVMValidator{
				SysctlConfigValidator(customSysctls),
				UlimitValidator(customContainerdUlimits),
			},
		},
	})
}

// Returns config for the 'gpu' E2E scenario
func Scenario_azurelinuxv2gpu(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a GPU-enabled node using a AzureLinuxV2 (CgroupV2) VHD can be properly bootstrapped",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_NC6s_v3"
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-azurelinux-v2-gen2"
				nbc.AgentPoolProfile.VMSize = "Standard_NC6s_v3"
				nbc.AgentPoolProfile.Distro = "aks-azurelinux-v2-gen2"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = false
				nbc.EnableNvidia = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NC6s_v3")
			},
		},
	})
}

func Scenario_azurelinuxv2gpu_azurecni(t *testing.T) {
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
				nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_NC6s_v3"
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-azurelinux-v2-gen2"
				nbc.AgentPoolProfile.VMSize = "Standard_NC6s_v3"
				nbc.AgentPoolProfile.Distro = "aks-azurelinux-v2-gen2"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = false
				nbc.EnableNvidia = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NC6s_v3")
			},
		},
	})
}

func Scenario_azurelinuxv2Wasm(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "tests that a new AzureLinuxV2 (CgroupV2) node using krustlet can be properly bootstrapped",
		Tags: Tags{
			WASM: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].WorkloadRuntime = datamodel.WasmWasi
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-azurelinux-v2-gen2"
				nbc.AgentPoolProfile.WorkloadRuntime = datamodel.WasmWasi
				nbc.AgentPoolProfile.Distro = "aks-azurelinux-v2-gen2"
			},
		},
	})
}

func Scenario_marinerv2(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using a MarinerV2 VHD can be properly bootstrapped",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDCBLMarinerV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-cblmariner-v2-gen2"
				nbc.AgentPoolProfile.Distro = "aks-cblmariner-v2-gen2"
			},
		},
	})
}

func Scenario_marinerv2AirGap(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using a MarinerV2 VHD can be properly bootstrapped",
		Tags: Tags{
			Airgap: true,
		},
		Config: Config{
			Cluster: ClusterKubenetAirgap,
			VHD:     config.VHDCBLMarinerV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-cblmariner-v2-gen2"
				nbc.AgentPoolProfile.Distro = "aks-cblmariner-v2-gen2"
			},
		},
	})
}

func Scenario_marinerv2ARM64(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using a MarinerV2 VHD on ARM64 architecture can be properly bootstrapped",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDCBLMarinerV2Gen2Arm64,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_D2pds_V5"
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-cblmariner-v2-arm64-gen2"
				nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.CustomKubeBinaryURL = "https://acs-mirror.azureedge.net/kubernetes/v1.24.9/binaries/kubernetes-node-linux-arm64.tar.gz"
				nbc.AgentPoolProfile.VMSize = "Standard_D2pds_V5"
				nbc.AgentPoolProfile.Distro = "aks-cblmariner-v2-arm64-gen2"
				nbc.IsARM64 = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_D2pds_V5")
			},
		},
	})
}

func Scenario_marinerv2ARM64AirGap(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using a MarinerV2 VHD on ARM64 architecture can be properly bootstrapped",
		Tags: Tags{
			Airgap: true,
		},
		Config: Config{
			Cluster: ClusterKubenetAirgap,
			VHD:     config.VHDCBLMarinerV2Gen2Arm64,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_D2pds_V5"
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-cblmariner-v2-arm64-gen2"
				nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.CustomKubeBinaryURL = "https://acs-mirror.azureedge.net/kubernetes/v1.24.9/binaries/kubernetes-node-linux-arm64.tar.gz"
				nbc.AgentPoolProfile.VMSize = "Standard_D2pds_V5"
				nbc.AgentPoolProfile.Distro = "aks-cblmariner-v2-arm64-gen2"
				nbc.IsARM64 = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_D2pds_V5")
			},
		},
	})
}

func Scenario_marinerv2_azurecni(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "marinerv2 scenario on a cluster configured with Azure CNI",
		Config: Config{
			Cluster: ClusterAzureNetwork,
			VHD:     config.VHDCBLMarinerV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = string(armcontainerservice.NetworkPluginAzure)
				nbc.AgentPoolProfile.KubernetesConfig.NetworkPlugin = string(armcontainerservice.NetworkPluginAzure)
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-cblmariner-v2-gen2"
				nbc.AgentPoolProfile.Distro = "aks-cblmariner-v2-gen2"
			},
		},
	})
}

func Scenario_marinerv2ChronyRestarts(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that the chrony service restarts if it is killed",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDCBLMarinerV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-cblmariner-v2-gen2"
				nbc.AgentPoolProfile.Distro = "aks-cblmariner-v2-gen2"
			},
			LiveVMValidators: []*LiveVMValidator{
				ServiceCanRestartValidator("chronyd", 10),
				FileHasContentsValidator("/etc/systemd/system/chronyd.service.d/10-chrony-restarts.conf", "Restart=always"),
				FileHasContentsValidator("/etc/systemd/system/chronyd.service.d/10-chrony-restarts.conf", "RestartSec=5"),
			},
		},
	})
}

func Scenario_marinerv2CustomSysctls(t *testing.T) {
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
				nbc.ContainerService.Properties.AgentPoolProfiles[0].CustomLinuxOSConfig = customLinuxConfig
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-cblmariner-v2-gen2"
				nbc.AgentPoolProfile.Distro = "aks-cblmariner-v2-gen2"
			},
			LiveVMValidators: []*LiveVMValidator{
				SysctlConfigValidator(customSysctls),
				UlimitValidator(customContainerdUlimits),
			},
		},
	})
}

// Returns config for the 'gpu' E2E scenario
func Scenario_marinerv2gpu(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a GPU-enabled node using a MarinerV2 VHD can be properly bootstrapped",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDCBLMarinerV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_NC6s_v3"
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-cblmariner-v2-gen2"
				nbc.AgentPoolProfile.VMSize = "Standard_NC6s_v3"
				nbc.AgentPoolProfile.Distro = "aks-cblmariner-v2-gen2"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = false
				nbc.EnableNvidia = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NC6s_v3")
			},
		},
	})
}

func Scenario_marinerv2gpu_azurecni(t *testing.T) {
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
				nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_NC6s_v3"
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-cblmariner-v2-gen2"
				nbc.AgentPoolProfile.VMSize = "Standard_NC6s_v3"
				nbc.AgentPoolProfile.Distro = "aks-cblmariner-v2-gen2"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = false
				nbc.EnableNvidia = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NC6s_v3")
			},
		},
	})
}

func Scenario_marinerv2Wasm(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "tests that a new marinerv2 node using krustlet can be properly bootstrapped",
		Tags: Tags{
			WASM: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDCBLMarinerV2Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].WorkloadRuntime = datamodel.WasmWasi
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-cblmariner-v2-gen2"
				nbc.AgentPoolProfile.WorkloadRuntime = datamodel.WasmWasi
				nbc.AgentPoolProfile.Distro = "aks-cblmariner-v2-gen2"
			},
		},
	})
}

// Returns config for the 'base' E2E scenario
func Scenario_ubuntu1804(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using an Ubuntu 1804 VHD can be properly bootstrapped",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu1804Gen2Containerd,
		},
	})
}

func Scenario_ubuntu1804_azurecni(t *testing.T) {
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

func Scenario_ubuntu1804ChronyRestarts(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that the chrony service restarts if it is killed",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu1804Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-containerd-18.04-gen2"
				nbc.AgentPoolProfile.Distro = "aks-ubuntu-containerd-18.04-gen2"
			},
			LiveVMValidators: []*LiveVMValidator{
				ServiceCanRestartValidator("chronyd", 10),
				FileHasContentsValidator("/etc/systemd/system/chrony.service.d/10-chrony-restarts.conf", "Restart=always"),
				FileHasContentsValidator("/etc/systemd/system/chrony.service.d/10-chrony-restarts.conf", "RestartSec=5"),
			},
		},
	})
}

// Returns config for the 'gpu' E2E scenario
func Scenario_ubuntu1804gpu(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a GPU-enabled node using an Ubuntu 1804 VHD can be properly bootstrapped",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu1804Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_NC6s_v3"
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-containerd-18.04-gen2"
				nbc.AgentPoolProfile.VMSize = "Standard_NC6s_v3"
				nbc.AgentPoolProfile.Distro = "aks-ubuntu-containerd-18.04-gen2"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = false
				nbc.EnableNvidia = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NC6s_v3")
			},
		},
	})
}

func Scenario_ubuntu1804gpu_azurecni(t *testing.T) {
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
				nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_NC6s_v3"
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-containerd-18.04-gen2"
				nbc.AgentPoolProfile.VMSize = "Standard_NC6s_v3"
				nbc.AgentPoolProfile.Distro = "aks-ubuntu-containerd-18.04-gen2"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = false
				nbc.EnableNvidia = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NC6s_v3")
			},
		},
	})
}

func Scenario_ubuntu2204(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using the Ubuntu 2204 VHD can be properly bootstrapped",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.AgentPoolProfile.Distro = "aks-ubuntu-containerd-22.04-gen2"
			},
		},
	})
}

func Scenario_ubuntu2204AirGap(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using the Ubuntu 2204 VHD and is airgap can be properly bootstrapped",
		Tags: Tags{
			Airgap: true,
		},
		Config: Config{
			Cluster: ClusterKubenetAirgap,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.AgentPoolProfile.Distro = "aks-ubuntu-containerd-22.04-gen2"
			},
		},
	})
}

func Scenario_ubuntu2204ARM64(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that an Ubuntu 2204 Node using ARM64 architecture can be properly bootstrapped",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Arm64Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_D2pds_V5"
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-arm64-containerd-22.04-gen2"
				// This needs to be set based on current CSE implementation...
				nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.CustomKubeBinaryURL = "https://acs-mirror.azureedge.net/kubernetes/v1.24.9/binaries/kubernetes-node-linux-arm64.tar.gz"
				nbc.AgentPoolProfile.VMSize = "Standard_D2pds_V5"
				nbc.AgentPoolProfile.Distro = "aks-ubuntu-arm64-containerd-22.04-gen2"
				nbc.IsARM64 = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_D2pds_V5")
			},
		},
	})
}

func Scenario_ubuntu2204ArtifactStreaming(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "tests that a new ubuntu 2204 node using artifact streaming can be properly bootstrapepd",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.EnableArtifactStreaming = true
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.AgentPoolProfile.Distro = "aks-ubuntu-containerd-22.04-gen2"
			},
			LiveVMValidators: []*LiveVMValidator{
				NonEmptyDirectoryValidator("/etc/overlaybd"),
			},
		},
	})
}

func Scenario_ubuntu2204ChronyRestarts(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that the chrony service restarts if it is killed",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.AgentPoolProfile.Distro = "aks-ubuntu-containerd-22.04-gen2"
			},
			LiveVMValidators: []*LiveVMValidator{
				ServiceCanRestartValidator("chronyd", 10),
				FileHasContentsValidator("/etc/systemd/system/chronyd.service.d/10-chrony-restarts.conf", "Restart=always"),
				FileHasContentsValidator("/etc/systemd/system/chronyd.service.d/10-chrony-restarts.conf", "RestartSec=5"),
			},
		},
	})
}

func Scenario_ubuntu2204CustomCATrust(t *testing.T) {
	const encodedTestCert = "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUgvVENDQmVXZ0F3SUJBZ0lRYUJZRTMvTTA4WEhZQ25OVm1jRkJjakFOQmdrcWhraUc5dzBCQVFzRkFEQnkKTVFzd0NRWURWUVFHRXdKVlV6RU9NQXdHQTFVRUNBd0ZWR1Y0WVhNeEVEQU9CZ05WQkFjTUIwaHZkWE4wYjI0eApFVEFQQmdOVkJBb01DRk5UVENCRGIzSndNUzR3TEFZRFZRUUREQ1ZUVTB3dVkyOXRJRVZXSUZOVFRDQkpiblJsCmNtMWxaR2xoZEdVZ1EwRWdVbE5CSUZJek1CNFhEVEl3TURRd01UQXdOVGd6TTFvWERUSXhNRGN4TmpBd05UZ3oKTTFvd2diMHhDekFKQmdOVkJBWVRBbFZUTVE0d0RBWURWUVFJREFWVVpYaGhjekVRTUE0R0ExVUVCd3dIU0c5MQpjM1J2YmpFUk1BOEdBMVVFQ2d3SVUxTk1JRU52Y25BeEZqQVVCZ05WQkFVVERVNVdNakF3T0RFMk1UUXlORE14CkZEQVNCZ05WQkFNTUMzZDNkeTV6YzJ3dVkyOXRNUjB3R3dZRFZRUVBEQlJRY21sMllYUmxJRTl5WjJGdWFYcGgKZEdsdmJqRVhNQlVHQ3lzR0FRUUJnamM4QWdFQ0RBWk9aWFpoWkdFeEV6QVJCZ3NyQmdFRUFZSTNQQUlCQXhNQwpWVk13Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRREhoZVJrYmIxRkNjN3hSS3N0CndLMEpJR2FLWTh0N0piUzJiUTJiNllJSkRnbkh1SVlIcUJyQ1VWNzlvZWxpa2tva1JrRnZjdnBhS2luRkhEUUgKVXBXRUk2UlVFUlltU0NnM084V2k0MnVPY1YyQjVaYWJtWENrd2R4WTVFY2w1MUJiTThVbkdkb0FHYmRObWlSbQpTbVRqY3MrbGhNeGc0ZkZZNmxCcGlFVkZpR1VqR1JSKzYxUjY3THo2VTRLSmVMTmNDbTA3UXdGWUtCbXBpMDhnCmR5Z1N2UmRVdzU1Sm9wcmVkaitWR3RqVWtCNGhGVDRHUVgvZ2h0NjlSbHF6Lys4dTBkRVFraHVVdXVjcnFhbG0KU0d5NDNIUndCZkRLRndZZVdNN0NQTWQ1ZS9kTyt0MDh0OFBianpWVFR2NWhRRENzRVlJVjJUN0FGSTlTY054TQpraDcvQWdNQkFBR2pnZ05CTUlJRFBUQWZCZ05WSFNNRUdEQVdnQlMvd1ZxSC95ajZRVDM5dDAva0hhK2dZVmdwCnZUQi9CZ2dyQmdFRkJRY0JBUVJ6TUhFd1RRWUlLd1lCQlFVSE1BS0dRV2gwZEhBNkx5OTNkM2N1YzNOc0xtTnYKYlM5eVpYQnZjMmwwYjNKNUwxTlRUR052YlMxVGRXSkRRUzFGVmkxVFUwd3RVbE5CTFRRd09UWXRVak11WTNKMApNQ0FHQ0NzR0FRVUZCekFCaGhSb2RIUndPaTh2YjJOemNITXVjM05zTG1OdmJUQWZCZ05WSFJFRUdEQVdnZ3QzCmQzY3VjM05zTG1OdmJZSUhjM05zTG1OdmJUQmZCZ05WSFNBRVdEQldNQWNHQldlQkRBRUJNQTBHQ3lxRWFBR0cKOW5jQ0JRRUJNRHdHRENzR0FRUUJncWt3QVFNQkJEQXNNQ29HQ0NzR0FRVUZCd0lCRmg1b2RIUndjem92TDNkMwpkeTV6YzJ3dVkyOXRMM0psY0c5emFYUnZjbmt3SFFZRFZSMGxCQll3RkFZSUt3WUJCUVVIQXdJR0NDc0dBUVVGCkJ3TUJNRWdHQTFVZEh3UkJNRDh3UGFBN29EbUdOMmgwZEhBNkx5OWpjbXh6TG5OemJDNWpiMjB2VTFOTVkyOXQKTFZOMVlrTkJMVVZXTFZOVFRDMVNVMEV0TkRBNU5pMVNNeTVqY213d0hRWURWUjBPQkJZRUZBREFGVUlhenc1cgpaSUhhcG5SeElVbnB3K0dMTUE0R0ExVWREd0VCL3dRRUF3SUZvRENDQVgwR0Npc0dBUVFCMW5rQ0JBSUVnZ0Z0CkJJSUJhUUZuQUhjQTlseVVMOUYzTUNJVVZCZ0lNSlJXanVOTkV4a3p2OThNTHlBTHpFN3haT01BQUFGeE0waG8KYndBQUJBTUFTREJHQWlFQTZ4ZWxpTlI4R2svNjNwWWRuUy92T3gvQ2pwdEVNRXY4OVdXaDEvdXJXSUVDSVFEeQpCcmVIVTI1RHp3dWtRYVJRandXNjU1WkxrcUNueGJ4UVdSaU9lbWo5SkFCMUFKUWd2QjZPMVkxc2lITWZnb3NpCkxBM1IyazFlYkUrVVBXSGJUaTlZVGFMQ0FBQUJjVE5JYU53QUFBUURBRVl3UkFJZ0dSRTR3emFiTlJkRDhrcS8KdkZQM3RRZTJobTB4NW5YdWxvd2g0SWJ3M2xrQ0lGWWIvM2xTRHBsUzdBY1I0citYcFd0RUtTVEZXSm1OQ1JiYwpYSnVyMlJHQkFIVUE3c0NWN28xeVpBK1M0OE81RzhjU28ybHFDWHRMYWhvVU9PWkhzc3Z0eGZrQUFBRnhNMGhvCjh3QUFCQU1BUmpCRUFpQjZJdmJvV3NzM1I0SXRWd2plYmw3RDN5b0ZhWDBORGgyZFdoaGd3Q3hySHdJZ0NmcTcKb2NNQzV0KzFqaTVNNXhhTG1QQzRJK1dYM0kvQVJrV1N5aU83SVFjd0RRWUpLb1pJaHZjTkFRRUxCUUFEZ2dJQgpBQ2V1dXI0UW51anFtZ3VTckhVM21oZitjSm9kelRRTnFvNHRkZStQRDEvZUZkWUFFTHU4eEYrMEF0N3hKaVBZCmk1Ukt3aWx5UDU2diszaVkyVDlsdzdTOFRKMDQxVkxoYUlLcDE0TXpTVXpSeWVvT0FzSjdRQURNQ2xIS1VEbEgKVVUycE51bzg4WTZpZ292VDNic253Sk5pRVFOcXltU1NZaGt0dzB0YWR1b3FqcVhuMDZnc1Zpb1dUVkRYeXNkNQpxRXg0dDZzSWdJY01tMjZZSDF2SnBDUUVoS3BjMnkwN2dSa2tsQlpSdE1qVGh2NGNYeXlNWDd1VGNkVDdBSkJQCnVlaWZDb1YyNUp4WHVvOGQ1MTM5Z3dQMUJBZTdJQlZQeDJ1N0tOL1V5T1hkWm13TWYvVG1GR3dEZENmc3lIZi8KWnNCMndMSG96VFlvQVZtUTlGb1UxSkxnY1ZpdnFKK3ZObEJoSFhobHhNZE4wajgwUjlOejZFSWdsUWplSzNPOApJL2NGR20vQjgrNDJoT2xDSWQ5WmR0bmRKY1JKVmppMHdEMHF3ZXZDYWZBOWpKbEh2L2pzRStJOVV6NmNwQ3loCnN3K2xyRmR4VWdxVTU4YXhxZUs4OUZSK05vNHEwSUlPK0ppMXJKS3I5bmtTQjBCcVhvelZuRTFZQi9LTHZkSXMKdVlaSnVxYjJwS2t1K3p6VDZnVXdIVVRadkJpTk90WEw0Tnh3Yy9LVDdXek9TZDJ3UDEwUUk4REtnNHZmaU5EcwpIV21CMWM0S2ppNmdPZ0E1dVNVemFHbXEvdjRWbmNLNVVyK245TGJmbmZMYzI4SjVmdC9Hb3Rpbk15RGszaWFyCkYxMFlscWNPbWVYMXVGbUtiZGkvWG9yR2xrQ29NRjNURHg4cm1wOURCaUIvCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0=" //nolint:lll
	RunScenario(t, &Scenario{
		Description: "Tests that a node using the Ubuntu 2204 VHD can be properly bootstrapped and custom CA was correctly added",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.AgentPoolProfile.Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.CustomCATrustConfig = &datamodel.CustomCATrustConfig{
					CustomCATrustCerts: []string{
						encodedTestCert,
					},
				}
			},
			LiveVMValidators: []*LiveVMValidator{
				NonEmptyDirectoryValidator("/usr/local/share/ca-certificates/certs"),
			},
		},
	})
}

func Scenario_ubuntu2204CustomSysctls(t *testing.T) {
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
				nbc.ContainerService.Properties.AgentPoolProfiles[0].CustomLinuxOSConfig = customLinuxConfig
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.AgentPoolProfile.Distro = "aks-ubuntu-containerd-22.04-gen2"
			},
			LiveVMValidators: []*LiveVMValidator{
				SysctlConfigValidator(customSysctls),
				UlimitValidator(customContainerdUlimits),
			},
		},
	})
}

func Scenario_ubuntu2204gpuncv(t *testing.T) {
	RunScenario(t, ubuntu2204gpu("ubuntu2204-gpu-ncv3", "Standard_NC6s_v3"))
}

func Scenario_ubuntu2204gpua100(t *testing.T) {
	RunScenario(t, ubuntu2204gpu("ubuntu2204-gpu-a100", "Standard_NC24ads_A100_v4"))
}

func Scenario_ubuntu2204gpua10(t *testing.T) {
	RunScenario(t, ubuntu2204gpu("ubuntu2204-gpu-a10", "Standard_NV6ads_A10_v5"))

}

// Returns config for the 'gpu' E2E scenario
func ubuntu2204gpu(name string, vmSize string) *Scenario {
	return &Scenario{
		Description: fmt.Sprintf("Tests that a GPU-enabled node with VM size %s using an Ubuntu 2204 VHD can be properly bootstrapped", vmSize),
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize = vmSize
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.AgentPoolProfile.VMSize = vmSize
				nbc.AgentPoolProfile.Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = false
				nbc.EnableNvidia = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr(vmSize)
			},
		},
	}
}

func Scenario_ubuntu2204GPUGridDriver(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a GPU-enabled node using the Ubuntu 2204 VHD with grid driver can be properly bootstrapped",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.AgentPoolProfile.Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.AgentPoolProfile.VMSize = "Standard_NV6ads_A10_v5"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = false
				nbc.EnableNvidia = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NV6ads_A10_v5")
			},
			LiveVMValidators: []*LiveVMValidator{
				NvidiaSMIInstalledValidator(),
			},
		},
	})
}

func Scenario_ubuntu2204gpuNoDriver(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a GPU-enabled node using the Ubuntu 2204 VHD opting for skipping gpu driver installation can be properly bootstrapped",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.AgentPoolProfile.Distro = "aks-ubuntu-containerd-22.04-gen2"
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
			LiveVMValidators: []*LiveVMValidator{
				NvidiaSMINotInstalledValidator(),
			},
		},
	})
}

// These tests were created to verify that the apt-get call in downloadContainerdFromVersion is not executed.
// The code path is not hit in either of these tests. In the future, testing with some kind of firewall to ensure no egress
// calls are made would be beneficial for airgap testing.

func Scenario_ubuntu2204ContainerdURL(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "tests that a node using the Ubuntu 2204 VHD with the ContainerdPackageURL override bootstraps with the provided URL and not the components.json containerd version",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.AgentPoolProfile.Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.ContainerdPackageURL = "https://packages.microsoft.com/ubuntu/22.04/prod/pool/main/m/moby-containerd/moby-containerd_1.6.9+azure-ubuntu22.04u1_amd64.deb"
			},
			LiveVMValidators: []*LiveVMValidator{
				containerdVersionValidator("1.6.9"),
			},
		},
	})
}

func Scenario_ubuntu2204ContainerdHasCurrentVersion(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "tests that a node using an Ubuntu2204 VHD and the ContainerdVersion override bootstraps with the correct components.json containerd version and ignores the override",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.AgentPoolProfile.Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.ContainerdVersion = "1.6.9"
			},
			LiveVMValidators: []*LiveVMValidator{
				// for containerd we only support one version at a time for each distro/release
				containerdVersionValidator(getExpectedPackageVersions("containerd", "default", "current")[0]),
			},
		},
	})
}

func Scenario_ubuntu2204Wasm(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "tests that a new ubuntu 2204 node using krustlet can be properly bootstrapepd",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].WorkloadRuntime = datamodel.WasmWasi
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.AgentPoolProfile.WorkloadRuntime = datamodel.WasmWasi
				nbc.AgentPoolProfile.Distro = "aks-ubuntu-containerd-22.04-gen2"
			},
		},
	})
}

/*
We could figure out the expected versions by parsing the components.json file, but then we'd need logic
to resolve the right version based on the distro. It seems easier to have the versions of containerd and runc hard coded here because:

* Having additional code for parsing the files and resolving the version introduces risk of bugs in the test code.
* Re-using the parser from prod introduces risk that there's a bug in the parser that we don't pick up as it impacts both test and prod code.
* It's not much effort for a dev to change the hard coded values here and in components.json
*/
func Scenario_marinerv2HasRightContainerdVersion(t *testing.T) {
	Scenario_genericHasRightContainerdVersion(t, config.VHDCBLMarinerV2Gen2, "1.6.26")
}

func Scenario_azurelinuxv2HasRightContainerdVersion(t *testing.T) {
	Scenario_genericHasRightContainerdVersion(t, config.VHDAzureLinuxV2Gen2, "1.6.26")
}

func Scenario_ubuntu2204HasRightContainerdVersion(t *testing.T) {
	Scenario_genericHasRightContainerdVersion(t, config.VHDUbuntu2204Gen2Containerd, "1.7.20-1")
}

func Scenario_ubuntu1804HasRightContainerdVersion(t *testing.T) {
	Scenario_genericHasRightContainerdVersion(t, config.VHDUbuntu1804Gen2Containerd, "1.7.1+azure-1")
}

func Scenario_marinerv2HasRightRuncVersion(t *testing.T) {
	Scenario_genericHasRightRunCVersion(t, config.VHDCBLMarinerV2Gen2, "1.1.9")
}

func Scenario_azurelinuxv2HasRightRuncVersion(t *testing.T) {
	Scenario_genericHasRightRunCVersion(t, config.VHDAzureLinuxV2Gen2, "1.1.9")
}

func Scenario_ubuntu2204HasRightRuncVersion(t *testing.T) {
	Scenario_genericHasRightRunCVersion(t, config.VHDUbuntu2204Gen2Containerd, "1.1.12-1")
}

func Scenario_ubuntu1804HasRightRuncVersion(t *testing.T) {
	Scenario_genericHasRightRunCVersion(t, config.VHDUbuntu1804Gen2Containerd, "1.1.12-1")
}

func Scenario_genericHasRightContainerdVersion(t *testing.T, vhd *config.Image, expectedVersion string) {
	RunScenario(t, &Scenario{
		Description: fmt.Sprintf("Tests that %s has the right containerd version", vhd.String()),
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     vhd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
			},
			LiveVMValidators: []*LiveVMValidator{
				containerdVersionValidator(expectedVersion),
			},
		},
	})
}

func Scenario_genericHasRightRunCVersion(t *testing.T, vhd *config.Image, expectedVersion string) {
	RunScenario(t, &Scenario{
		Description: fmt.Sprintf("Tests that %s has the right runc version", vhd.String()),
		Config: Config{
			Cluster:                ClusterKubenet,
			VHD:                    vhd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {},
			LiveVMValidators: []*LiveVMValidator{
				runcVersionValidator(expectedVersion),
			},
		},
	})
}
