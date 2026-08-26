package e2e

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/assert"
	"github.com/Azure/agentbaker/e2e/components"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
)

func getDCGMPackageNames(os string) []string {
	packages := []string{
		"datacenter-gpu-manager-4-core",
		"datacenter-gpu-manager-4-proprietary",
		"dcgm-exporter",
	}

	return packages
}

// expectedPackageVersion returns the single version components.json pins for the
// package on the given OS. Anything other than exactly one entry is a bug in
// components.json, and callers cannot continue without the version string.
func expectedPackageVersion(packageName, os, osVersion string) (string, error) {
	versions := components.GetExpectedPackageVersions(packageName, os, osVersion)
	if err := assert.Equal(len(versions), 1, "expected exactly one %s version for %s %s but got %d", packageName, os, osVersion, len(versions)); err != nil {
		return "", err
	}
	return versions[0], nil
}

// validateDCGMPackageVersions checks that every DCGM package pinned for the OS is
// the version actually installed on the node.
func validateDCGMPackageVersions(ctx context.Context, s *Scenario, os, osVersion string) error {
	var errs []error
	for _, packageName := range getDCGMPackageNames(os) {
		version, err := expectedPackageVersion(packageName, os, osVersion)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		errs = append(errs, ValidateInstalledPackageVersion(ctx, s, packageName, version))
	}
	return errors.Join(errs...)
}

// validateNPDNvidiaConditions runs the NPD device plugin and DCGM checks. Each step
// depends on the node state left behind by the previous one - the *AfterFailure
// checks deliberately break a service and then repair it - so the sequence stops at
// the first failure instead of injecting more faults onto an already broken node.
func validateNPDNvidiaConditions(ctx context.Context, s *Scenario) error {
	if err := ValidateNPDUnhealthyNvidiaDevicePlugin(ctx, s); err != nil {
		return err
	}
	if err := ValidateNPDUnhealthyNvidiaDevicePluginCondition(ctx, s); err != nil {
		return err
	}
	if err := ValidateNPDUnhealthyNvidiaDevicePluginAfterFailure(ctx, s); err != nil {
		return err
	}
	if err := ValidateNPDUnhealthyNvidiaDCGMServices(ctx, s); err != nil {
		return err
	}
	if err := ValidateNPDUnhealthyNvidiaDCGMServicesCondition(ctx, s); err != nil {
		return err
	}
	return ValidateNPDUnhealthyNvidiaDCGMServicesAfterFailure(ctx, s)
}

// validateNPDNvidiaGridLicense verifies the grid license status is reported as
// healthy before the failure is simulated, so the checks run in order.
func validateNPDNvidiaGridLicense(ctx context.Context, s *Scenario) error {
	if err := ValidateNPDHealthyNvidiaGridLicenseStatus(ctx, s); err != nil {
		return err
	}
	return ValidateNPDUnhealthyNvidiaGridLicenseStatusAfterFailure(ctx, s)
}

