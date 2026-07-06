package e2e

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Azure/agentbaker/e2e/components"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
	"github.com/stretchr/testify/require"
)

func getDCGMPackageNames(os string) []string {
	packages := []string{
		"datacenter-gpu-manager-4-core",
		"datacenter-gpu-manager-4-proprietary",
		"dcgm-exporter",
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

// extractPackageRevision returns the distro rebuild-revision counter from a
// version string. This is the integer that MUST stay in lockstep across OS
// variants of the same package: a Renovate bump is expected to move Ubuntu and
// Azure Linux together, so a divergence here means only one OS was updated.
//
// Examples:
//
//	"4.8.2-ubuntu24.04u2" -> "2"  (Ubuntu PMC rebuild counter)
//	"4.8.2-1.azl3"        -> "1"  (Azure Linux rebuild counter)
//	"1:4.5.3-1"           -> "1"  (handles epoch prefix)
func extractPackageRevision(version string) string {
	// Remove epoch prefix (e.g., "1:" in "1:4.4.1-1")
	version = regexp.MustCompile(`^\d+:`).ReplaceAllString(version, "")

	// The rebuild revision lives in the trailing token after the last "-".
	idx := strings.LastIndex(version, "-")
	if idx == -1 {
		return ""
	}
	rev := version[idx+1:] // e.g. "ubuntu24.04u2", "1.azl3", "1"

	// Ubuntu scheme: "...uN" at the end of the token.
	if m := regexp.MustCompile(`u(\d+)$`).FindStringSubmatch(rev); m != nil {
		return m[1]
	}
	// Azure Linux / plain scheme: leading integer (e.g. "1.azl3", "1").
	if m := regexp.MustCompile(`^(\d+)`).FindStringSubmatch(rev); m != nil {
		return m[1]
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
			{"dcgm-exporter", "ubuntu", "r2404"},
			{"dcgm-exporter", "ubuntu", "r2204"},
			{"dcgm-exporter", "azurelinux", "v3.0"},
		},
	}

	for _, packageGroup := range allPackageVariants {
		expectedVersion := ""
		expectedRevision := ""
		for _, pkgVar := range packageGroup {
			componentVersions := components.GetExpectedPackageVersions(pkgVar.pkgName, pkgVar.osName, pkgVar.osRelease)
			require.Lenf(t, componentVersions, 1,
				"Expected exactly one %s version for %s %s but got %d",
				pkgVar.pkgName, pkgVar.osName, pkgVar.osRelease, len(componentVersions))

			pkgVersion := extractMajorMinorPatchVersion(componentVersions[0])
			require.NotEmptyf(t, pkgVersion, "Failed to extract major.minor.patch version from %s for %s %s",
				componentVersions[0], pkgVar.osName, pkgVar.osRelease)

			pkgRevision := extractPackageRevision(componentVersions[0])
			require.NotEmptyf(t, pkgRevision, "Failed to extract rebuild revision from %s for %s %s",
				componentVersions[0], pkgVar.osName, pkgVar.osRelease)

			// For the first iteration, set the expected values
			if expectedVersion == "" {
				expectedVersion = pkgVersion
				expectedRevision = pkgRevision
				continue
			}
			require.Equalf(t, expectedVersion, pkgVersion,
				"Expected all %s versions to have the same major.minor.patch version, but found mismatch: %s vs %s for %s.%s",
				pkgVar.pkgName, expectedVersion, pkgVersion, pkgVar.osName, pkgVar.osRelease)

			// The trailing rebuild revision must also stay in lockstep across OS variants.
			// A divergence here means Renovate (or a manual bump) updated one OS but not the
			// others - e.g. Ubuntu moved to ...u2 while Azure Linux stayed at ...-1.azl3.
			require.Equalf(t, expectedRevision, pkgRevision,
				"Partial OS update detected for %s: rebuild revision %q (%s.%s) does not match %q from the first OS variant. "+
					"Renovate likely updated one OS but not the others - align ALL OS entries in components.json for this package (or revert the partial bump).",
				pkgVar.pkgName, pkgRevision, pkgVar.osName, pkgVar.osRelease, expectedRevision)
		}
	}
}

func Test_extractPackageRevision(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{input: "4.8.2-ubuntu24.04u2", expected: "2"},
		{input: "4.8.2-ubuntu22.04u2", expected: "2"},
		{input: "4.8.2-1.azl3", expected: "1"},
		{input: "1:4.5.3-1", expected: "1"},
		{input: "0.19.2-ubuntu22.04u10", expected: "10"},
		{input: "4.6.0-3.azl3", expected: "3"},
		{input: "", expected: ""},
		{input: "4.8.2", expected: ""},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("extractPackageRevision(%q)=%q", test.input, test.expected), func(t *testing.T) {
			require.Equal(t, test.expected, extractPackageRevision(test.input))
		})
	}
}

