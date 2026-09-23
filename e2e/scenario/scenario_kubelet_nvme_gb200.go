package scenario

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
)

const (
	gb200VMSizeEnv          = "GB200_VM_SIZE"
	gb200KubeletMarker      = "/var/lib/kubelet/.agentbaker-e2e-gb200-reboot-marker"
	gb200PermissionTestFile = "/var/lib/kubelet/.agentbaker-e2e-unprivileged-write-test"
)

var ubuntu2404ARM64GB200KubeletNVMePermissions = Register(&Scenario{
	Name:        "Ubuntu2404_ARM64_GB200_KubeletNVMePermissions",
	Description: "Validates the GB200 NVMe RAID kubelet mount and permissions before and after reboot",
	SkipIf:      skipIfGB200VMSizeMissing,
	Tags: Tags{
		GPU: true,
	},
	Config: Config{
		Cluster:               ClusterKubenet,
		VHD:                   config.VHDUbuntu2404ArmGBContainerd,
		WaitForSSHAfterReboot: 15 * time.Minute,
		VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
			vmss.SKU.Name = to.Ptr(gb200VMSize())
		},
		BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
			vmSize := gb200VMSize()
			nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize = vmSize
			nbc.AgentPoolProfile.VMSize = vmSize
			nbc.IsARM64 = true
		},
		Validator: validateGB200KubeletNVMeAfterReboot,
	},
})

func gb200VMSize() string {
	return strings.TrimSpace(os.Getenv(gb200VMSizeEnv))
}

func skipIfGB200VMSizeMissing(context.Context) string {
	if gb200VMSize() == "" {
		return gb200VMSizeEnv + " must identify a GB200 VM SKU with four local NVMe disks"
	}
	return ""
}

func validateGB200KubeletNVMeAfterReboot(ctx context.Context, s *Scenario) error {
	if err := validateGB200KubeletNVMe(ctx, s, false); err != nil {
		return fmt.Errorf("validate GB200 kubelet NVMe mount before reboot: %w", err)
	}
	if err := createGB200RebootMarker(ctx, s); err != nil {
		return fmt.Errorf("create GB200 kubelet reboot marker: %w", err)
	}
	if err := RebootVMAndWaitForSSH(ctx, s); err != nil {
		return fmt.Errorf("reboot VM: %w", err)
	}
	if err := validateGB200KubeletNVMe(ctx, s, true); err != nil {
		return fmt.Errorf("validate GB200 kubelet NVMe mount after reboot: %w", err)
	}
	return nil
}

func validateGB200KubeletNVMe(ctx context.Context, s *Scenario, expectRebootMarker bool) error {
	markerCheck := fmt.Sprintf("test ! -e %q", gb200KubeletMarker)
	if expectRebootMarker {
		markerCheck = fmt.Sprintf("test -f %q\nrm -f %q", gb200KubeletMarker, gb200KubeletMarker)
	}

	script := fmt.Sprintf(`set -eux
systemctl is-active format-mount-nvme-root.service
systemctl is-active kubelet
test -b /dev/md0
test "$(mdadm --detail /dev/md0 | awk -F: '/^[[:space:]]*Raid Level[[:space:]]*:/ { gsub(/[[:space:]]/, "", $2); print $2 }')" = "raid0"
test "$(mdadm --detail /dev/md0 | awk -F: '/^[[:space:]]*Active Devices[[:space:]]*:/ { gsub(/[[:space:]]/, "", $2); print $2 }')" = "4"
mountpoint -q /mnt/aks
mountpoint -q /var/lib/kubelet
findmnt /mnt/aks
findmnt /var/lib/kubelet
test "$(findmnt -rn -o SOURCE --target /mnt/aks)" = "/dev/md0"
case "$(findmnt -rn -o SOURCE --target /var/lib/kubelet)" in
    /dev/md0*) ;;
    *) exit 1 ;;
esac
stat -c '%%A %%a %%U:%%G %%n' /var/lib/kubelet
test "$(stat -c '%%U:%%G' /var/lib/kubelet)" = "root:root"
test -z "$(find /var/lib/kubelet -maxdepth 0 -perm /022 -print -quit)"
if sudo -u nobody touch %q; then
    rm -f %q
    exit 1
fi
%s`, gb200PermissionTestFile, gb200PermissionTestFile, markerCheck)

	result, err := execScriptOnVMForScenarioValidateExitCode(
		ctx,
		s,
		script,
		0,
		"GB200 kubelet NVMe mount or directory permissions are invalid",
	)
	if err != nil {
		return err
	}
	s.Logger.Logf("GB200 kubelet NVMe validation:\n%s", result.stdout)
	return nil
}

func createGB200RebootMarker(ctx context.Context, s *Scenario) error {
	result, err := execScriptOnVMForScenarioValidateExitCode(
		ctx,
		s,
		fmt.Sprintf("set -eux\nprintf 'persisted across reboot\\n' > %q\nsync", gb200KubeletMarker),
		0,
		"failed to create GB200 kubelet reboot marker",
	)
	if err != nil {
		return err
	}
	s.Logger.Logf("created GB200 kubelet reboot marker:\n%s", result.stdout)
	return nil
}