// validateDCGMExporterRunning checks the DCGM exporter service is up, scrapable and
// advertised through the node label.
func validateDCGMExporterRunning(ctx context.Context, s *Scenario, metric string) error {
	if err := ValidateNvidiaDCGMExporterSystemDServiceRunning(ctx, s); err != nil {
		return err
	}
	// Scraping only makes sense once the exporter endpoint answers.
	if err := ValidateNvidiaDCGMExporterIsScrapable(ctx, s); err != nil {
		return err
	}
	return errors.Join(
		ValidateNvidiaDCGMExporterScrapeCommonMetric(ctx, s, metric),
		ValidateNodeHasLabel(ctx, s, "kubernetes.azure.com/dcgm-exporter", "enabled"),
	)
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

var _ = registerDCGMExporterCompatibilityScenarios()

func registerDCGMExporterCompatibilityScenarios() bool {
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

	getVersions := func(s *Scenario, tc testCase) (string, string, string, error) {
		dcgmExporterVersion, err := expectedPackageVersion("dcgm-exporter", tc.os, tc.osVersion)
		if err != nil {
			return "", "", "", err
		}
		expectedCoreVersion, err := expectedPackageVersion("datacenter-gpu-manager-4-core", tc.os, tc.osVersion)
		if err != nil {
			return "", "", "", err
		}
		expectedPropVersion, err := expectedPackageVersion("datacenter-gpu-manager-4-proprietary", tc.os, tc.osVersion)
		if err != nil {
			return "", "", "", err
		}

		s.Logger.Logf("Expected versions from components.json:")
		s.Logger.Logf("  dcgm-exporter: %s", dcgmExporterVersion)
		s.Logger.Logf("  datacenter-gpu-manager-4-core: %s", expectedCoreVersion)
		s.Logger.Logf("  datacenter-gpu-manager-4-proprietary: %s", expectedPropVersion)

		return dcgmExporterVersion, expectedCoreVersion, expectedPropVersion, nil
	}

	parseVersions := func(s *Scenario, tc testCase, cmdLineOutput string) (string, string, error) {
		coreRegex := regexp.MustCompile(tc.coreRegex)
		coreMatches := coreRegex.FindStringSubmatch(cmdLineOutput)

		propRegex := regexp.MustCompile(tc.propRegex)
		propMatches := propRegex.FindStringSubmatch(cmdLineOutput)

		if err := errors.Join(
			assert.Equal(len(coreMatches), 2, "failed to extract datacenter-gpu-manager-4-core version from dependencies:\n%s", cmdLineOutput),
			assert.Equal(len(propMatches), 2, "failed to extract datacenter-gpu-manager-4-proprietary version from dependencies:\n%s", cmdLineOutput),
		); err != nil {
			return "", "", err
		}
		actualCoreVersion := coreMatches[1]
		actualPropVersion := propMatches[1]

		s.Logger.Logf("Actual versions from dcgm-exporter package:")
		s.Logger.Logf("  datacenter-gpu-manager-4-core: %s", actualCoreVersion)
		s.Logger.Logf("  datacenter-gpu-manager-4-proprietary: %s", actualPropVersion)

		return actualCoreVersion, actualPropVersion, nil
	}

	for _, tc := range testCases {
		Register(&Scenario{
			Name:        "DCGM_Exporter_Compatibility/" + tc.name,
			Description: tc.description,
			Config: Config{
				Cluster: ClusterKubenet,
				VHD:     tc.vhd,

				// We are only validating if the package versions are compatible, and for that we need an environment like
				// Ubuntu or Az Linux, and nothing else. This test doesn't care about any other validation.
				SkipDefaultValidation: true,
				Validator: func(ctx context.Context, s *Scenario) error {
					// Step 1: Get expected versions from components.json
					dcgmExporterVersion, expectedCoreVersion, expectedPropVersion, err := getVersions(s, tc)
					if err != nil {
						return err
					}

					// Step 2: Download dcgm-exporter package from PMC
					s.Logger.Logf("Downloading dcgm-exporter package from PMC...")
					downloadCmd := fmt.Sprintf(tc.downloadCmd, dcgmExporterVersion)
					if _, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, downloadCmd, 0, "Failed to download dcgm-exporter package"); err != nil {
						return err
					}

					// Step 3: Extract dependency versions from the package
					s.Logger.Logf("Extracting dependency versions from package...")
					result, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, tc.extractDepsCmd, 0, "Failed to extract dependencies from package")
					if err != nil {
						return err
					}

					dependsOutput := result.stdout
					s.Logger.Logf("Package dependencies: %s", dependsOutput)

					// Step 4: Parse and verify versions match components.json
					actualCoreVersion, actualPropVersion, err := parseVersions(s, tc, dependsOutput)
					if err != nil {
						return err
					}

					// Verify versions match
					if err := errors.Join(
						assert.Equal(actualCoreVersion, expectedCoreVersion,
							"datacenter-gpu-manager-4-core version mismatch: components.json has %s but dcgm-exporter requires %s",
							expectedCoreVersion, actualCoreVersion),
						assert.Equal(actualPropVersion, expectedPropVersion,
							"datacenter-gpu-manager-4-proprietary version mismatch: components.json has %s but dcgm-exporter requires %s",
							expectedPropVersion, actualPropVersion),
					); err != nil {
						return err
					}

					s.Logger.Logf("✅ Version compatibility verified: dcgm-exporter %s is compatible with DCGM packages %s",
						dcgmExporterVersion, expectedCoreVersion)
					return nil
				},
			},
		})
	}
	return true
}

