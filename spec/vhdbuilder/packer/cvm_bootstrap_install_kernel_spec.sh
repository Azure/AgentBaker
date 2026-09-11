#!/bin/bash
# shellcheck disable=SC2329,SC2317

# Tests for the pure functions in vhdbuilder/packer/cvm-bootstrap-install-kernel.sh
# (CVM Stage 1 / bootstrap, pre-reboot half). The script is guarded so that
# sourcing it (via Include) never executes main().

Describe 'cvm-bootstrap-install-kernel.sh'
  Include './vhdbuilder/packer/cvm-bootstrap-install-kernel.sh'

  Describe 'isNullbootInstalled / failIfNullbootPresent'
    mock_dpkg_query_installed() {
      # shellcheck disable=SC2329
      dpkg-query() { echo 'install ok installed'; }
    }
    mock_dpkg_query_absent() {
      # shellcheck disable=SC2329
      dpkg-query() { return 1; }
    }
    mock_dpkg_query_removed() {
      # nullboot was purged but dpkg still has a "deinstall" record
      # shellcheck disable=SC2329
      dpkg-query() { echo 'deinstall ok config-files'; }
    }

    It 'reports nullboot installed when dpkg-query shows "install ok installed"'
      mock_dpkg_query_installed
      When call isNullbootInstalled
      The status should be success
    End

    It 'reports nullboot not installed when dpkg-query has no record'
      mock_dpkg_query_absent
      When call isNullbootInstalled
      The status should be failure
    End

    It 'reports nullboot not installed when only removed (config-files) remains'
      mock_dpkg_query_removed
      When call isNullbootInstalled
      The status should be failure
    End

    It 'fails the build with a clear message when nullboot is present'
      mock_dpkg_query_installed
      When run failIfNullbootPresent "test context"
      The status should be failure
      The stderr should include "nullboot is installed (test context)"
      The stderr should include "GRUB-managed boot chain"
    End

    It 'continues without error when nullboot is absent'
      mock_dpkg_query_absent
      When call failIfNullbootPresent "test context"
      The status should be success
      The output should include "not installed"
    End
  End

  Describe 'buildFdeKernelPackageList'
    It 'includes the modules-extra package when available in the apt cache'
      # shellcheck disable=SC2329
      apt-cache() { return 0; }
      When call buildFdeKernelPackageList "26.04"
      The status should be success
      The line 1 of output should eq "linux-image-azure-fde-lts-26.04"
      The line 2 of output should eq "linux-tools-azure-lts-26.04"
      The line 3 of output should eq "linux-cloud-tools-azure-lts-26.04"
      The line 4 of output should eq "linux-headers-azure-lts-26.04"
      The line 5 of output should eq "linux-modules-extra-azure-lts-26.04"
    End

    It 'omits the modules-extra package when unavailable in the apt cache'
      # shellcheck disable=SC2329
      apt-cache() { return 1; }
      When call buildFdeKernelPackageList "26.04"
      The status should be success
      The lines of stdout should eq 4
      The output should not include "modules-extra"
      The stderr should include "not available - skipping"
    End

    It 'only the image package carries the -fde- infix'
      # shellcheck disable=SC2329
      apt-cache() { return 0; }
      When call buildFdeKernelPackageList "24.04"
      The status should be success
      The output should include "linux-image-azure-fde-lts-24.04"
      The output should not include "linux-tools-azure-fde"
      The output should not include "linux-headers-azure-fde"
      The output should not include "linux-cloud-tools-azure-fde"
    End
  End

  Describe 'getUbuntuRelease'
    It 'reads VERSION_ID from /etc/os-release'
      When call getUbuntuRelease
      The status should be success
      # Whatever distro this happens to run on in CI, VERSION_ID must be non-empty.
      The output should not eq ""
    End
  End
End
