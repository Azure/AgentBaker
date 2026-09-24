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

const ubuntu2404ARM64KubeletDiskVMSize = "Standard_D2pds_V5"

var _ = Register(&Scenario{
	Name:        "Ubuntu2404_NonTemporaryKubeletDisk",
	Description: "Validates kubelet directory permissions on an Ubuntu 24.04 x64 non-Temporary disk before and after reboot",
	Config: Config{
		Cluster:                ClusterKubenet,
		VHD:                    config.VHDUbuntu2404Gen2Containerd,
		WaitForSSHAfterReboot:  10 * time.Minute,
		BootstrapConfigMutator: EmptyBootstrapConfigMutator,
		Validator:              validateNonTemporaryKubeletDiskAfterReboot,
	},
})

var _ = Register(&Scenario{
	Name:        "Ubuntu2404_ARM64_NonTemporaryKubeletDisk",
	Description: "Validates kubelet directory permissions on an Ubuntu 24.04 ARM64 non-Temporary disk before and after reboot",
	Config: Config{
		Cluster:               ClusterKubenet,
		VHD:                   config.VHDUbuntu2404ArmContainerd,
		WaitForSSHAfterReboot: 10 * time.Minute,
		VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
			vmss.SKU.Name = to.Ptr(ubuntu2404ARM64KubeletDiskVMSize)
		},
		BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
			setUbuntu2404ARM64KubeletDiskVMSize(nbc)
		},
		Validator: validateNonTemporaryKubeletDiskAfterReboot,
	},
})

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
			vmss.SKU.Name = to.Ptr(ubuntu2404ARM64KubeletDiskVMSize)
		},
		BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
			setUbuntu2404ARM64KubeletDiskVMSize(nbc)
			setTemporaryKubeletDisk(nbc)
		},
		Validator: validateTemporaryKubeletDiskAfterReboot,
	},
})

func setUbuntu2404ARM64KubeletDiskVMSize(nbc *datamodel.NodeBootstrappingConfiguration) {
	nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize = ubuntu2404ARM64KubeletDiskVMSize
	nbc.AgentPoolProfile.VMSize = ubuntu2404ARM64KubeletDiskVMSize
	nbc.IsARM64 = true
}

func setTemporaryKubeletDisk(nbc *datamodel.NodeBootstrappingConfiguration) {
	nbc.ContainerService.Properties.AgentPoolProfiles[0].KubeletDiskType = datamodel.TempDisk
	nbc.AgentPoolProfile.KubeletDiskType = datamodel.TempDisk
}

func validateNonTemporaryKubeletDiskAfterReboot(ctx context.Context, s *Scenario) error {
	return validateKubeletDiskAfterReboot(ctx, s, false)
}

func validateTemporaryKubeletDiskAfterReboot(ctx context.Context, s *Scenario) error {
	return validateKubeletDiskAfterReboot(ctx, s, true)
}

func validateKubeletDiskAfterReboot(ctx context.Context, s *Scenario, temporary bool) error {
	diskType := "non-Temporary"
	if temporary {
		diskType = "Temporary"
	}
	if err := validateKubeletDisk(ctx, s, temporary); err != nil {
		return fmt.Errorf("validate %s disk before reboot: %w", diskType, err)
	}
	if err := RebootVMAndWaitForSSH(ctx, s); err != nil {
		return fmt.Errorf("reboot VM: %w", err)
	}
	if err := validateKubeletDisk(ctx, s, temporary); err != nil {
		return fmt.Errorf("validate %s disk after reboot: %w", diskType, err)
	}
	return nil
}

func validateKubeletDisk(ctx context.Context, s *Scenario, temporary bool) error {
	mountAssertion := "if mountpoint -q /var/lib/kubelet; then exit 1; fi"
	diskType := "non-Temporary"
	if temporary {
		mountAssertion = "mountpoint -q /var/lib/kubelet"
		diskType = "Temporary"
	}

	result, err := execScriptOnVMForScenarioValidateExitCode(
		ctx,
		s,
		fmt.Sprintf(`set -eux
%s
findmnt -T /var/lib/kubelet
stat -c '%%U:%%G %%a' /var/lib/kubelet
test "$(stat -c '%%U:%%G' /var/lib/kubelet)" = "root:root"
test -z "$(find /var/lib/kubelet -maxdepth 0 -perm /022 -print -quit)"
if sudo -u nobody touch /var/lib/kubelet/e2e-unprivileged-write-test; then
    sudo rm -f /var/lib/kubelet/e2e-unprivileged-write-test
    exit 1
fi
systemctl is-active kubelet`, mountAssertion),
		0,
		"kubelet directory permissions are unsafe",
	)
	if err != nil {
		return err
	}
	s.Logger.Logf("kubelet %s disk validation:\n%s", diskType, result.stdout)
	return nil
}
