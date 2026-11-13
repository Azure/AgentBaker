#!/bin/bash
# shellcheck disable=SC2329

Describe 'reloadSystemdAfterPackageMaintenance'
  BeforeAll "eval \"\$(sed -n '/^reloadSystemdAfterPackageMaintenance()/,/^}/p' './vhdbuilder/packer/pre-install-dependencies.sh')\""

  setup_daemon_reload() {
    TEST_DIR="${SHELLSPEC_WORKDIR}/daemon-reload.${SHELLSPEC_EXAMPLE_NO}"
    mkdir -p "$TEST_DIR"
    RELOAD_CALLS="${TEST_DIR}/systemctl"
    TIMEOUT_CALLS="${TEST_DIR}/timeout"
    SLEEP_CALLS="${TEST_DIR}/sleep"
    : > "$RELOAD_CALLS"
    : > "$TIMEOUT_CALLS"
    : > "$SLEEP_CALLS"
    RELOAD_MODE=success
    unset SECONDS
    SECONDS=0
  }
  BeforeEach 'setup_daemon_reload'

  timeout() {
    echo "LC_ALL=$LC_ALL timeout $*" >> "$TIMEOUT_CALLS"
    if [ "$RELOAD_MODE" = "unresponsive" ]; then
      echo 'mock daemon-reload exceeded its timeout' >&2
      return "$MOCK_TIMEOUT_STATUS"
    fi
    shift 2
    "$@"
  }

  systemctl() {
    echo "$*" >> "$RELOAD_CALLS"
    case "$RELOAD_MODE" in
      success) return 0 ;;
      recovering)
        if [ "$(wc -l < "$RELOAD_CALLS")" -gt 1 ]; then
          return 0
        fi
        ;;
      unrelated)
        echo "$MOCK_FAILURE_MESSAGE" >&2
        return "$MOCK_FAILURE_STATUS"
        ;;
    esac
    echo 'Failed to connect to system scope bus via local transport: Connection refused' >&2
    return 1
  }

  sleep() {
    echo "sleep $*" >> "$SLEEP_CALLS"
    SECONDS=$((SECONDS + $1))
  }

  It 'continues on immediate success without sleeping or retrying'
    When call reloadSystemdAfterPackageMaintenance
    The status should be success
    The output should equal ''
    The contents of file "$RELOAD_CALLS" should equal 'daemon-reload'
    The contents of file "$TIMEOUT_CALLS" should equal 'LC_ALL=C timeout --signal=KILL 30s systemctl daemon-reload'
    The contents of file "$SLEEP_CALLS" should equal ''
  End

  It 'retries the actual reload after a transient refusal and reports recovery'
    RELOAD_MODE=recovering
    When call reloadSystemdAfterPackageMaintenance
    The status should be success
    The output should include 'Failed to connect to system scope bus via local transport: Connection refused'
    The output should include 'attempt 1 failed (exit 1, elapsed 0s)'
    The output should include 'systemctl daemon-reload recovered on attempt 2 after 1s.'
    The contents of file "$RELOAD_CALLS" should equal "$(printf 'daemon-reload\ndaemon-reload')"
    The contents of file "$SLEEP_CALLS" should equal 'sleep 1'
  End

  Describe 'when the reload fails for an unrelated reason'
    Parameters
      1 'Failed to connect to a different bus: Connection refused'
      42 'Failed to reload daemon: Access denied'
    End
    It 'stops immediately and preserves the failure status'
      RELOAD_MODE=unrelated
      MOCK_FAILURE_STATUS=$1
      MOCK_FAILURE_MESSAGE=$2
      When call reloadSystemdAfterPackageMaintenance
      The status should equal "$1"
      The output should include "$2"
      The output should include "attempt 1 failed (exit $1, elapsed 0s)"
      The contents of file "$RELOAD_CALLS" should equal 'daemon-reload'
      The contents of file "$SLEEP_CALLS" should equal ''
    End
  End

  It 'exhausts persistent refusals within the deadline and caps the final attempt to the remaining budget'
    RELOAD_MODE=refused
    When call reloadSystemdAfterPackageMaintenance
    The status should equal 124
    The output should include 'attempt 60 failed (exit 1, elapsed 59s)'
    The output should include 'exhausted its 60-second window after 60 attempts'
    The output should not include 'attempt 61'
    The contents of file "$TIMEOUT_CALLS" should include 'timeout --signal=KILL 1s systemctl daemon-reload'
    The output should not include 'recovered'
  End

  Describe 'when the actual reload is unresponsive'
    Parameters:value 124 137
    It 'fails on the bounded call without retrying and preserves the timeout status'
      RELOAD_MODE=unresponsive
      MOCK_TIMEOUT_STATUS=$1
      When call reloadSystemdAfterPackageMaintenance
      The status should equal "$1"
      The output should include 'mock daemon-reload exceeded its timeout'
      The output should include "attempt 1 failed (exit $1, elapsed 0s)"
      The contents of file "$TIMEOUT_CALLS" should equal 'LC_ALL=C timeout --signal=KILL 30s systemctl daemon-reload'
      The contents of file "$SLEEP_CALLS" should equal ''
    End
  End
