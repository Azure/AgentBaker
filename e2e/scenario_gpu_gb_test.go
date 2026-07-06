package e2e

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
	"github.com/stretchr/testify/require"
)

// gbGPUCountPerNode is the number of GPUs a single GB200/GB300 VM advertises to the kubelet.
// TODO(confirm-on-hardware): set from the real SKU; GB200/GB300 VMs are expected to expose 4
// Blackwell GPUs each. Override at runtime with GB_GPU_COUNT if this differs.
const gbGPUCountPerNode int64 = 4

// gbGPUCount resolves the expected per-node GPU count, allowing a GB_GPU_COUNT override so a wrong
// hardcode doesn't fail an otherwise-good run.
func gbGPUCount(t testing.TB) int64 {
	if v := os.Getenv("GB_GPU_COUNT"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		require.NoErrorf(t, err, "invalid GB_GPU_COUNT=%q", v)
		require.Positivef(t, n, "GB_GPU_COUNT must be > 0, got %d", n)
		return n
	}
	return gbGPUCountPerNode
}

// gbArm64VHD returns the vanilla arm64 Ubuntu 24.04 image (the SKU our GB design targets: GB runs on
// the general arm64 image, no bespoke VHD). If GB_VHD_VERSION is set, that exact SIG image version is
// pinned; otherwise the image is resolved by the usual tag mechanism (e.g.
// SIG_VERSION_TAG_NAME=buildId SIG_VERSION_TAG_VALUE=<build> to point at a specific pipeline build).
func gbArm64VHD() *config.Image {
	vhd := *config.VHDUbuntu2404ArmContainerd
	if v := os.Getenv("GB_VHD_VERSION"); v != "" {
		vhd.Version = v
	}
	return &vhd
}

// Test_Ubuntu2404Arm64_GB300_IMEX validates a Grace-Blackwell GB300 node on the vanilla arm64 image:
// the open R580 driver comes up via aks-gpu, Fabric Manager stays OFF, and IMEX is installed + its
// channel created while the daemon stays disabled. See runGBIMEXScenario for the full assertion set.
func Test_Ubuntu2404Arm64_GB300_IMEX(t *testing.T) {
	runGBIMEXScenario(t, "Standard_ND128isr_GB300_v6")
}

// Test_Ubuntu2404Arm64_GB200_IMEX is the GB200 counterpart of Test_Ubuntu2404Arm64_GB300_IMEX.
func Test_Ubuntu2404Arm64_GB200_IMEX(t *testing.T) {
	runGBIMEXScenario(t, "Standard_ND128isr_NDR_GB200_v6")
}

// runGBIMEXScenario builds and runs the GB IMEX scenario for the given VM size. The FM-off / IMEX-on
// behavior is driven entirely by the VM size (via datamodel.FabricManagerGPUSizes / IMEXGPUSizes), so
// the two GB tests differ only in the size string.
//
// This scenario deliberately does NOT set SkipScriptlessNBC, so RunScenario also runs the
// scriptless_nbc sub-test -- that is the aks-node-controller path where GPU_NEEDS_IMEX is plumbed, and
// the only path that exercises the parser wiring. The IMEX channel modprobe sets REBOOTREQUIRED, so
// WaitForSSHAfterReboot lets SSH validation ride out the provisioning reboot.
func runGBIMEXScenario(t *testing.T, vmSize string) {
	RunScenario(t, &Scenario{
		Description: fmt.Sprintf("Grace-Blackwell %s on vanilla arm64: open R580 via aks-gpu, Fabric Manager OFF, IMEX installed + channel created (daemon disabled)", vmSize),
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster:               ClusterKubenet,
			VHD:                   gbArm64VHD(),
			WaitForSSHAfterReboot: 5 * time.Minute,
			BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = vmSize
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = true
				nbc.EnableNvidia = true
				nbc.ManagedGPUExperienceAFECEnabled = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr(vmSize)
				if vmss.Tags == nil {
					vmss.Tags = map[string]*string{}
				}
				vmss.Tags["EnableManagedGPUExperience"] = to.Ptr("true")

				// Enable the AKS VM extension for GPU nodes (matches the managed-GPU tests).
				extension, err := createVMExtensionLinuxAKSNode(t.Context(), vmss.Location)
				require.NoError(t, err, "creating AKS VM extension")
				vmss.Properties = addVMExtensionToVMSS(vmss.Properties, extension)
			},
			Validator: func(ctx context.Context, s *Scenario) {
				// Driver: aks-gpu installed the OPEN R580 kernel module on arm64 and nvidia-smi works.
				ValidateNvidiaSMIInstalled(ctx, s)
				ValidateNvidiaOpenKernelModuleDriver(ctx, s)

				// Fabric: GB has no on-board NVSwitch -> Fabric Manager must NOT run.
				ValidateFabricManagerNotRunning(ctx, s)

				// IMEX: channel modprobe conf written, channel device present after reboot, package
				// installed from the baked cache but daemon left disabled, and the cache cleaned up.
				ValidateFileHasContent(ctx, s, "/etc/modprobe.d/nvidia-imex.conf", "NVreg_CreateImexChannel0=1")
				ValidateIMEXChannelDeviceExists(ctx, s)
				ValidateNvidiaIMEXInstalledButDisabled(ctx, s)
				ValidateIMEXCacheRemoved(ctx, s)

				// Behavior parity: the managed device plugin advertises GPUs to the kubelet.
				ValidateNodeAdvertisesGPUResources(ctx, s, gbGPUCount(s.T), "nvidia.com/gpu")
			},
		},
	})
}