func Test_DCGM_Exporter_Compatibility(t *testing.T) {
	type testCase struct {
		name           string
		vhd            *config.Image
		os             string
		osVersion      string
		description    string
		downloadCmd    string
		extractDepsCmd string
		coreRegex      string
		propRegex      string
	}

	testCases := []testCase{
		{
			name:           "Ubuntu2404",
			vhd:            config.VHDUbuntu2404Gen2Containerd,
			os:             "ubuntu",
			osVersion:      "r2404",
			description:    "Tests that DCGM Exporter is compatible with its dependencies on Ubuntu 24.04 GPU nodes",
			downloadCmd:    "curl -fL --retry 3 --retry-all-errors -o /tmp/dcgm-exporter.deb 'https://packages.microsoft.com/repos/microsoft-ubuntu-noble-prod/pool/main/d/dcgm-exporter/dcgm-exporter_%s_amd64.deb'",
			extractDepsCmd: "dpkg-deb -f /tmp/dcgm-exporter.deb Depends",

			// Parse output like: "..., datacenter-gpu-manager-4-core (= 1:4.4.2-1), datacenter-gpu-manager-4-proprietary (= 1:4.4.2-1), ..."
			coreRegex: `datacenter-gpu-manager-4-core \(= ([^)]+)\)`,
			propRegex: `datacenter-gpu-manager-4-proprietary \(= ([^)]+)\)`,
		},
		{
			name:           "AzureLinux3",
			vhd:            config.VHDAzureLinuxV3Gen2,
			os:             "azurelinux",
			osVersion:      "v3.0",
			description:    "Tests that DCGM Exporter is compatible with its dependencies on Azure Linux 3.0 GPU nodes",
			downloadCmd:    "curl -fL --retry 3 --retry-all-errors -o /tmp/dcgm-exporter.rpm 'https://packages.microsoft.com/azurelinux/3.0/prod/cloud-native/x86_64/Packages/d/dcgm-exporter-%s.x86_64.rpm'",
			extractDepsCmd: "rpm -qpR /tmp/dcgm-exporter.rpm | grep datacenter-gpu-manager",

			// Parse output like: "...\ndatacenter-gpu-manager-4-core = 1:4.5.1-1\ndatacenter-gpu-manager-4-proprietary = 1:4.5.1-1\n..."
			coreRegex: `datacenter-gpu-manager-4-core = (\S+)`,
			propRegex: `datacenter-gpu-manager-4-proprietary = (\S+)`,
		},
	}

	getVersions := func(s *Scenario, tc testCase) (string, string, string) {
		s.T.Helper()

		dcgmExporterVersions := components.GetExpectedPackageVersions("dcgm-exporter", tc.os, tc.osVersion)
		require.Len(s.T, dcgmExporterVersions, 1, "Expected exactly one dcgm-exporter version")
		dcgmExporterVersion := dcgmExporterVersions[0]

		coreVersions := components.GetExpectedPackageVersions("datacenter-gpu-manager-4-core", tc.os, tc.osVersion)
		require.Len(s.T, coreVersions, 1, "Expected exactly one core version")
		expectedCoreVersion := coreVersions[0]

		propVersions := components.GetExpectedPackageVersions("datacenter-gpu-manager-4-proprietary", tc.os, tc.osVersion)
		require.Len(s.T, propVersions, 1, "Expected exactly one proprietary version")
		expectedPropVersion := propVersions[0]

		s.T.Logf("Expected versions from components.json:")
		s.T.Logf("  dcgm-exporter: %s", dcgmExporterVersion)
		s.T.Logf("  datacenter-gpu-manager-4-core: %s", expectedCoreVersion)
		s.T.Logf("  datacenter-gpu-manager-4-proprietary: %s", expectedPropVersion)

		return dcgmExporterVersion, expectedCoreVersion, expectedPropVersion
	}

	parseVersions := func(s *Scenario, tc testCase, cmdLineOutput string) (string, string) {
		s.T.Helper()

		coreRegex := regexp.MustCompile(tc.coreRegex)
		coreMatches := coreRegex.FindStringSubmatch(cmdLineOutput)
		require.Len(s.T, coreMatches, 2, "Failed to extract datacenter-gpu-manager-4-core version from dependencies")
		actualCoreVersion := coreMatches[1]

		propRegex := regexp.MustCompile(tc.propRegex)
		propMatches := propRegex.FindStringSubmatch(cmdLineOutput)
		require.Len(s.T, propMatches, 2, "Failed to extract datacenter-gpu-manager-4-proprietary version from dependencies")
		actualPropVersion := propMatches[1]

		s.T.Logf("Actual versions from dcgm-exporter package:")
		s.T.Logf("  datacenter-gpu-manager-4-core: %s", actualCoreVersion)
		s.T.Logf("  datacenter-gpu-manager-4-proprietary: %s", actualPropVersion)

		return actualCoreVersion, actualPropVersion
	}

	for _, tc := range testCases {
		tc := tc // capture range variable for parallel execution
		t.Run(tc.name, func(t *testing.T) {
			RunScenario(t, &Scenario{
				Description: tc.description,
				Config: Config{
					Cluster:                ClusterKubenet,
					VHD:                    tc.vhd,
					BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {},

					// We are only validating if the package versions are compatible, and for that we need an environment like
					// Ubuntu or Az Linux, and nothing else. This test doesn't care about any other validation.
					SkipDefaultValidation: true,
					Validator: func(ctx context.Context, s *Scenario) {
						// Step 1: Get expected versions from components.json
						dcgmExporterVersion, expectedCoreVersion, expectedPropVersion := getVersions(s, tc)

						// Step 2: Download dcgm-exporter package from PMC
						s.T.Logf("Downloading dcgm-exporter package from PMC...")
						downloadCmd := fmt.Sprintf(tc.downloadCmd, dcgmExporterVersion)
						execScriptOnVMForScenarioValidateExitCode(ctx, s, downloadCmd, 0, "Failed to download dcgm-exporter package")

						// Step 3: Extract dependency versions from the package
						s.T.Logf("Extracting dependency versions from package...")
						result := execScriptOnVMForScenarioValidateExitCode(ctx, s, tc.extractDepsCmd, 0, "Failed to extract dependencies from package")

						dependsOutput := result.stdout
						s.T.Logf("Package dependencies: %s", dependsOutput)

						// Step 4: Parse and verify versions match components.json
						actualCoreVersion, actualPropVersion := parseVersions(s, tc, dependsOutput)

						// Verify versions match
						require.Equalf(s.T, expectedCoreVersion, actualCoreVersion,
							"datacenter-gpu-manager-4-core version mismatch: components.json has %s but dcgm-exporter requires %s",
							expectedCoreVersion, actualCoreVersion)

						require.Equalf(s.T, expectedPropVersion, actualPropVersion,
							"datacenter-gpu-manager-4-proprietary version mismatch: components.json has %s but dcgm-exporter requires %s",
							expectedPropVersion, actualPropVersion)

						s.T.Logf("✅ Version compatibility verified: dcgm-exporter %s is compatible with DCGM packages %s",
							dcgmExporterVersion, expectedCoreVersion)
					},
				},
			})
		})
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
			BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
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
				extension, err := createVMExtensionLinuxAKSNode(t.Context(), vmss.Location)
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
				ValidateNodeAdvertisesGPUResources(ctx, s, 1, "nvidia.com/gpu")

				// Validate that GPU workloads can be scheduled
				ValidateGPUWorkloadSchedulable(ctx, s, 1, "nvidia.com/gpu")

				// Validate that the NVIDIA DCGM packages were installed correctly
				for _, packageName := range getDCGMPackageNames(os) {
					versions := components.GetExpectedPackageVersions(packageName, os, osVersion)
					require.Lenf(s.T, versions, 1, "Expected exactly one %s version for %s %s but got %d", packageName, os, osVersion, len(versions))
					ValidateInstalledPackageVersion(ctx, s, packageName, versions[0])
				}

				ValidateNvidiaDCGMExporterSystemDServiceRunning(ctx, s)
				ValidateNvidiaDCGMExporterIsScrapable(ctx, s)
				ValidateNvidiaDCGMExporterScrapeCommonMetric(ctx, s, "DCGM_FI_DEV_GPU_UTIL")
				ValidateNodeHasLabel(ctx, s, "kubernetes.azure.com/dcgm-exporter", "enabled")

				// Let's run the NPD validation tests to verify that the nvidia
				// device plugin & DCGM services are reporting status correctly
				ValidateNodeProblemDetector(ctx, s)
				// Restart NPD to ensure it picks up the managed GPU experience marker file,
				// which may have been created after NPD's initial startup during provisioning.
				RestartNodeProblemDetector(ctx, s)
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
			BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
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
				extension, err := createVMExtensionLinuxAKSNode(t.Context(), vmss.Location)
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
				ValidateNodeAdvertisesGPUResources(ctx, s, 1, "nvidia.com/gpu")

				// Validate that GPU workloads can be scheduled
				ValidateGPUWorkloadSchedulable(ctx, s, 1, "nvidia.com/gpu")

				for _, packageName := range getDCGMPackageNames(os) {
					versions := components.GetExpectedPackageVersions(packageName, os, osVersion)
					require.Lenf(s.T, versions, 1, "Expected exactly one %s version for %s %s but got %d", packageName, os, osVersion, len(versions))
					ValidateInstalledPackageVersion(ctx, s, packageName, versions[0])
				}

				ValidateNvidiaDCGMExporterSystemDServiceRunning(ctx, s)
				ValidateNvidiaDCGMExporterIsScrapable(ctx, s)
				ValidateNvidiaDCGMExporterScrapeCommonMetric(ctx, s, "DCGM_FI_DEV_GPU_UTIL")
				ValidateNodeHasLabel(ctx, s, "kubernetes.azure.com/dcgm-exporter", "enabled")

				// Let's run the NPD validation tests to verify that the nvidia
				// device plugin & DCGM services are reporting status correctly
				ValidateNodeProblemDetector(ctx, s)
				// Restart NPD to ensure it picks up the managed GPU experience marker file,
				// which may have been created after NPD's initial startup during provisioning.
				RestartNodeProblemDetector(ctx, s)
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
		Location:    "westus2",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV3Gen2,
			BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = "Standard_NC4as_T4_v3"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = true
				nbc.EnableNvidia = true
				nbc.ManagedGPUExperienceAFECEnabled = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NC4as_T4_v3")
				if vmss.Tags == nil {
					vmss.Tags = map[string]*string{}
				}
				vmss.Tags["EnableManagedGPUExperience"] = to.Ptr("true")

				// Enable the AKS VM extension for GPU nodes
				extension, err := createVMExtensionLinuxAKSNode(t.Context(), vmss.Location)
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
				ValidateNodeAdvertisesGPUResources(ctx, s, 1, "nvidia.com/gpu")

				// Validate that GPU workloads can be scheduled
				ValidateGPUWorkloadSchedulable(ctx, s, 1, "nvidia.com/gpu")

				for _, packageName := range getDCGMPackageNames(os) {
					versions := components.GetExpectedPackageVersions(packageName, os, osVersion)
					require.Lenf(s.T, versions, 1, "Expected exactly one %s version for %s %s but got %d", packageName, os, osVersion, len(versions))
					ValidateInstalledPackageVersion(ctx, s, packageName, versions[0])
				}

				ValidateNvidiaDCGMExporterSystemDServiceRunning(ctx, s)
				ValidateNvidiaDCGMExporterIsScrapable(ctx, s)
				ValidateNvidiaDCGMExporterScrapeCommonMetric(ctx, s, "DCGM_FI_DEV_GPU_UTIL")
				ValidateNodeHasLabel(ctx, s, "kubernetes.azure.com/dcgm-exporter", "enabled")

				// Let's run the NPD validation tests to verify that the nvidia
				// device plugin & DCGM services are reporting status correctly
				ValidateNodeProblemDetector(ctx, s)
				// Restart NPD to ensure it picks up the managed GPU experience marker file,
				// which may have been created after NPD's initial startup during provisioning.
				RestartNodeProblemDetector(ctx, s)
				ValidateNPDUnhealthyNvidiaDevicePlugin(ctx, s)
				ValidateNPDUnhealthyNvidiaDevicePluginCondition(ctx, s)
				ValidateNPDUnhealthyNvidiaDevicePluginAfterFailure(ctx, s)
				ValidateNPDUnhealthyNvidiaDCGMServices(ctx, s)
				ValidateNPDUnhealthyNvidiaDCGMServicesCondition(ctx, s)
				ValidateNPDUnhealthyNvidiaDCGMServicesAfterFailure(ctx, s)
			},
		},
	})
}