var _ = Register(&Scenario{
	Name:        "Ubuntu2404_NvidiaDevicePluginRunning",
	Description: "Tests that NVIDIA device plugin and DCGM Exporter work on Ubuntu 24.04 via NBC EnableManagedGPU without a VMSS tag",
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
			nbc.EnableManagedGPU = true
		},
		VMConfigMutatorWithError: func(ctx context.Context, vmss *armcompute.VirtualMachineScaleSet) error {
			vmss.SKU.Name = to.Ptr("Standard_NV6ads_A10_v5")
			// Do not set EnableManagedGPUExperience: this test verifies that
			// the NBC EnableManagedGPU field activates the managed GPU path.

			// Enable the AKS VM extension for GPU nodes
			extension, err := createVMExtensionLinuxAKSNode(ctx, vmss.Location)
			if err != nil {
				return fmt.Errorf("create AKS VM extension: %w", err)
			}
			vmss.Properties = addVMExtensionToVMSS(vmss.Properties, extension)
			return nil
		},
		Validator: func(ctx context.Context, s *Scenario) error {
			os := "ubuntu"
			osVersion := "r2404"

			// Validate that the NVIDIA device plugin binary was installed correctly
			devicePluginVersion, err := expectedPackageVersion("nvidia-device-plugin", os, osVersion)
			if err != nil {
				return err
			}
			if err := errors.Join(
				ValidateInstalledPackageVersion(ctx, s, "nvidia-device-plugin", devicePluginVersion),
				// Validate that the NVIDIA device plugin systemd service is running
				ValidateNvidiaDevicePluginServiceRunning(ctx, s),
			); err != nil {
				return err
			}
			// Resource advertisement depends on the device plugin service.
			if err := ValidateNodeAdvertisesGPUResources(ctx, s, 1, "nvidia.com/gpu"); err != nil {
				return err
			}

			// Validate that GPU workloads can be scheduled. Only meaningful once the GPU
			// resources above are advertised, otherwise the pod simply never gets scheduled.
			if err := ValidateGPUWorkloadSchedulable(ctx, s, 1, "nvidia.com/gpu"); err != nil {
				return err
			}

			// Validate that the NVIDIA DCGM packages were installed correctly
			if err := errors.Join(
				validateDCGMPackageVersions(ctx, s, os, osVersion),
				validateDCGMExporterRunning(ctx, s, "DCGM_FI_DEV_GPU_UTIL"),
			); err != nil {
				return err
			}

			// Let's run the NPD validation tests to verify that the nvidia
			// device plugin & DCGM services are reporting status correctly
			if err := ValidateNodeProblemDetector(ctx, s); err != nil {
				return err
			}
			// Restart NPD to ensure it picks up the managed GPU experience marker file,
			// which may have been created after NPD's initial startup during provisioning.
			if err := RestartNodeProblemDetector(ctx, s); err != nil {
				return err
			}
			if err := validateNPDNvidiaConditions(ctx, s); err != nil {
				return err
			}
			// Verify NVIDIA GRID license status checks are reporting status correctly.
			return validateNPDNvidiaGridLicense(ctx, s)
		},
	},
})

