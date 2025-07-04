package e2e

import (
	"context"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
)

func Test_TwoStageKubeletConfiguration_Linux(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests complete two-stage workflow: Stage 1 (PreProvisionOnly) then Stage 2 on same VM",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV2Gen2,
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				// Configure VM to use persistent disk instead of ephemeral for image capture
				if vmss.Properties.VirtualMachineProfile.StorageProfile.OSDisk != nil {
					vmss.Properties.VirtualMachineProfile.StorageProfile.OSDisk.DiffDiskSettings = nil
				}
			},
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.PreProvisionOnly = true

			},
			SkipDefaultValidation: true,
			Validator: func(ctx context.Context, s *Scenario) {
				// Stage 1 validation: Verify kubelet was skipped
				//ValidateFileHasContent(ctx, s, "/var/log/azure/cluster-provision.log", "Skipping kubelet configuration as requested")
				ValidateFileExists(ctx, s, "/etc/containerd/config.toml")
				ValidateSystemdUnitIsRunning(ctx, s, "containerd")
				ValidateFileExists(ctx, s, "/opt/azure/containers/preprovision.complete")
				ValidateFileDoesNotExist(ctx, s, "/opt/azure/containers/provision.complete")

				t.Log("=== Stage 1 validation complete, proceeding to Stage 2 ===")
				customVHD := CreateImage(ctx, s)

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
							},
							Validator: func(ctx context.Context, s *Scenario) {
								// Stage 2 validation: Verify kubelet is now working
								ValidateFileHasContent(ctx, s, "/var/log/azure/cluster-provision.log", "Running in kubelet-only mode")
								ValidateSystemdUnitIsRunning(ctx, s, "kubelet")
								ValidateFileExists(ctx, s, "/opt/azure/containers/provision.complete")
							},
						},
					})
				})

			},
		},
	})
}

func Test_TwoStageKubeletConfiguration_Windows(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests complete two-stage workflow: Stage 1 (PreProvision) with VHD creation, then Stage 2 using created VHD",
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
				nbc.PreProvisionOnly = true
			},
			SkipDefaultValidation: true, // Skip default validation since Stage 1 CSE may fail
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileExists(ctx, s, "C:\\AzureData\\preprovision.complete")
				ValidateFileDoesNotExist(ctx, s, "C:\\AzureData\\provision.complete")
				ValidateWindowsServiceIsNotRunning(ctx, s, "kubelet")

				t.Log("=== Stage 1 validation complete, proceeding to Stage 2 ===")
				customVHD := CreateImage(ctx, s)
				stage1SSHKeys := s.Runtime.NBC.ContainerService.Properties.LinuxProfile.SSH.PublicKeys

				// Create a subtest so RunScenario won't fail on t.Parallel()
				t.Run("SecondStage", func(t *testing.T) {
					// Run Stage 2 scenario using the custom VHD
					// RunScenario fails due to the running of the t.Parallel() for the second time
					RunScenario(t, &Scenario{
						Description: "Stage 2: Create VMSS from captured VHD via SIG",
						Config: Config{
							Cluster: s.Config.Cluster,
							VHD:     customVHD,
							BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
								if nbc.ContainerService.Properties.LinuxProfile != nil {
									nbc.ContainerService.Properties.LinuxProfile.SSH.PublicKeys = stage1SSHKeys
								}
							},
							Validator: func(ctx context.Context, s *Scenario) {
								// Stage 2 validation: Verify kubelet is now working
								ValidateFileExists(ctx, s, "C:\\AzureData\\preprovision.complete") // Test with known existing file first
								ValidateFileExists(ctx, s, "C:\\AzureData\\provision.complete")
								ValidateWindowsServiceIsRunning(ctx, s, "kubelet")
							},
						},
					})
				})
			},
		},
	})
}
