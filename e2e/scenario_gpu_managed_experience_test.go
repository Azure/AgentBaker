package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/agentbaker/e2e/components"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/stretchr/testify/require"
)

func getDCGMPackageNames(os string) []string {
	packages := []string{
		"datacenter-gpu-manager-4-core",
		"datacenter-gpu-manager-4-proprietary",
	}

	switch os {
	case "ubuntu":
		packages = append(packages, "datacenter-gpu-manager-exporter")
	case "azurelinux":
		packages = append(packages, "dcgm-exporter")
	}

	return packages
}

func Test_Ubuntu2404_NvidiaDevicePluginRunning(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that NVIDIA device plugin and DCGM Exporter are running & functional on Ubuntu 24.04 GPU nodes",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2404Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = "Standard_NC6s_v3"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = true
				nbc.EnableNvidia = true
				nbc.ManagedGPUExperienceAFECEnabled = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NC6s_v3")
				if vmss.Tags == nil {
					vmss.Tags = map[string]*string{}
				}
				vmss.Tags["EnableManagedGPUExperience"] = to.Ptr("true")
			},
			Validator: func(ctx context.Context, s *Scenario) {
				os := "ubuntu"
				osVersion := "r2404"

				// Validate that the NVIDIA device plugin binary was installed correctly
				versions := components.GetExpectedPackageVersions("nvidia-device-plugin", os, osVersion)
				require.Lenf(s.T, versions, 1, "Expected exactly one nvidia-device-plugin version for %s %s but got %d", os, osVersion, len(versions))
				ValidateInstalledPackageVersion(ctx, s, "nvidia-device-plugin", versions[0])

				// Validate that the NVIDIA device plugin systemd service is running
				ValidateNvidiaDevicePluginServiceRunning(ctx, s)

				// Validate that GPU resources are advertised by the device plugin
				ValidateNodeAdvertisesGPUResources(ctx, s, 1)

				// Validate that GPU workloads can be scheduled
				ValidateGPUWorkloadSchedulable(ctx, s, 1)

				// Validate that the NVIDIA DCGM packages were installed correctly
				for _, packageName := range getDCGMPackageNames(os) {
					versions := components.GetExpectedPackageVersions(packageName, os, osVersion)
					require.Lenf(s.T, versions, 1, "Expected exactly one %s version for %s %s but got %d", packageName, os, osVersion, len(versions))
					ValidateInstalledPackageVersion(ctx, s, packageName, versions[0])
				}

				ValidateNvidiaDCGMExporterSystemDServiceRunning(ctx, s)
				ValidateNvidiaDCGMExporterIsScrapable(ctx, s)
				ValidateNvidiaDCGMExporterScrapeCommonMetric(ctx, s, "DCGM_FI_DEV_GPU_UTIL")
			},
		},
	})
}

func Test_Ubuntu2204_NvidiaDevicePluginRunning(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that NVIDIA device plugin and DCGM Exporter are running & functional on Ubuntu 22.04 GPU nodes",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = "Standard_NC6s_v3"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = true
				nbc.EnableNvidia = true
				nbc.ManagedGPUExperienceAFECEnabled = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NC6s_v3")
				if vmss.Tags == nil {
					vmss.Tags = map[string]*string{}
				}
				vmss.Tags["EnableManagedGPUExperience"] = to.Ptr("true")
			},
			Validator: func(ctx context.Context, s *Scenario) {
				os := "ubuntu"
				osVersion := "r2204"

				// Validate that the NVIDIA device plugin binary was installed correctly
				versions := components.GetExpectedPackageVersions("nvidia-device-plugin", os, osVersion)
				require.Lenf(s.T, versions, 1, "Expected exactly one nvidia-device-plugin version for %s %s but got %d", os, osVersion, len(versions))
				ValidateInstalledPackageVersion(ctx, s, "nvidia-device-plugin", versions[0])

				// Validate that the NVIDIA device plugin systemd service is running
				ValidateNvidiaDevicePluginServiceRunning(ctx, s)

				// Validate that GPU resources are advertised by the device plugin
				ValidateNodeAdvertisesGPUResources(ctx, s, 1)

				// Validate that GPU workloads can be scheduled
				ValidateGPUWorkloadSchedulable(ctx, s, 1)

				for _, packageName := range getDCGMPackageNames(os) {
					versions := components.GetExpectedPackageVersions(packageName, os, osVersion)
					require.Lenf(s.T, versions, 1, "Expected exactly one %s version for %s %s but got %d", packageName, os, osVersion, len(versions))
					ValidateInstalledPackageVersion(ctx, s, packageName, versions[0])
				}

				ValidateNvidiaDCGMExporterSystemDServiceRunning(ctx, s)
				ValidateNvidiaDCGMExporterIsScrapable(ctx, s)
				ValidateNvidiaDCGMExporterScrapeCommonMetric(ctx, s, "DCGM_FI_DEV_GPU_UTIL")
			},
		},
	})
}

