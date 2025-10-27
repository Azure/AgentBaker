

func Test_Ubuntu2404_GB200(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that GB200 images boot on GB200, have all expected services and packages, and match the current checked-in CRD",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster:               ClusterKubenet,
			VHD:                   config.VHDUbuntu2404GB200,
			WaitForSSHAfterReboot: 5 * time.Minute,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = "Standard_ND128isr_NDR_GB200_v6"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = true
				nbc.EnableNvidia = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_ND128isr_NDR_GB200_v6")
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
