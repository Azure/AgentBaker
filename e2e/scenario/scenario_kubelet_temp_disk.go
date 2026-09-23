package scenario

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
)

const ubuntu2404ARM64TempDiskVMSize = "Standard_D2pds_V5"

var _ = Register(&Scenario{
	Name:        "Ubuntu2404_TemporaryKubeletDisk",
	Description: "Validates kubelet directory permissions on an Ubuntu 24.04 x64 Temporary disk before and after reboot",
	Config: Config{
		Cluster:               ClusterKubenet,
		VHD:                   config.VHDUbuntu2404Gen2Containerd,
		WaitForSSHAfterReboot: 10 * time.Minute,
		BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
			setTemporaryKubeletDisk(nbc)
		},
		Validator: validateTemporaryKubeletDiskAfterReboot,
	},
})

var _ = Register(&Scenario{
	Name:        "Ubuntu2404_ARM64_TemporaryKubeletDisk",
	Description: "Validates kubelet directory permissions on an Ubuntu 24.04 ARM64 Temporary disk before and after reboot",
	Config: Config{
		Cluster:               ClusterKubenet,
		VHD:                   config.VHDUbuntu2404ArmContainerd,
		WaitForSSHAfterReboot: 10 * time.Minute,
		VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
			vmss.SKU.Name = to.Ptr(ubuntu2404ARM64TempDiskVMSize)
		},
		BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
			nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize = ubuntu2404ARM64TempDiskVMSize
			nbc.AgentPoolProfile.VMSize = ubuntu2404ARM64TempDiskVMSize
			nbc.IsARM64 = true
			setTemporaryKubeletDisk(nbc)
		},
		Validator: validateTemporaryKubeletDiskAfterReboot,
	},
})

func setTemporaryKubeletDisk(nbc *datamodel.NodeBootstrappingConfiguration) {
	nbc.ContainerService.Properties.AgentPoolProfiles[0].KubeletDiskType = datamodel.TempDisk
	nbc.AgentPoolProfile.KubeletDiskType = datamodel.TempDisk
}

func validateTemporaryKubeletDiskAfterReboot(ctx context.Context, s *Scenario) error {
	if err := validateTemporaryKubeletDisk(ctx, s); err != nil {
		return fmt.Errorf("validate Temporary disk before reboot: %w", err)
	}
	if err := RebootVMAndWaitForSSH(ctx, s); err != nil {
		return fmt.Errorf("reboot VM: %w", err)
	}
	if err := validateTemporaryKubeletDisk(ctx, s); err != nil {
		return fmt.Errorf("validate Temporary disk after reboot: %w", err)
	}
	return nil
}

func validateTemporaryKubeletDisk(ctx context.Context, s *Scenario) error {
	result, err := execScriptOnVMForScenarioValidateExitCode(
		ctx,
		s,
		`set -eux
mountpoint -q /var/lib/kubelet
findmnt /var/lib/kubelet
stat -c '%A %a %U:%G %n' /var/lib/kubelet
test "$(stat -c '%U:%G' /var/lib/kubelet)" = "root:root"
test -z "$(find /var/lib/kubelet -maxdepth 0 -perm /022 -print -quit)"
if sudo -u nobody touch /var/lib/kubelet/e2e-unprivileged-write-test; then
    sudo rm -f /var/lib/kubelet/e2e-unprivileged-write-test
    exit 1
fi
systemctl is-active kubelet`,
		0,
		"kubelet directory permissions are unsafe",
	)
	if err != nil {
		return err
	}
	s.Logger.Logf("kubelet Temporary disk validation:\n%s", result.stdout)
	return nil
}
