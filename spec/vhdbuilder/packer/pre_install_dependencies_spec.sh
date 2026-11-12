#!/bin/bash
# shellcheck disable=SC2329

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