var _ = Register(&Scenario{
	Name:        "Ubuntu2204_NvidiaDevicePluginRunning",
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
		VMConfigMutatorWithError: func(ctx context.Context, vmss *armcompute.VirtualMachineScaleSet) error {
			vmss.SKU.Name = to.Ptr("Standard_NV6ads_A10_v5")
			if vmss.Tags == nil {
				vmss.Tags = map[string]*string{}
			}
			vmss.Tags["EnableManagedGPUExperience"] = to.Ptr("true")

			// Enable the AKS VM extension for GPU nodes
			extension, err := createVMExtensionLinuxAKSNode(ctx, vmss.Location)
			if err != nil {
				return fmt.Errorf("create AKS VM extension: %w", err)
			}
			vmss.Properties = addVMExtensionToVMSS(vmss.Properties, extension)
			return nil
		},
		Validator: func(ctx context.Context, s *Scenario) error {
			os := "ubuntu"
			osVersion := "r2204"

			// Validate that the NVIDIA device plugin binary was installed correctly
			devicePluginVersion, err := expectedPackageVersion("nvidia-device-plugin", os, osVersion)
			if err != nil {
				return err
			}
			if err := errors.Join(
				ValidateInstalledPackageVersion(ctx, s, "nvidia-device-plugin", devicePluginVersion),
				// Validate that the NVIDIA device plugin systemd service is running
				ValidateNvidiaDevicePluginServiceRunning(ctx, s),
			); err != nil {
				return err
			}
			// Resource advertisement depends on the device plugin service.
			if err := ValidateNodeAdvertisesGPUResources(ctx, s, 1, "nvidia.com/gpu"); err != nil {
				return err
			}

			// Validate that GPU workloads can be scheduled. Only meaningful once the GPU
			// resources above are advertised, otherwise the pod simply never gets scheduled.
			if err := ValidateGPUWorkloadSchedulable(ctx, s, 1, "nvidia.com/gpu"); err != nil {
				return err
			}

			// Validate that the NVIDIA DCGM packages were installed correctly
			if err := errors.Join(
				validateDCGMPackageVersions(ctx, s, os, osVersion),
				validateDCGMExporterRunning(ctx, s, "DCGM_FI_DEV_GPU_UTIL"),
			); err != nil {
				return err
			}

			// Let's run the NPD validation tests to verify that the nvidia
			// device plugin & DCGM services are reporting status correctly
			if err := ValidateNodeProblemDetector(ctx, s); err != nil {
				return err
			}
			// Restart NPD to ensure it picks up the managed GPU experience marker file,
			// which may have been created after NPD's initial startup during provisioning.
			if err := RestartNodeProblemDetector(ctx, s); err != nil {
				return err
			}
			if err := validateNPDNvidiaConditions(ctx, s); err != nil {
				return err
			}
			// Verify NVIDIA GRID license status checks are reporting status correctly.
			return validateNPDNvidiaGridLicense(ctx, s)
		},
	},
})

var _ = Register(&Scenario{
	Name:        "AzureLinux3_NvidiaDevicePluginRunning",
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
		VMConfigMutatorWithError: func(ctx context.Context, vmss *armcompute.VirtualMachineScaleSet) error {
			vmss.SKU.Name = to.Ptr("Standard_NC4as_T4_v3")
			if vmss.Tags == nil {
				vmss.Tags = map[string]*string{}
			}
			vmss.Tags["EnableManagedGPUExperience"] = to.Ptr("true")

			// Enable the AKS VM extension for GPU nodes
			extension, err := createVMExtensionLinuxAKSNode(ctx, vmss.Location)
			if err != nil {
				return fmt.Errorf("create AKS VM extension: %w", err)
			}
			vmss.Properties = addVMExtensionToVMSS(vmss.Properties, extension)
			return nil
		},
		Validator: func(ctx context.Context, s *Scenario) error {
			os := "azurelinux"
			osVersion := "v3.0"

			// Validate that the NVIDIA device plugin binary was installed correctly
			devicePluginVersion, err := expectedPackageVersion("nvidia-device-plugin", os, osVersion)
			if err != nil {
				return err
			}
			if err := errors.Join(
				ValidateInstalledPackageVersion(ctx, s, "nvidia-device-plugin", devicePluginVersion),
				// Validate that the NVIDIA device plugin systemd service is running
				ValidateNvidiaDevicePluginServiceRunning(ctx, s),
			); err != nil {
				return err
			}
			// Resource advertisement depends on the device plugin service.
			if err := ValidateNodeAdvertisesGPUResources(ctx, s, 1, "nvidia.com/gpu"); err != nil {
				return err
			}

			// Validate that GPU workloads can be scheduled. Only meaningful once the GPU
			// resources above are advertised, otherwise the pod simply never gets scheduled.
			if err := ValidateGPUWorkloadSchedulable(ctx, s, 1, "nvidia.com/gpu"); err != nil {
				return err
			}

			// Validate that the NVIDIA DCGM packages were installed correctly
			if err := errors.Join(
				validateDCGMPackageVersions(ctx, s, os, osVersion),
				validateDCGMExporterRunning(ctx, s, "DCGM_FI_DEV_GPU_UTIL"),
			); err != nil {
				return err
			}

			// Let's run the NPD validation tests to verify that the nvidia
			// device plugin & DCGM services are reporting status correctly
			if err := ValidateNodeProblemDetector(ctx, s); err != nil {
				return err
			}
			// Restart NPD to ensure it picks up the managed GPU experience marker file,
			// which may have been created after NPD's initial startup during provisioning.
			if err := RestartNodeProblemDetector(ctx, s); err != nil {
				return err
			}
			return validateNPDNvidiaConditions(ctx, s)
		},
	},
})