End

Describe 'installAzureLinuxArm64DualKernel'
  BeforeAll "eval \"\$(sed -n '/^installAzureLinuxArm64DualKernel()/,/^}/p' './vhdbuilder/packer/pre-install-dependencies.sh')\""

  setup_dual_kernel() {
    TEST_DIR=$(mktemp -d)
    BOOT_DIR="${TEST_DIR}/boot"
    GRUB_MODULE_SOURCE="${TEST_DIR}/grub-modules"
    MOCK_KERNEL_HWE_INSTALLED=false
    MOCK_GRUB_EFI_BINARY_VERSION="2.06-27.azl3"

    mkdir -p "$GRUB_MODULE_SOURCE" "$BOOT_DIR/grub2"
    echo "smbios" > "$GRUB_MODULE_SOURCE/smbios.mod"
  }

  cleanup_dual_kernel() {
    rm -rf "$TEST_DIR"
  }

  dnf_install() {
    echo "dnf_install $*"
    if [ "${*: -1}" = "kernel-hwe" ]; then
      MOCK_KERNEL_HWE_INSTALLED=true
    fi
  }

  rpm() {
    if [ "$1" = "-ql" ]; then
      echo "/boot/vmlinuz-$2"
      return 0
    fi
    if [ "$1" = "-q" ] && [ "$2" = "--queryformat" ]; then
      case "${*: -1}" in
        grub2-efi-binary) echo "$MOCK_GRUB_EFI_BINARY_VERSION" ;;
        grub2|grub2-efi) echo "2.06-27.azl3" ;;
      esac
      return 0
    fi
    if [ "$1" = "-q" ] && [ "$2" = "kernel-hwe" ] && [ "$MOCK_KERNEL_HWE_INSTALLED" != "true" ]; then
      return 1
    fi
    return 0
  }

  grub2-mkconfig() {
    echo "grub2-mkconfig $*"
    touch "$2"
  }

  BeforeEach 'setup_dual_kernel'
  AfterEach 'cleanup_dual_kernel'

  It 'installs HWE and package-owned GRUB modules without staging copies'
    When call installAzureLinuxArm64DualKernel "$BOOT_DIR" "$GRUB_MODULE_SOURCE"

    The status should be success
    The output should include "dnf_install 30 1 600 kernel-hwe"
    The output should include "dnf_install 30 1 600 grub2-efi"
    The output should include "grub2-mkconfig -o ${BOOT_DIR}/grub2/grub.cfg"
    The contents of file "${GRUB_MODULE_SOURCE}/smbios.mod" should equal "smbios"
    The path "${BOOT_DIR}/grub2/arm64-efi" should not be exist
  End

  Parameters
    "mismatched GRUB versions"  "GRUB package versions do not match"
    "missing SMBIOS module" "smbios.mod is missing"
  End

  It "rejects $1"
    case "$1" in
      "mismatched GRUB versions") MOCK_GRUB_EFI_BINARY_VERSION="2.06-28.azl3" ;;
      "missing SMBIOS module") rm "$GRUB_MODULE_SOURCE/smbios.mod" ;;
    esac

    When call installAzureLinuxArm64DualKernel "$BOOT_DIR" "$GRUB_MODULE_SOURCE"

    The status should be failure
    The error should include "$2"
    The output should not include "grub2-mkconfig"
    The path "${BOOT_DIR}/grub2/arm64-efi" should not be exist
  End
End
