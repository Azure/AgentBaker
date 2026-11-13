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

Describe 'cache-warmup registration after package upgrades'
  Include "./parts/linux/cloud-init/artifacts/cse_helpers.sh"

  setup_cache_warmup() {
    TEST_DIR=$(mktemp -d)
    export TEST_DIR
    export MOCK_RELOAD_FAILURES=1
    unset CSE_STARTTIME_SECONDS
    printf '0' > "${TEST_DIR}/reload-count"
    sed -n '/^if { isUbuntu /,/^fi$/p' ./vhdbuilder/packer/pre-install-dependencies.sh > "${TEST_DIR}/warmup.sh"
    export -f isUbuntu isAzureLinux tee systemctl timeout sleep retrycmd_if_failure _retrycmd_internal
  }

  cleanup_cache_warmup() {
    rm -f -- "${TEST_DIR}/reload-count" "${TEST_DIR}/warmup.sh" \
      "${TEST_DIR}/cache-warmup.service" "${TEST_DIR}/enabled-service"
    rmdir -- "$TEST_DIR"
  }

  isUbuntu() { [ "$1" = "UBUNTU" ]; }
  isAzureLinux() { [ "$1" = "AZURELINUX" ]; }
  timeout() { shift; "$@"; }
  sleep() { :; }

  tee() {
    if [ "$1" = "-a" ]; then
      cat > /dev/null
    else
      cat > "${TEST_DIR}/cache-warmup.service"
    fi
  }

  systemctl() {
    if [ "$1" = "daemon-reload" ]; then
      local attempts
      attempts=$(cat "${TEST_DIR}/reload-count")
      attempts=$((attempts + 1))
      printf '%s' "$attempts" > "${TEST_DIR}/reload-count"
      if [ "$attempts" -le "$MOCK_RELOAD_FAILURES" ]; then
        echo "Failed to connect to system scope bus via local transport: Connection refused" >&2
        return 1
      fi
    elif [ "$1" = "enable" ]; then
      printf '%s' "$2" > "${TEST_DIR}/enabled-service"
    else
      return 1
    fi
  }

  BeforeEach 'setup_cache_warmup'
  AfterEach 'cleanup_cache_warmup'

  Describe 'transient reload failures'
    Parameters
      "UBUNTU"
      "AZURELINUX"
    End

    It "enables cache-warmup after systemd recovers on $1"
      export OS="$1"
      When run bash -e "${TEST_DIR}/warmup.sh"
      The status should be success
      The error should include "Failed to connect to system scope bus"
      The output should include 'Executed "systemctl daemon-reload" 2 times.'
      The contents of file "${TEST_DIR}/enabled-service" should equal "cache-warmup.service"
    End
  End

  It 'fails without enabling cache-warmup when the systemd manager does not recover'
    export OS="UBUNTU"
    export MOCK_RELOAD_FAILURES=100
    When run bash -e "${TEST_DIR}/warmup.sh"
    The status should be failure
    The error should include "Failed to connect to system scope bus"
    The path "${TEST_DIR}/enabled-service" should not be exist
  End
End