var _ = Register(&Scenario{
	Name:        "Ubuntu2404_NvidiaDevicePluginRunning_MIG",
	Description: "Tests that NVIDIA device plugin and DCGM Exporter work with MIG enabled on Ubuntu 24.04 GPU nodes",
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
			nbc.GPUInstanceProfile = "MIG2g"
			nbc.EnableManagedGPU = true
			nbc.MigStrategy = "Single"
		},
		VMConfigMutatorWithError: func(ctx context.Context, vmss *armcompute.VirtualMachineScaleSet) error {
			vmss.SKU.Name = to.Ptr("Standard_NC24ads_A100_v4")

			// Enable the AKS VM extension for GPU nodes
			extension, err := createVMExtensionLinuxAKSNode(ctx, vmss.Location)
			if err != nil {
				return fmt.Errorf("create AKS VM extension: %w", err)
			}
			vmss.Properties = addVMExtensionToVMSS(vmss.Properties, extension)
			return nil
		},
		Validator: func(ctx context.Context, s *Scenario) error {
			os := "ubuntu"
			osVersion := "r2404"

			// Validate that the NVIDIA device plugin binary was installed correctly
			devicePluginVersion, err := expectedPackageVersion("nvidia-device-plugin", os, osVersion)
			if err != nil {
				return err
			}
			if err := errors.Join(
				ValidateInstalledPackageVersion(ctx, s, "nvidia-device-plugin", devicePluginVersion),
				// Validate that the NVIDIA device plugin systemd service is running
				ValidateNvidiaDevicePluginServiceRunning(ctx, s),
			); err != nil {
				return err
			}
			if err := ValidateMIGModeEnabled(ctx, s, 1); err != nil {
				return err
			}
			if err := ValidateMIGInstancesCreated(ctx, s, "MIG 2g.20gb", 3); err != nil {
				return err
			}
			if err := ValidateNodeAdvertisesGPUResources(ctx, s, 3, "nvidia.com/gpu"); err != nil {
				return err
			}

			// Validate that GPU workloads can be scheduled. Only meaningful once the GPU
			// resources above are advertised, otherwise the pod simply never gets scheduled.
			if err := ValidateGPUWorkloadSchedulable(ctx, s, 3, "nvidia.com/gpu"); err != nil {
				return err
			}

			// Validate that the NVIDIA DCGM packages were installed correctly
			if err := errors.Join(
				validateDCGMPackageVersions(ctx, s, os, osVersion),
				validateDCGMExporterRunning(ctx, s, "DCGM_FI_DEV_GPU_TEMP"),
			); err != nil {
				return err
			}

			// Let's run the NPD validation tests to verify that the nvidia
			// device plugin & DCGM services are reporting status correctly
			if err := ValidateNodeProblemDetector(ctx, s); err != nil {
				return err
			}
			return validateNPDNvidiaConditions(ctx, s)
		},
	},
})

