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

func Test_Ubuntu2404_GB200(t *testing.T) {
	vhd_img := *config.VHDUbuntu2404GB200 // Shallow copy
	vhd_img.Version = "1.1.316"           // specific GB200 image version present in relevant regions

	RunScenario(t, &Scenario{
		Description: "Tests that GB200 images boot on GB200, have all expected services and packages, and match the current checked-in CRD",
		Tags: Tags{
			GPU:   true,
			GB200: true,
		},
		Location:         "centraluseuap",
		K8sSystemPoolSKU: "standard_d2s_v5",
		Config: Config{
			Cluster:               clusterKubenet,
			VHD:                   &vhd_img,
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

				// Check that:
				// 1. All packages installed match or exceed the CRD from HPC team/NVIDIA (either checked-in or provided via environment variable)
				// 2. azure-nvidia kernel is installed and in use
				// 3. DCGM and DCGM exporter are running
				// 4. GPU driver and plugin are running and advertise GPU resources
				// 5. The IB interface is up
				// 6. DOCA/OFED drivers are running
				// 7. NVIDIA IMEX service is running

				// First, CRD check.
				ValidateGB200CRDSatisfied(ctx, s)

				// Next, validate that the azure-nvidia kernel is in use.
				ValidateAzureNvidiaKernelRunning(ctx, s)

				// Validate that the NVIDIA DCGM packages are installed and running.
				for _, packageName := range getDCGMPackageNames(os) {
					versions := components.GetExpectedPackageVersions(packageName, os, osVersion)
					require.Lenf(s.T, versions, 1, "Expected exactly one %s version for %s %s but got %d", packageName, os, osVersion, len(versions))
					ValidateInstalledPackageVersion(ctx, s, packageName, versions[0])
				}

				ValidateNvidiaDCGMExporterSystemDServiceRunning(ctx, s)
				ValidateNvidiaDCGMExporterIsScrapable(ctx, s)
				ValidateNvidiaDCGMExporterScrapeCommonMetric(ctx, s, "DCGM_FI_DEV_GPU_TEMP")

				// Validate that the NVIDIA device plugin binary was installed correctly
				versions := components.GetExpectedPackageVersions("nvidia-device-plugin", os, osVersion)
				require.Lenf(s.T, versions, 1, "Expected exactly one nvidia-device-plugin version for %s %s but got %d", os, osVersion, len(versions))
				ValidateInstalledPackageVersion(ctx, s, "nvidia-device-plugin", versions[0])

				// Validate that the NVIDIA device plugin systemd service is running
				ValidateNvidiaDevicePluginServiceRunning(ctx, s)

				// Validate that GPU resources are advertised by the device plugin
				ValidateNodeAdvertisesGPUResources(ctx, s, 4)

				// 5. Validate IB interface.
				ValidateIBInterfacesUp(ctx, s)

				// 6. Validate DOCA/OFED drivers are running.
				ValidateDOCAOFEDDriversRunning(ctx, s)

				// 7. Validate NVIDIA IMEX service is running.
				ValidateNvidiaIMEXServiceRunning(ctx, s)

				// TODO: IB interface, DOCA/OFED, IMEX.
			},
		},
	})
}