func Test_Ubuntu2404_NvidiaDevicePluginRunning_MIG(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that NVIDIA device plugin and DCGM Exporter work with MIG enabled on Ubuntu 24.04 GPU nodes",
		Location:    "westus2",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster:               ClusterKubenet,
			VHD:                   config.VHDUbuntu2404Gen2Containerd,
			SkipScriptlessNBC:     true,
			WaitForSSHAfterReboot: 5 * time.Minute,
			BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = "Standard_NC24ads_A100_v4"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = true
				nbc.EnableNvidia = true
				nbc.GPUInstanceProfile = "MIG2g"
				nbc.EnableManagedGPU = true
				nbc.MigStrategy = "Single"
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NC24ads_A100_v4")

				// Enable the AKS VM extension for GPU nodes
				extension, err := createVMExtensionLinuxAKSNode(t.Context(), vmss.Location)
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
				ValidateNodeAdvertisesGPUResources(ctx, s, 3, "nvidia.com/gpu")

				// Validate that MIG workloads can be scheduled
				ValidateGPUWorkloadSchedulable(ctx, s, 3, "nvidia.com/gpu")

				// Validate that the NVIDIA DCGM packages were installed correctly
				for _, packageName := range getDCGMPackageNames(os) {
					versions := components.GetExpectedPackageVersions(packageName, os, osVersion)
					require.Lenf(s.T, versions, 1, "Expected exactly one %s version for %s %s but got %d", packageName, os, osVersion, len(versions))
					ValidateInstalledPackageVersion(ctx, s, packageName, versions[0])
				}

				ValidateNvidiaDCGMExporterSystemDServiceRunning(ctx, s)
				ValidateNvidiaDCGMExporterIsScrapable(ctx, s)
				ValidateNvidiaDCGMExporterScrapeCommonMetric(ctx, s, "DCGM_FI_DEV_GPU_TEMP")
				ValidateNodeHasLabel(ctx, s, "kubernetes.azure.com/dcgm-exporter", "enabled")

				// Let's run the NPD validation tests to verify that the nvidia
				// device plugin & DCGM services are reporting status correctly
				ValidateNodeProblemDetector(ctx, s)
				ValidateNPDUnhealthyNvidiaDevicePlugin(ctx, s)
				ValidateNPDUnhealthyNvidiaDevicePluginCondition(ctx, s)
				ValidateNPDUnhealthyNvidiaDevicePluginAfterFailure(ctx, s)
				ValidateNPDUnhealthyNvidiaDCGMServices(ctx, s)
				ValidateNPDUnhealthyNvidiaDCGMServicesCondition(ctx, s)
				ValidateNPDUnhealthyNvidiaDCGMServicesAfterFailure(ctx, s)
			},
		},
	})
}