var _ = Register(newUbuntu2404_NvidiaDevicePluginRunning_MIG_MultiGPUScenario())

func newUbuntu2404_NvidiaDevicePluginRunning_MIG_MultiGPUScenario() *Scenario {
	const (
		gpuCount           = 2
		migInstancesPerGPU = 3
		totalMIGInstances  = gpuCount * migInstancesPerGPU
		multiGPUA100VMSize = "Standard_NC48ads_A100_v4"
	)

	return &Scenario{
		Name:             "Ubuntu2404_NvidiaDevicePluginRunning_MIG_MultiGPU",
		Description:      "Tests that a MIG profile is applied to every GPU on an Ubuntu 24.04 multi-GPU VM",
		K8sSystemPoolSKU: "Standard_D2s_v3",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster:               ClusterKubenet,
			VHD:                   config.VHDUbuntu2404Gen2Containerd,
			WaitForSSHAfterReboot: 5 * time.Minute,
			BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = multiGPUA100VMSize
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = true
				nbc.EnableNvidia = true
				nbc.GPUInstanceProfile = "MIG2g"
				nbc.EnableManagedGPU = true
				nbc.MigStrategy = "Single"
			},
			VMConfigMutatorWithError: func(ctx context.Context, vmss *armcompute.VirtualMachineScaleSet) error {
				vmss.SKU.Name = to.Ptr(multiGPUA100VMSize)

				extension, err := createVMExtensionLinuxAKSNode(ctx, vmss.Location)
				if err != nil {
					return fmt.Errorf("create AKS VM extension: %w", err)
				}
				vmss.Properties = addVMExtensionToVMSS(vmss.Properties, extension)
				return nil
			},
			Validator: func(ctx context.Context, s *Scenario) error {
				devicePluginVersion, err := expectedPackageVersion("nvidia-device-plugin", "ubuntu", "r2404")
				if err != nil {
					return err
				}
				if err := errors.Join(
					ValidateInstalledPackageVersion(ctx, s, "nvidia-device-plugin", devicePluginVersion),
					ValidateNvidiaDevicePluginServiceRunning(ctx, s),
				); err != nil {
					return err
				}
				if err := ValidateMIGModeEnabled(ctx, s, gpuCount); err != nil {
					return err
				}
				if err := ValidateMIGInstancesCreated(ctx, s, "MIG 2g.20gb", totalMIGInstances); err != nil {
					return err
				}
				if err := ValidateNodeAdvertisesGPUResources(ctx, s, totalMIGInstances, "nvidia.com/gpu"); err != nil {
					return err
				}
				// Scheduling a GPU workload only works once the MIG resources above are advertised.
				return ValidateGPUWorkloadSchedulable(ctx, s, 1, "nvidia.com/gpu")
			},
		},
	}
}

