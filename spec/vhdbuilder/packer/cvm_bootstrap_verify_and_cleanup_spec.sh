#!/bin/bash
# shellcheck disable=SC2329,SC2317

# Tests for the pure functions in vhdbuilder/packer/cvm-bootstrap-verify-and-cleanup.sh
# (CVM Stage 1 / bootstrap, post-reboot half). The script is guarded so that
# sourcing it (via Include) never executes main().

Describe 'cvm-bootstrap-verify-and-cleanup.sh'
  Include './vhdbuilder/packer/cvm-bootstrap-verify-and-cleanup.sh'

  Describe 'assertRunningAzureFdeKernel'
    It 'succeeds for a kernel release ending in -azure-fde'
      When call assertRunningAzureFdeKernel "6.14.0-1008-azure-fde"
      The status should be success
      The output should include "expected azure-fde suffix"
    End

    It 'fails for the vanilla (non-fde) azure kernel'
      When run assertRunningAzureFdeKernel "6.14.0-1008-azure"
      The status should be failure
      The stderr should include "does not have the expected '-azure-fde' suffix"
    End

    It 'fails for a generic (non-azure) kernel'
      When run assertRunningAzureFdeKernel "6.8.0-1021-generic"
      The status should be failure
      The stderr should include "does not have the expected '-azure-fde' suffix"
    End
  End

  Describe 'assertEfiBoot'
    It 'succeeds when the given EFI path exists'
      efi_dir="$(mktemp -d)"
      When call assertEfiBoot "${efi_dir}"
      The status should be success
      The output should include "present: VM booted in UEFI mode"
      rm -rf "${efi_dir}"
    End

    It 'fails when the given EFI path does not exist'
      When run assertEfiBoot "/nonexistent-efi-path-for-tests"
      The status should be failure
      The stderr should include "is missing; the VM did not boot in UEFI mode"
    End
  End

  Describe 'assertPackageStateClean'
    It 'succeeds when dpkg --audit is empty and apt-get check passes'
      # shellcheck disable=SC2329
      dpkg() { [ "$1" = "--audit" ] && return 0; }
      # shellcheck disable=SC2329
      apt-get() { [ "$1" = "check" ] && return 0; }
      When call assertPackageStateClean
      The status should be success
      The output should include "dpkg --audit: clean"
      The output should include "apt-get check: clean"
    End

    It 'fails when dpkg --audit reports inconsistent packages'
      # shellcheck disable=SC2329
      dpkg() { echo "some-package is in an inconsistent state"; }
      # shellcheck disable=SC2329
      apt-get() { return 0; }
      When run assertPackageStateClean
      The status should be failure
      The stderr should include "dpkg --audit reported packages in an inconsistent state"
    End

    It 'fails when apt-get check reports broken dependencies'
      # shellcheck disable=SC2329
      dpkg() { return 0; }
      # shellcheck disable=SC2329
      apt-get() { return 1; }
      When run assertPackageStateClean
      The status should be failure
      The stdout should include "dpkg --audit: clean"
      The stderr should include "apt-get check reported broken dependencies"
    End
  End

  Describe 'computeSafeKernelPurgeList'
    setup_dpkg_query_mock() {
      # Args become the fake set of installed 'linux-*' packages, one per
      # invocation of dpkg-query -W -f='${Package}\n' ... , OR the fake
      # per-package status check dpkg-query -W -f='${Status}' <pkg>.
      # shellcheck disable=SC2329
      dpkg-query() {
        case "$*" in
          *'${Package}'*)
            printf '%s\n' "${MOCK_INSTALLED_KERNEL_PACKAGES[@]}"
            ;;
          *'${Status}'*)
            local queried_pkg="${*: -1}"
            for pkg in "${MOCK_INSTALLED_METAPACKAGES[@]}"; do
              if [ "${pkg}" = "${queried_pkg}" ]; then
                echo "install ok installed"
                return 0
              fi
            done
            return 1
            ;;
        esac
      }
    }

    BeforeEach 'setup_dpkg_query_mock'

    It 'purges only packages tied to the prior kernel version'
      MOCK_INSTALLED_KERNEL_PACKAGES=(
        "linux-image-6.8.0-1050-azure"
        "linux-modules-6.8.0-1050-azure"
        "linux-image-6.8.0-1061-azure-fde"
      )
      MOCK_INSTALLED_METAPACKAGES=()
      When call computeSafeKernelPurgeList "6.8.0-1050-azure" "6.8.0-1061-azure-fde" "24.04"
      The status should be success
      The output should include "linux-image-6.8.0-1050-azure"
      The output should include "linux-modules-6.8.0-1050-azure"
      The output should not include "6.8.0-1061-azure-fde"
    End

    It 'includes installed non-FDE kernel metapackages'
      MOCK_INSTALLED_KERNEL_PACKAGES=()
      MOCK_INSTALLED_METAPACKAGES=("linux-azure-lts-26.04" "linux-image-azure-lts-26.04")
      When call computeSafeKernelPurgeList "6.14.0-1001-azure" "6.14.0-1008-azure-fde" "26.04"
      The status should be success
      The output should include "linux-azure-lts-26.04"
      The output should include "linux-image-azure-lts-26.04"
    End

    It 'never includes a package matching the currently running kernel, even if matched'
      # Realistic overlap: the azure-fde flavor often shares the exact same
      # upstream version/ABI as the vanilla azure kernel it replaces, so a
      # package name tied to the prior kernel can be a substring of the
      # current (azure-fde) kernel's package name.
      MOCK_INSTALLED_KERNEL_PACKAGES=(
        "linux-image-6.14.0-1008-azure-fde"
      )
      MOCK_INSTALLED_METAPACKAGES=()
      When call computeSafeKernelPurgeList "6.14.0-1008-azure" "6.14.0-1008-azure-fde" "26.04"
      The status should be success
      The output should not include "linux-image-6.14.0-1008-azure-fde"
      The stderr should include "Refusing to purge"
    End

    It 'fails when the prior kernel is empty'
      MOCK_INSTALLED_KERNEL_PACKAGES=()
      MOCK_INSTALLED_METAPACKAGES=()
      When call computeSafeKernelPurgeList "" "6.14.0-1008-azure-fde" "26.04"
      The status should be failure
      The stderr should include "prior kernel version is empty"
    End

    It 'fails when the prior kernel equals the current kernel (reboot did not take effect)'
      MOCK_INSTALLED_KERNEL_PACKAGES=()
      MOCK_INSTALLED_METAPACKAGES=()
      When call computeSafeKernelPurgeList "6.14.0-1008-azure-fde" "6.14.0-1008-azure-fde" "26.04"
      The status should be failure
      The stderr should include "refusing to purge anything"
    End
  End
End
