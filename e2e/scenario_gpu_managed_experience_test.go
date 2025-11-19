package e2e

import (
	"context"
	"regexp"
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

// extractMajorMinorPatchVersion extracts the major.minor.patch version from a
// version string
//
// Examples:
//
//	"4.6.0-1" -> "4.6.0"
//	"4.5.2-1.azl3" -> "4.5.2"
//	"1:4.4.1-1" -> "4.4.1" (handles epoch prefix)
func extractMajorMinorPatchVersion(version string) string {
	// Remove epoch prefix (e.g., "1:" in "1:4.4.1-1")
	version = regexp.MustCompile(`^\d+:`).ReplaceAllString(version, "")

	// Match major.minor.patch pattern
	re := regexp.MustCompile(`^(\d+\.\d+\.\d+)`)
	matches := re.FindStringSubmatch(version)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

type packageOSVariant struct {
	pkgName   string
	osName    string
	osRelease string
}

func Test_Version_Consistency_GPU_Managed_Components(t *testing.T) {
	allPackageVariants := [][]packageOSVariant{
		{
			{"nvidia-device-plugin", "ubuntu", "r2404"},
			{"nvidia-device-plugin", "ubuntu", "r2204"},
			{"nvidia-device-plugin", "azurelinux", "v3.0"},
		},
		{
			{"datacenter-gpu-manager-4-core", "ubuntu", "r2404"},
			{"datacenter-gpu-manager-4-core", "ubuntu", "r2204"},
			{"datacenter-gpu-manager-4-core", "azurelinux", "v3.0"},
		},
		{
			{"datacenter-gpu-manager-4-proprietary", "ubuntu", "r2404"},
			{"datacenter-gpu-manager-4-proprietary", "ubuntu", "r2204"},
			{"datacenter-gpu-manager-4-proprietary", "azurelinux", "v3.0"},
		},
		{
			{"datacenter-gpu-manager-exporter", "ubuntu", "r2404"},
			{"datacenter-gpu-manager-exporter", "ubuntu", "r2204"},
			{"dcgm-exporter", "azurelinux", "v3.0"},
		},
	}

	for _, packageGroup := range allPackageVariants {
		expectedVersion := ""
		for _, pkgVar := range packageGroup {
			componentVersions := components.GetExpectedPackageVersions(pkgVar.pkgName, pkgVar.osName, pkgVar.osRelease)
			require.Lenf(t, componentVersions, 1,
				"Expected exactly one %s version for %s %s but got %d",
				pkgVar.pkgName, pkgVar.osName, pkgVar.osRelease, len(componentVersions))

			pkgVersion := extractMajorMinorPatchVersion(componentVersions[0])
			require.NotEmptyf(t, pkgVersion, "Failed to extract major.minor.patch version from %s for %s %s",
				componentVersions[0], pkgVar.osName, pkgVar.osRelease)

			// For the first iteration, set the expectedVersion
			if expectedVersion == "" {
				expectedVersion = pkgVersion
				continue
			}
			require.Equalf(t, expectedVersion, pkgVersion,
				"Expected all %s versions to have the same major.minor.patch version, but found mismatch: %s vs %s for %s.%s",
				pkgVar.pkgName, expectedVersion, pkgVersion, pkgVar.osName, pkgVar.osRelease)
		}
	}
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
				nbc.AgentPoolProfile.VMSize = "Standard_NV6ads_A10_v5"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = true
				nbc.EnableNvidia = true
				nbc.ManagedGPUExperienceAFECEnabled = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NV6ads_A10_v5")
				if vmss.Tags == nil {
					vmss.Tags = map[string]*string{}
				}
				vmss.Tags["EnableManagedGPUExperience"] = to.Ptr("true")

				// Enable the AKS VM extension for GPU nodes
				extension, err := createVMExtensionLinuxAKSNode(vmss.Location)
				require.NoError(t, err, "creating AKS VM extension")
				vmss.Properties = addVMExtensionToVMSS(vmss.Properties, extension)
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

				// Let's run the NPD validation tests to verify that the nvidia
				// device plugin & DCGM services are reporting status correctly
				ValidateNodeProblemDetector(ctx, s)
				ValidateNPDUnhealthyNvidiaDevicePlugin(ctx, s)
				ValidateNPDUnhealthyNvidiaDevicePluginCondition(ctx, s)
				ValidateNPDUnhealthyNvidiaDevicePluginAfterFailure(ctx, s)
				ValidateNPDUnhealthyNvidiaDCGMServices(ctx, s)
				ValidateNPDUnhealthyNvidiaDCGMServicesCondition(ctx, s)
				ValidateNPDUnhealthyNvidiaDCGMServicesAfterFailure(ctx, s)
				// verify nvidia grid license status checks are reporting status correctly
				ValidateNPDHealthyNvidiaGridLicenseStatus(ctx, s)
				ValidateNPDUnhealthyNvidiaGridLicenseStatusAfterFailure(ctx, s)
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
				nbc.AgentPoolProfile.VMSize = "Standard_NV6ads_A10_v5"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = true
				nbc.EnableNvidia = true
				nbc.ManagedGPUExperienceAFECEnabled = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NV6ads_A10_v5")
				if vmss.Tags == nil {
					vmss.Tags = map[string]*string{}
				}
				vmss.Tags["EnableManagedGPUExperience"] = to.Ptr("true")

				// Enable the AKS VM extension for GPU nodes
				extension, err := createVMExtensionLinuxAKSNode(vmss.Location)
				require.NoError(t, err, "creating AKS VM extension")
				vmss.Properties = addVMExtensionToVMSS(vmss.Properties, extension)
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

				// Let's run the NPD validation tests to verify that the nvidia
				// device plugin & DCGM services are reporting status correctly
				ValidateNodeProblemDetector(ctx, s)
				ValidateNPDUnhealthyNvidiaDevicePlugin(ctx, s)
				ValidateNPDUnhealthyNvidiaDevicePluginCondition(ctx, s)
				ValidateNPDUnhealthyNvidiaDevicePluginAfterFailure(ctx, s)
				ValidateNPDUnhealthyNvidiaDCGMServices(ctx, s)
				ValidateNPDUnhealthyNvidiaDCGMServicesCondition(ctx, s)
				ValidateNPDUnhealthyNvidiaDCGMServicesAfterFailure(ctx, s)
				// verify nvidia grid license status checks are reporting status correctly
				ValidateNPDHealthyNvidiaGridLicenseStatus(ctx, s)
				ValidateNPDUnhealthyNvidiaGridLicenseStatusAfterFailure(ctx, s)
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
				nbc.AgentPoolProfile.VMSize = "Standard_NV6ads_A10_v5"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = true
				nbc.EnableNvidia = true
				nbc.ManagedGPUExperienceAFECEnabled = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NV6ads_A10_v5")
				if vmss.Tags == nil {
					vmss.Tags = map[string]*string{}
				}
				vmss.Tags["EnableManagedGPUExperience"] = to.Ptr("true")

				// Enable the AKS VM extension for GPU nodes
				extension, err := createVMExtensionLinuxAKSNode(vmss.Location)
				require.NoError(t, err, "creating AKS VM extension")
				vmss.Properties = addVMExtensionToVMSS(vmss.Properties, extension)
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

				// Let's run the NPD validation tests to verify that the nvidia
				// device plugin & DCGM services are reporting status correctly
				ValidateNodeProblemDetector(ctx, s)
				ValidateNPDUnhealthyNvidiaDevicePlugin(ctx, s)
				ValidateNPDUnhealthyNvidiaDevicePluginCondition(ctx, s)
				ValidateNPDUnhealthyNvidiaDevicePluginAfterFailure(ctx, s)
				ValidateNPDUnhealthyNvidiaDCGMServices(ctx, s)
				ValidateNPDUnhealthyNvidiaDCGMServicesCondition(ctx, s)
				ValidateNPDUnhealthyNvidiaDCGMServicesAfterFailure(ctx, s)
				// verify nvidia grid license status checks are reporting status correctly
				ValidateNPDHealthyNvidiaGridLicenseStatus(ctx, s)
				ValidateNPDUnhealthyNvidiaGridLicenseStatusAfterFailure(ctx, s)
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

				// Enable the AKS VM extension for GPU nodes
				extension, err := createVMExtensionLinuxAKSNode(vmss.Location)
				require.NoError(t, err, "creating AKS VM extension")
				vmss.Properties = addVMExtensionToVMSS(vmss.Properties, extension)
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

				// Let's run the NPD validation tests to verify that the nvidia
				// device plugin & DCGM services are reporting status correctly
				ValidateNodeProblemDetector(ctx, s)
				ValidateNPDUnhealthyNvidiaDevicePlugin(ctx, s)
				ValidateNPDUnhealthyNvidiaDevicePluginCondition(ctx, s)
				ValidateNPDUnhealthyNvidiaDevicePluginAfterFailure(ctx, s)
				ValidateNPDUnhealthyNvidiaDCGMServices(ctx, s)
				ValidateNPDUnhealthyNvidiaDCGMServicesCondition(ctx, s)
				ValidateNPDUnhealthyNvidiaDCGMServicesAfterFailure(ctx, s)
				// verify nvidia grid license status checks are reporting status correctly
				ValidateNPDHealthyNvidiaGridLicenseStatus(ctx, s)
				ValidateNPDUnhealthyNvidiaGridLicenseStatusAfterFailure(ctx, s)
			},
		},
	})
}