var _ = Register(&Scenario{
	Name:        "Ubuntu2204_NvidiaDevicePluginRunning_WithoutVMSSTag",
	Description: "Tests that NVIDIA device plugin and DCGM Exporter work via NBC EnableManagedGPU field without VMSS tag",
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
			nbc.EnableManagedGPU = true
		},
		VMConfigMutatorWithError: func(ctx context.Context, vmss *armcompute.VirtualMachineScaleSet) error {
			vmss.SKU.Name = to.Ptr("Standard_NV6ads_A10_v5")
			// Explicitly DO NOT set the EnableManagedGPUExperience VMSS tag
			// to test that NBC EnableManagedGPU field works independently

			// Enable the AKS VM extension for GPU nodes
			extension, err := createVMExtensionLinuxAKSNode(ctx, vmss.Location)
			if err != nil {
				return fmt.Errorf("create AKS VM extension: %w", err)
			}
			vmss.Properties = addVMExtensionToVMSS(vmss.Properties, extension)
			return nil
		},
		Validator: func(ctx context.Context, s *Scenario) error {
			os := "ubuntu"
			osVersion := "r2204"

			// Validate that the NVIDIA device plugin binary was installed correctly
			devicePluginVersion, err := expectedPackageVersion("nvidia-device-plugin", os, osVersion)
			if err != nil {
				return err
			}
			if err := errors.Join(
				ValidateInstalledPackageVersion(ctx, s, "nvidia-device-plugin", devicePluginVersion),
				// Validate that the NVIDIA device plugin systemd service is running
				ValidateNvidiaDevicePluginServiceRunning(ctx, s),
			); err != nil {
				return err
			}
			// Resource advertisement depends on the device plugin service.
			if err := ValidateNodeAdvertisesGPUResources(ctx, s, 1, "nvidia.com/gpu"); err != nil {
				return err
			}

			// Validate that GPU workloads can be scheduled. Only meaningful once the GPU
			// resources above are advertised, otherwise the pod simply never gets scheduled.
			if err := ValidateGPUWorkloadSchedulable(ctx, s, 1, "nvidia.com/gpu"); err != nil {
				return err
			}

			// Validate that the NVIDIA DCGM packages were installed correctly
			if err := errors.Join(
				validateDCGMPackageVersions(ctx, s, os, osVersion),
				validateDCGMExporterRunning(ctx, s, "DCGM_FI_DEV_GPU_UTIL"),
			); err != nil {
				return err
			}

			// Let's run the NPD validation tests to verify that the nvidia
			// device plugin & DCGM services are reporting status correctly
			if err := ValidateNodeProblemDetector(ctx, s); err != nil {
				return err
			}
			// Restart NPD to ensure it picks up the managed GPU experience marker file,
			// which may have been created after NPD's initial startup during provisioning.
			if err := RestartNodeProblemDetector(ctx, s); err != nil {
				return err
			}
			if err := validateNPDNvidiaConditions(ctx, s); err != nil {
				return err
			}
			// Verify NVIDIA GRID license status checks are reporting status correctly.
			return validateNPDNvidiaGridLicense(ctx, s)
		},
	},
})

var _ = Register(&Scenario{
	Name:        "Ubuntu2404_NvidiaDevicePluginRunning_MIG_Mixed",
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
		VMConfigMutatorWithError: func(ctx context.Context, vmss *armcompute.VirtualMachineScaleSet) error {
			vmss.SKU.Name = to.Ptr("Standard_NC24ads_A100_v4")

			// Enable the AKS VM extension for GPU nodes
			extension, err := createVMExtensionLinuxAKSNode(ctx, vmss.Location)
			if err != nil {
				return fmt.Errorf("create AKS VM extension: %w", err)
			}
			vmss.Properties = addVMExtensionToVMSS(vmss.Properties, extension)
			return nil
		},
		Validator: func(ctx context.Context, s *Scenario) error {
			os := "ubuntu"
			osVersion := "r2404"

			// Validate that the NVIDIA device plugin binary was installed correctly
			devicePluginVersion, err := expectedPackageVersion("nvidia-device-plugin", os, osVersion)
			if err != nil {
				return err
			}
			migResourceName := "nvidia.com/mig-1g.10gb"
			if err := errors.Join(
				ValidateInstalledPackageVersion(ctx, s, "nvidia-device-plugin", devicePluginVersion),
				// Validate that the NVIDIA device plugin systemd service is running
				ValidateNvidiaDevicePluginServiceRunning(ctx, s),
			); err != nil {
				return err
			}
			if err := ValidateMIGModeEnabled(ctx, s, 1); err != nil {
				return err
			}
			if err := ValidateMIGInstancesCreated(ctx, s, "MIG 1g.10gb", 7); err != nil {
				return err
			}
			if err := ValidateNodeAdvertisesGPUResources(ctx, s, 7, migResourceName); err != nil {
				return err
			}

			// Validate that MIG workloads can be scheduled. Only meaningful once the MIG
			// resources above are advertised, otherwise the pod simply never gets scheduled.
			return ValidateGPUWorkloadSchedulable(ctx, s, 2, migResourceName)
		},
	},
})