func Test_Ubuntu2204_NvidiaDevicePluginRunning_WithoutVMSSTag(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that NVIDIA device plugin and DCGM Exporter work via NBC EnableManagedGPU field without VMSS tag",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster:           ClusterKubenet,
			VHD:               config.VHDUbuntu2204Gen2Containerd,
			SkipScriptlessNBC: true,
			BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = "Standard_NV6ads_A10_v5"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = true
				nbc.EnableNvidia = true
				nbc.ManagedGPUExperienceAFECEnabled = true
				nbc.EnableManagedGPU = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NV6ads_A10_v5")
				// Explicitly DO NOT set the EnableManagedGPUExperience VMSS tag
				// to test that NBC EnableManagedGPU field works independently

				// Enable the AKS VM extension for GPU nodes
				extension, err := createVMExtensionLinuxAKSNode(t.Context(), vmss.Location)
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
				ValidateNodeAdvertisesGPUResources(ctx, s, 1, "nvidia.com/gpu")

				// Validate that GPU workloads can be scheduled
				ValidateGPUWorkloadSchedulable(ctx, s, 1, "nvidia.com/gpu")

				for _, packageName := range getDCGMPackageNames(os) {
					versions := components.GetExpectedPackageVersions(packageName, os, osVersion)
					require.Lenf(s.T, versions, 1, "Expected exactly one %s version for %s %s but got %d", packageName, os, osVersion, len(versions))
					ValidateInstalledPackageVersion(ctx, s, packageName, versions[0])
				}

				ValidateNvidiaDCGMExporterSystemDServiceRunning(ctx, s)
				ValidateNvidiaDCGMExporterIsScrapable(ctx, s)
				ValidateNvidiaDCGMExporterScrapeCommonMetric(ctx, s, "DCGM_FI_DEV_GPU_UTIL")
				ValidateNodeHasLabel(ctx, s, "kubernetes.azure.com/dcgm-exporter", "enabled")

				// Let's run the NPD validation tests to verify that the nvidia
				// device plugin & DCGM services are reporting status correctly
				ValidateNodeProblemDetector(ctx, s)
				// Restart NPD to ensure it picks up the managed GPU experience marker file,
				// which may have been created after NPD's initial startup during provisioning.
				RestartNodeProblemDetector(ctx, s)
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

func Test_CreateVMExtensionLinuxAKSNode_Timing(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// First call — may hit the Azure API or cache
	start := time.Now()
	ext, err := createVMExtensionLinuxAKSNode(t.Context(), nil)
	firstDuration := time.Since(start)
	require.NoError(t, err, "first call to createVMExtensionLinuxAKSNode failed")
	require.NotNil(t, ext, "first call returned nil extension")
	t.Logf("First call duration: %s", firstDuration)

	// Second call — should be served from cache
	start = time.Now()
	ext2, err := createVMExtensionLinuxAKSNode(t.Context(), nil)
	secondDuration := time.Since(start)
	require.NoError(t, err, "second call to createVMExtensionLinuxAKSNode failed")
	require.NotNil(t, ext2, "second call returned nil extension")
	t.Logf("Second call duration: %s", secondDuration)

	// Both calls should return a valid, consistent TypeHandlerVersion
	require.NotNil(t, ext.Properties, "first extension has nil Properties")
	require.NotNil(t, ext2.Properties, "second extension has nil Properties")
	require.NotNil(t, ext.Properties.TypeHandlerVersion, "first TypeHandlerVersion is nil")
	require.NotNil(t, ext2.Properties.TypeHandlerVersion, "second TypeHandlerVersion is nil")
	require.NotEmpty(t, *ext.Properties.TypeHandlerVersion, "first TypeHandlerVersion is empty")
	require.NotEmpty(t, *ext2.Properties.TypeHandlerVersion, "second TypeHandlerVersion is empty")

	// Ensure we actually hit Azure and didn't just get the fallback version
	require.NotEqual(t, "1.413", *ext.Properties.TypeHandlerVersion,
		"extension version is the hardcoded fallback — Azure API may not have been reached")

	// Cache consistency: both calls should return the same version
	require.Equal(t, *ext.Properties.TypeHandlerVersion, *ext2.Properties.TypeHandlerVersion,
		"both calls should return the same extension version")
}

func Test_Ubuntu2404_NvidiaDevicePluginRunning_MIG_Mixed(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that NVIDIA device plugin work with MIG Mixed mode on Ubuntu 24.04 GPU nodes",
		Location:    "westus2",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster:               ClusterKubenet,
			VHD:                   config.VHDUbuntu2404Gen2Containerd,
			WaitForSSHAfterReboot: 5 * time.Minute,
			BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = "Standard_NC24ads_A100_v4"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = true
				nbc.EnableNvidia = true
				nbc.GPUInstanceProfile = "MIG1g"
				nbc.EnableManagedGPU = true
				nbc.MigStrategy = "Mixed"
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NC24ads_A100_v4")

				// Enable the AKS VM extension for GPU nodes
				extension, err := createVMExtensionLinuxAKSNode(t.Context(), vmss.Location)
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
				ValidateMIGInstancesCreated(ctx, s, "MIG 1g.10gb")

				// Validate that MIG profile-specific GPU resources are advertised by the device plugin
				migResourceName := "nvidia.com/mig-1g.10gb"
				ValidateNodeAdvertisesGPUResources(ctx, s, 7, migResourceName)

				// Validate that MIG workloads can be scheduled
				ValidateGPUWorkloadSchedulable(ctx, s, 2, migResourceName)
			},
		},
	})
}
