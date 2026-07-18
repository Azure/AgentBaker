package e2e

import (
	"context"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
)

// Test_Ubuntu2404Arm64_GB300_ManagedDriver provisions an 18-node GB300 (arm64)
// VMSS and drives the *managed* aks-gpu driver install straight through CSE,
// deliberately bypassing the RP's VMSizeDoesNotSupportGPU gate (the RP refuses
// --gpu-driver Install for GB300 because Compute SKU metadata lacks the "GPUs"
// capability). This answers design-doc Experiment 2: does the managed aks-gpu
// image DKMS-build the open R580 driver on the linux-azure-nvidia arm64 kernel?
//
// How the bypass works: the GPU-install gate in CSE is GPU_NODE, and
// variables.go maps "gpuNode" -> config.EnableNvidia (NOT any SKU allowlist).
// Setting EnableNvidia + ConfigGPUDriverIfNeeded here forces the install, and
// GetGPUDriverVersion() defaults GB300 to the cuda-lts (R580 LTS) image
// mcr.microsoft.com/aks/aks-gpu-cuda-lts.
//
// The 18-node capacity is the GB300 ICB rack requirement; dedicated quota
// covers allocation (no interconnect-block/group profile is needed on the VMSS).
// Run with KEEP_VMSS=true to leave the rack up for on-node inspection:
//
//	KEEP_VMSS=true go test -run '^Test_Ubuntu2404Arm64_GB300_ManagedDriver$' -count=1 -v -timeout 120m
func Test_Ubuntu2404Arm64_GB300_ManagedDriver(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Provisions an 18-node GB300 arm64 VMSS with the managed aks-gpu (R580 LTS) driver install and verifies nvidia-smi + open kernel module on-node",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			// The plain 2404gen2arm64containerd definition is the UNIFIED dual-kernel
			// arm64 24.04 image: its release notes bake both linux-azure (6.8 LTS) and
			// linux-azure-nvidia (6.14.0-1007) — the same kernel a real GB300 node runs
			// (verified: gb300-test-rg/gb300 booted this definition on 6.14.0-1007-azure-nvidia).
			// grub's 10_azure_nvidia selects the -azure-nvidia kernel on GB hardware.
			// Unpinned → resolves in-tenant (test gallery, latest branch=main); no prod
			// cross-tenant gallery, so no InvalidAuthenticationTokenTenant.
			VHD: config.VHDUbuntu2404ArmContainerd,
			BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = "Standard_ND128isr_GB300_v6"
				// GPU_NODE=true (variables.go: gpuNode <- EnableNvidia) + run the
				// aks-gpu install; GB300 falls through GetGPUDriverVersion to cuda-lts.
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableNvidia = true
				// localdns fails to start on GB300 and CSE retries it ~100× (basePrep),
				// exhausting the provisioning window before the GPU install runs. It's
				// not needed for the driver test — disable it so CSE reaches nodePrep.
				nbc.AgentPoolProfile.LocalDNSProfile = nil
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_ND128isr_GB300_v6")
				// GB300 allocates as an 18-node ICB/NVLink rack (dedicated quota covers it).
				vmss.SKU.Capacity = to.Ptr[int64](18)
				p := vmss.Properties
				// The GB300 NVLink rack must span placement groups — a single placement
				// group can't hold the vertical-connect rack. Setting SinglePlacementGroup
				// = false is what makes the VMSS eligible for Vertical Connect; a real AKS
				// GB300 nodepool VMSS sets exactly this and does NOT set
				// HighSpeedInterconnectPlacement, so we don't either (setting it, or leaving
				// SinglePlacementGroup at its default true, yields 400 "Vertical Connect
				// Feature is not available for this Virtual Machine Scale Set").
				p.SinglePlacementGroup = to.Ptr(false)
				p.PlatformFaultDomainCount = to.Ptr[int32](1)
				// GB300 OS disk, matching a real AKS GB300 VMSS: ephemeral on the CacheDisk
				// (ResourceDisk → InvalidParameter, NvmeDisk → NotSupported), 1024 GB,
				// Standard_LRS, ReadOnly. The image advertises DiskControllerTypes: SCSI,NVMe
				// so Compute picks NVMe automatically — no explicit diskControllerType needed.
				osd := p.VirtualMachineProfile.StorageProfile.OSDisk
				osd.DiffDiskSettings = &armcompute.DiffDiskSettings{
					Option:    to.Ptr(armcompute.DiffDiskOptionsLocal),
					Placement: to.Ptr(armcompute.DiffDiskPlacementCacheDisk),
				}
				osd.Caching = to.Ptr(armcompute.CachingTypesReadOnly)
				osd.DiskSizeGB = to.Ptr[int32](1024)
				osd.ManagedDisk = &armcompute.VirtualMachineScaleSetManagedDiskParameters{
					StorageAccountType: to.Ptr(armcompute.StorageAccountTypesStandardLRS),
				}
			},
			Validator: func(ctx context.Context, s *Scenario) {
				// The managed driver install must succeed on arm64 GB300.
				ValidateNvidiaSMIInstalled(ctx, s)
				ValidateNvidiaModProbeInstalled(ctx, s)
				// Experiment-2 assertions: R580 driver, built as the OPEN kernel
				// module (Blackwell is open-module only).
				ValidateGB300ManagedDriverOpenR580(ctx, s)
			},
		},
	})
}

// ValidateGB300ManagedDriverOpenR580 asserts the managed aks-gpu install landed
// the R580 (580.x) driver and that it is the NVIDIA *open* kernel module — the
// only variant Blackwell supports. It reads the driver version from nvidia-smi
// and the module flavor from /proc/driver/nvidia/version.
func ValidateGB300ManagedDriverOpenR580(ctx context.Context, s *Scenario) {
	s.T.Helper()
	command := []string{
		"set -ex",
		"driver_version=$(sudo nvidia-smi --query-gpu=driver_version --format=csv,noheader | head -n1 | tr -d '[:space:]')",
		"echo \"nvidia driver_version=$driver_version\"",
		"case \"$driver_version\" in 580.*) ;; *) echo \"expected R580 (580.x) driver, got '$driver_version'\"; exit 1 ;; esac",
		// Open kernel module: /proc/driver/nvidia/version reports the flavor.
		"ver=$(cat /proc/driver/nvidia/version)",
		"echo \"$ver\"",
		"echo \"$ver\" | grep -qi 'Open Kernel Module' || { echo 'expected NVIDIA OPEN kernel module (Blackwell is open-only)'; exit 1; }",
	}
	execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0, "expected managed R580 open-module GPU driver on GB300")
}