func Test_AzureLinux3_NvidiaDevicePluginRunning(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that NVIDIA device plugin and DCGM Exporter are running & functional on Azure Linux v3 GPU nodes",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV3Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = "Standard_NC6s_v3"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = true
				nbc.EnableNvidia = true
				nbc.ManagedGPUExperienceAFECEnabled = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NC6s_v3")
				if vmss.Tags == nil {
					vmss.Tags = map[string]*string{}
				}
				vmss.Tags["EnableManagedGPUExperience"] = to.Ptr("true")
			},
			Validator: func(ctx context.Context, s *Scenario) {
				os := "azurelinux"
				osVersion := "v3.0"

				// Validate that the NVIDIA device plugin binary was installed correctly
				versions := components.GetExpectedPackageVersions("nvidia-device-plugin", os, osVersion)
				require.Lenf(s.T, versions, 1, "Expected exactly one nvidia-device-plugin version for %s %s but got %d", os, osVersion, len(versions))
				ValidateInstalledPackageVersion(ctx, s, "nvidia-device-plugin", versions[0])

				// Validate that the NVIDIA device plugin systemd service is running
				ValidateNvidiaDevicePluginServiceRunning(ctx, s)

				// Validate that GPU resources are advertised by the device plugin
				ValidateNodeAdvertisesGPUResources(ctx, s, 1)

				// Validate that GPU workloads can be scheduled
				ValidateGPUWorkloadSchedulable(ctx, s, 1)

				for _, packageName := range getDCGMPackageNames(os) {
					versions := components.GetExpectedPackageVersions(packageName, os, osVersion)
					require.Lenf(s.T, versions, 1, "Expected exactly one %s version for %s %s but got %d", packageName, os, osVersion, len(versions))
					ValidateInstalledPackageVersion(ctx, s, packageName, versions[0])
				}

				ValidateNvidiaDCGMExporterSystemDServiceRunning(ctx, s)
				ValidateNvidiaDCGMExporterIsScrapable(ctx, s)
				ValidateNvidiaDCGMExporterScrapeCommonMetric(ctx, s, "DCGM_FI_DEV_GPU_UTIL")
			},
		},
	})
}

func Test_Ubuntu2404_NvidiaDevicePluginRunning_MIG(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that NVIDIA device plugin and DCGM Exporter work with MIG enabled on Ubuntu 24.04 GPU nodes",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster:               ClusterKubenet,
			VHD:                   config.VHDUbuntu2404Gen2Containerd,
			WaitForSSHAfterReboot: 5 * time.Minute,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = "Standard_NC24ads_A100_v4"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = true
				nbc.EnableNvidia = true
				nbc.GPUInstanceProfile = "MIG2g"
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NC24ads_A100_v4")
				if vmss.Tags == nil {
					vmss.Tags = map[string]*string{}
				}
				vmss.Tags["EnableManagedGPUExperience"] = to.Ptr("true")
			},
			Validator: func(ctx context.Context, s *Scenario) {
				os := "ubuntu"
				osVersion := "r2404"

				// Validate that the NVIDIA device plugin binary was installed correctly
				versions := components.GetExpectedPackageVersions("nvidia-device-plugin", os, osVersion)
				require.Lenf(s.T, versions, 1, "Expected exactly one nvidia-device-plugin version for %s %s but got %d", os, osVersion, len(versions))
				ValidateInstalledPackageVersion(ctx, s, "nvidia-device-plugin", versions[0])

				// Validate that the NVIDIA device plugin systemd service is running
				ValidateNvidiaDevicePluginServiceRunning(ctx, s)

				// Validate that MIG mode is enabled via nvidia-smi
				ValidateMIGModeEnabled(ctx, s)

				// Validate that MIG instances are created
				ValidateMIGInstancesCreated(ctx, s, "MIG 2g.20gb")

				// Validate that GPU resources are advertised by the device plugin
				ValidateNodeAdvertisesGPUResources(ctx, s, 3)

				// Validate that MIG workloads can be scheduled
				ValidateGPUWorkloadSchedulable(ctx, s, 3)

				// Validate that the NVIDIA DCGM packages were installed correctly
				for _, packageName := range getDCGMPackageNames(os) {
					versions := components.GetExpectedPackageVersions(packageName, os, osVersion)
					require.Lenf(s.T, versions, 1, "Expected exactly one %s version for %s %s but got %d", packageName, os, osVersion, len(versions))
					ValidateInstalledPackageVersion(ctx, s, packageName, versions[0])
				}

				ValidateNvidiaDCGMExporterSystemDServiceRunning(ctx, s)
				ValidateNvidiaDCGMExporterIsScrapable(ctx, s)
				ValidateNvidiaDCGMExporterScrapeCommonMetric(ctx, s, "DCGM_FI_DEV_GPU_TEMP")
			},
		},
	})
}