var _ = Register(&Scenario{
	Name:        "Ubuntu2404_DraDriverNvidiaGpuRunning",
	Description: "Tests DRA driver works on Ubuntu 24.04 VHD with containerd v2",
	Tags: Tags{
		GPU: true,
	},

	Config: Config{
		Cluster: ClusterKubenet,
		VHD:     config.VHDUbuntu2404Gen2Containerd,
		BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
			nbc.AgentPoolProfile.VMSize = "Standard_NV6ads_A10_v5"
			nbc.ConfigGPUDriverIfNeeded = true
			nbc.EnableNvidia = true
			nbc.EnableManagedGPUDRA = true
		},
		VMConfigMutatorWithError: func(ctx context.Context, vmss *armcompute.VirtualMachineScaleSet) error {
			vmss.SKU.Name = to.Ptr("Standard_NV6ads_A10_v5")

			// Enable the AKS VM extension for GPU nodes
			extension, err := createVMExtensionLinuxAKSNode(ctx, vmss.Location)
			if err != nil {
				return fmt.Errorf("create AKS VM extension: %w", err)
			}
			vmss.Properties = addVMExtensionToVMSS(vmss.Properties, extension)
			return nil
		},
		Validator: func(ctx context.Context, s *Scenario) error {
			containerdVersions := components.GetExpectedPackageVersions("containerd", "ubuntu", "r2404")
			runcVersions := components.GetExpectedPackageVersions("runc", "ubuntu", "r2404")
			if err := errors.Join(
				ValidateContainerd2Properties(ctx, s, containerdVersions),
				ValidateRuncVersion(ctx, s, runcVersions),
				ValidateContainerRuntimePlugins(ctx, s),
			); err != nil {
				return err
			}
			if err := ValidateDraDriverNvidiaGpuServiceRunning(ctx, s); err != nil {
				return err
			}
			return ValidateDRAWorkloadSchedulable(ctx, s)
		},
	},
})

var _ = Register(&Scenario{
	Name:        "Ubuntu2404_DraDriverNvidiaGpuRunning_AKSNodeController",
	Description: "Tests DRA driver works on Ubuntu 24.04 VHD with containerd v2 using aks-node-controller",
	Tags: Tags{
		GPU:        true,
		Scriptless: true,
	},

	Config: Config{
		Cluster: ClusterKubenet,
		VHD:     config.VHDUbuntu2404Gen2Containerd,
		BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
			nbc.AgentPoolProfile.VMSize = "Standard_NV6ads_A10_v5"
			nbc.ConfigGPUDriverIfNeeded = true
			nbc.EnableNvidia = true
			nbc.EnableManagedGPUDRA = true
		},
		AKSNodeConfigMutator: func(_ *Cluster, config *aksnodeconfigv1.Configuration) {
			config.VmSize = "Standard_NV6ads_A10_v5"
			config.GpuConfig.ConfigGpuDriver = true
			config.GpuConfig.EnableNvidia = to.Ptr(true)
			config.GpuConfig.EnableManagedGpuDra = true
		},
		VMConfigMutatorWithError: func(ctx context.Context, vmss *armcompute.VirtualMachineScaleSet) error {
			vmss.SKU.Name = to.Ptr("Standard_NV6ads_A10_v5")

			// Enable the AKS VM extension for GPU nodes
			extension, err := createVMExtensionLinuxAKSNode(ctx, vmss.Location)
			if err != nil {
				return fmt.Errorf("creating AKS VM extension: %w", err)
			}
			vmss.Properties = addVMExtensionToVMSS(vmss.Properties, extension)
			return nil
		},
		Validator: func(ctx context.Context, s *Scenario) error {
			containerdVersions := components.GetExpectedPackageVersions("containerd", "ubuntu", "r2404")
			runcVersions := components.GetExpectedPackageVersions("runc", "ubuntu", "r2404")
			if err := errors.Join(
				ValidateContainerd2Properties(ctx, s, containerdVersions),
				ValidateRuncVersion(ctx, s, runcVersions),
				ValidateContainerRuntimePlugins(ctx, s),
			); err != nil {
				return err
			}
			if err := ValidateDraDriverNvidiaGpuServiceRunning(ctx, s); err != nil {
				return err
			}
			return ValidateDRAWorkloadSchedulable(ctx, s)
		},
	},
})
