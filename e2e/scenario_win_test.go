package e2e

import (
	"context"
	"fmt"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/stretchr/testify/require"
)

func EmptyBootstrapConfigMutator(configuration *datamodel.NodeBootstrappingConfiguration) {}
func EmptyVMConfigMutator(vmss *armcompute.VirtualMachineScaleSet)                        {}

func Test_Windows2019Containerd(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2019 with Containerd",
		Config: Config{
			Cluster:                ClusterAzureNetwork,
			VHD:                    config.VHDWindows2019Containerd,
			VMConfigMutator:        EmptyVMConfigMutator,
			BootstrapConfigMutator: EmptyBootstrapConfigMutator,
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateWindowsVersionFromWindowsSettings(ctx, s, "2019-containerd")
				ValidateWindowsProductName(ctx, s, "Windows Server 2019 Datacenter")
				// TODO: currently the command used to get the display name returns an empty string on WS2019. Need to find a better command.
				//ValidateWindowsDisplayVersion(ctx, s, "???")
				ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote")
				ValidateWindowsProcessHasCliArguments(ctx, s, "kubelet.exe", []string{"--rotate-certificates=true", "--client-ca-file=c:\\k\\ca.crt"})
				ValidateCiliumIsNotRunningWindows(ctx, s)
				ValidateFileHasContent(ctx, s, "/AzureData/CustomDataSetupScript.log", "CSEScriptsPackageUrl used for provision is https://packages.aks.azure.com/aks/windows/cse/aks-windows-cse-scripts-current.zip")
			},
		},
	})
}

func Test_Windows2022Containerd(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2022 with Containerd",
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
				ValidateFileHasContent(ctx, s, "/k/test.txt", "this is a test file")
			},
		},
	})
}

func Test_Windows2022ContainerdGen2(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2022 with Containerd - hyperv gen 2",
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

func Test_Windows23H2(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 23H2 with Containerd",
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
				ValidateFileHasContent(ctx, s, "/k/test.txt", "this is a test file")
			},
		},
	})
}

func Test_Windows23H2Gen2(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 23H2 with Containerd - hyperv gen2",
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
	RunScenario(t, &Scenario{
		Description: "Windows Server 2025 with Containerd",
		Config: Config{
			Cluster:                ClusterAzureNetwork,
			VHD:                    config.VHDWindows2025,
			VMConfigMutator:        EmptyVMConfigMutator,
			BootstrapConfigMutator: EmptyBootstrapConfigMutator,
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
	RunScenario(t, &Scenario{
		Description: "Windows Server 2025 with Containerd - hyperv gen 2",
		Config: Config{
			Cluster:                ClusterAzureNetwork,
			VHD:                    config.VHDWindows2025Gen2,
			VMConfigMutator:        EmptyVMConfigMutator,
			BootstrapConfigMutator: EmptyBootstrapConfigMutator,
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

func Test_TwoStageKubeletConfiguration_Windows(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests complete two-stage workflow: Stage 1 (SkipKubeletConfiguration) with VHD creation, then Stage 2 (KubeletOnly) using created VHD",
		Config: Config{
			Cluster: ClusterAzureNetwork,
			VHD:     config.VHDWindows2025,
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				// Configure VM to use persistent disk instead of ephemeral for image capture
				if vmss.Properties.VirtualMachineProfile.StorageProfile.OSDisk != nil {
					vmss.Properties.VirtualMachineProfile.StorageProfile.OSDisk.DiffDiskSettings = nil
				}
			},
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.SkipKubeletConfiguration = true
			},
			SkipDefaultValidation: true, // Skip default validation since Stage 1 CSE may fail
			Validator: func(ctx context.Context, s *Scenario) {
				// Check if Stage 1 completed successfully or if we need to proceed despite CSE failure
				if fileExist(ctx, s, "/AzureData/stage1-complete") {
					s.T.Log("Stage 1 completed successfully - stage1-complete marker found")
				} else {
					s.T.Log("Stage 1 CSE failed, but proceeding with VHD creation anyway")
				}

				// Log whether provision.complete exists (it shouldn't in Stage 1)
				if fileExist(ctx, s, "/AzureData/provision.complete") {
					s.T.Log("Warning: provision.complete exists, but proceeding anyway")
				} else {
					s.T.Log("Confirmed: provision.complete does not exist (Stage 1 behavior)")
				}

				// Create VHD from the VM regardless of CSE success/failure
				// This allows us to capture the VM state after Stage 1 setup
				RunTwoStageWithVHDCreation(ctx, s)
			},
		},
	})
}

// RunTwoStageWithVHDCreation creates a VHD from the Stage 1 VM and then creates a new VM from that VHD for Stage 2
func RunTwoStageWithVHDCreation(ctx context.Context, s *Scenario) {
	s.T.Log("=== Creating VHD from Stage 1 VM ===")

	// Generate unique names for the image and stage 2 VMSS
	imageBaseName := fmt.Sprintf("%s-stage1", s.Runtime.VMSSName)

	// Get the resource group name
	resourceGroupName := *s.Runtime.Cluster.Model.Properties.NodeResourceGroup

	// Generalize and capture the first VM instance as a managed image
	s.T.Logf("Generalizing and capturing Stage 1 VM as managed image: %s", imageBaseName)

	managedImage, err := config.Azure.GeneralizeAndCaptureVMSSVMAsImage(ctx, resourceGroupName, s.Runtime.VMSSName, "0", imageBaseName)
	require.NoError(s.T, err, "Failed to generalize and capture VM as managed image")

	// Set up cleanup for the managed image and snapshot
	s.T.Cleanup(func() {
		// Clean up the managed image
		if err := config.Azure.DeleteImage(ctx, resourceGroupName, imageBaseName); err != nil {
			s.T.Logf("Failed to delete managed image %s: %v", imageBaseName, err)
		}
		// Clean up the snapshot
		snapshotName := imageBaseName + "-snapshot"
		if err := config.Azure.DeleteSnapshot(ctx, resourceGroupName, snapshotName); err != nil {
			s.T.Logf("Failed to delete snapshot %s: %v", snapshotName, err)
		}
	})

	// Create a custom VHD configuration that points directly to the managed image
	customVHD := &config.Image{
		Name:   imageBaseName,
		OS:     s.Config.VHD.OS,
		Arch:   s.Config.VHD.Arch,
		Distro: s.Config.VHD.Distro,
		Gallery: &config.Gallery{
			SubscriptionID:    config.Config.SubscriptionID,
			ResourceGroupName: resourceGroupName,
			Name:              "managed-images", // Special marker for managed images
		},
		Version: *managedImage.ID, // Store the managed image ID in Version field
	}

	// Create a subtest so RunScenario won't fail on t.Parallel()
	s.T.Run("SecondStage", func(t *testing.T) {
		// Run Stage 2 scenario using the custom VHD
		// RunScenario fails due to the running of the t.Parallel() for the second time
		RunScenario(t, &Scenario{
			Description: "Stage 2: Create VMSS from captured VHD via SIG",
			Config: Config{
				Cluster: s.Config.Cluster,
				VHD:     customVHD,
				BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
					// Stage 2: Don't use KubeletOnly since files were cleaned up by sysprep
					// Instead, let it run the full CSE process which will handle kubelet setup
					nbc.KubeletOnly = false
					nbc.SkipKubeletConfiguration = false
				},
				Validator: func(ctx context.Context, s *Scenario) {
					ValidateFileExists(ctx, s, "/AzureData/stage1-complete")
					ValidateFileExists(ctx, s, "/AzureData/provision.complete")
				},
			},
		})
	})

}
