package e2e

import (
	"context"
	"strings"

	"github.com/stretchr/testify/require"
)

// This file holds validators specific to Grace-Blackwell (GB200/GB300) nodes. The GB fabric model is
// "ND - Fabric Manager + IMEX": GB has NO on-board NVSwitch (the switch trays live in the rack, run by
// NVOS), so host Fabric Manager must stay OFF, and cross-node NVLink is set up via IMEX instead. On the
// host, CSE installs nvidia-imex + creates the IMEX channel, but deliberately does NOT start the daemon
// (the per-domain peer list is owned at runtime by the GPU Operator ComputeDomains). These validators
// assert exactly that shape.

// ValidateNvidiaOpenKernelModuleDriver asserts the loaded NVIDIA driver is the OPEN kernel module.
// Blackwell is open-only, and the aks-gpu cuda-lts image ships the open R580 build; this confirms the
// driver actually came up as the open module (and, implicitly, that aks-gpu DKMS-built it on arm64).
func ValidateNvidiaOpenKernelModuleDriver(ctx context.Context, s *Scenario) {
	s.T.Helper()
	s.T.Logf("validating that the loaded NVIDIA driver is the OPEN kernel module (Blackwell is open-only)")
	command := []string{"set -ex", "cat /proc/driver/nvidia/version"}
	execResult := execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0, "could not read /proc/driver/nvidia/version")
	require.Contains(s.T, execResult.stdout, "Open Kernel Module", "expected the OPEN NVIDIA kernel module on GB, but got: %s", execResult.stdout)
}

// ValidateIMEXChannelDeviceExists asserts the IMEX channel device is present. It is a CHARACTER
// device created by the nvidia kernel module once NVreg_CreateImexChannel0=1 is set (after the
// provisioning reboot), independent of the imex daemon -- so this must use `test -c`, not `test -f`
// (ValidateFileExists uses `test -f`, which is always false for a device node).
func ValidateIMEXChannelDeviceExists(ctx context.Context, s *Scenario) {
	s.T.Helper()
	s.T.Logf("validating that the IMEX channel character device exists (channel created, daemon not required)")
	execScriptOnVMForScenarioValidateExitCode(ctx, s, "test -c /dev/nvidia-caps-imex-channels/channel0", 0, "IMEX channel device /dev/nvidia-caps-imex-channels/channel0 should exist (char device)")
}

// ValidateIMEXCacheRemoved asserts the baked install cache directory was cleaned up after install.
// /opt/nvidia-imex is a DIRECTORY, so this must use `test -e` (ValidateFileDoesNotExist uses `test -f`,
// which is always false for a directory and would pass regardless -- a silent no-op).
func ValidateIMEXCacheRemoved(ctx context.Context, s *Scenario) {
	s.T.Helper()
	s.T.Logf("validating that the baked IMEX install cache /opt/nvidia-imex was removed after install")
	execScriptOnVMForScenarioValidateExitCode(ctx, s, "test ! -e /opt/nvidia-imex", 0, "IMEX install cache /opt/nvidia-imex should be removed after install")
}

// ValidateFabricManagerNotRunning asserts nvidia-fabricmanager is NOT active. Starting Fabric Manager on
// GB fails provisioning (there is no on-board NVSwitch for it to program), so on GB it must never run --
// the inverse of HGX A100/H100/H200 where FM is required.
func ValidateFabricManagerNotRunning(ctx context.Context, s *Scenario) {
	s.T.Helper()
	s.T.Logf("validating that nvidia-fabricmanager is NOT running (GB has no on-board NVSwitch)")
	// is-active exits 0 only when the unit is active; invert that so the validator's exit-0 means "not running".
	command := "if systemctl is-active --quiet nvidia-fabricmanager; then echo FM_ACTIVE; exit 1; fi; echo FM_NOT_ACTIVE"
	execScriptOnVMForScenarioValidateExitCode(ctx, s, command, 0, "nvidia-fabricmanager must NOT run on GB (Fabric Manager is off)")
}

// ValidateNvidiaIMEXInstalledButDisabled asserts nvidia-imex is installed from the baked cache but the
// daemon is NOT started/enabled. CSE lays down the package + the IMEX channel; the daemon and its
// per-domain peer list are owned at runtime by the GPU Operator ComputeDomains, so the host unit stays
// disabled and inactive.
func ValidateNvidiaIMEXInstalledButDisabled(ctx context.Context, s *Scenario) {
	s.T.Helper()
	s.T.Logf("validating nvidia-imex is installed but the daemon is disabled (channel-only; peer list owned by ComputeDomains)")
	command := []string{
		"set -x",
		"dpkg -s nvidia-imex >/dev/null 2>&1 && echo IMEX_PKG_PRESENT || echo IMEX_PKG_MISSING",
		"systemctl is-enabled nvidia-imex 2>/dev/null || true",
		"systemctl is-active nvidia-imex 2>/dev/null || true",
	}
	execResult := execScriptOnVMForScenario(ctx, s, strings.Join(command, "\n"))
	require.Contains(s.T, execResult.stdout, "IMEX_PKG_PRESENT", "nvidia-imex should be installed from the baked cache; got: %s", execResult.stdout)
	require.Contains(s.T, execResult.stdout, "disabled", "nvidia-imex service should be disabled (CSE must not start the daemon); got: %s", execResult.stdout)
	require.Contains(s.T, execResult.stdout, "inactive", "nvidia-imex daemon should NOT be running; got: %s", execResult.stdout)
}
