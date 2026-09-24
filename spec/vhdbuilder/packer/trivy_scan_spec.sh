#!/bin/bash
# shellcheck disable=SC2329,SC2317

Describe 'install_trivy'
  BeforeAll "eval \"\$(sed -n '/^install_trivy()/,/^}$/p' vhdbuilder/packer/trivy-scan.sh)\""
  Include './parts/linux/cloud-init/artifacts/ubuntu/cse_install_ubuntu.sh'
  Include './parts/linux/cloud-init/artifacts/mariner/cse_install_mariner.sh'

  TRIVY_PMC_VERSION=0.72.0

  source() {
    [ "$1" = /opt/azure/containers/provision_installs_distro.sh ]
  }

  getCPUArch() { echo "${TEST_ARCH}"; }
  apt_get_update() { echo "apt update"; }
  apt_get_install() { echo "apt install $*"; }
  dnf_install() { echo "dnf install $*"; }
  install_trivy_from_github() { echo "GitHub fallback"; }

  apt() {
    echo 'Listing...'
    echo "trivy/prod 0.72.0-ubuntu${TEST_OS_VERSION}u9 ${TEST_ARCH}"
    echo "trivy/prod 0.72.0-ubuntu${TEST_OS_VERSION}u12 ${TEST_ARCH}"
    echo "trivy/prod 0.72.0-ubuntu${TEST_OS_VERSION}u99 other-arch"
    echo "trivy/prod 0.72.1-ubuntu${TEST_OS_VERSION}u1 ${TEST_ARCH}"
    echo "trivy/prod 0.72.00-ubuntu${TEST_OS_VERSION}u1 ${TEST_ARCH}"
  }

  dnf() {
    echo 'Available Packages'
    echo 'trivy.x86_64 0.72.0-9.azl3 cloud-native'
    echo 'trivy.x86_64 0.72.0-12.azl3 cloud-native'
    echo 'trivy.x86_64 0.72.1-1.azl3 cloud-native'
    echo 'trivy.x86_64 0.72.00-1.azl3 cloud-native'
  }

  VHD_LOGS_FILEPATH=""

  Describe 'Ubuntu'
    Parameters:matrix
      20.04 22.04 24.04 26.04
      amd64 arm64
    End

    It "installs the latest matching Ubuntu $1 revision on $2"
      TEST_OS_VERSION="$1"
      TEST_ARCH="$2"
      When call install_trivy Ubuntu "$1"
      The status should be success
      The line 1 of output should eq "apt update"
      The line 2 of output should eq "Resolved trivy package version 0.72.0 -> 0.72.0-ubuntu${1}u12"
      The line 3 of output should eq "apt install 5 1 60 trivy=0.72.0-ubuntu${1}u12"
    End
  End

  It 'installs the latest matching Azure Linux revision'
    When call install_trivy AzureLinux 3.0
    The status should be success
    The line 1 of output should eq "Resolved trivy package version 0.72.0 -> 0.72.0-12.azl3"
    The line 2 of output should eq "dnf install 5 1 60 trivy-0.72.0-12.azl3"
  End

  It 'fails without installing when the DEB version is unavailable'
    TEST_ARCH=amd64
    apt() { echo 'Listing...'; }
    When call install_trivy Ubuntu 22.04
    The status should be failure
    The output should eq "apt update"
    The stderr should eq "Failed to resolve trivy deb revision for 0.72.0"
  End

  It 'propagates DEB query failures without installing'
    TEST_ARCH=amd64
    apt() { return 1; }
    install_with_pipefail() {
      set -o pipefail
      install_trivy Ubuntu 22.04
    }
    When run install_with_pipefail
    The status should be failure
    The output should eq "apt update"
  End

  It 'fails without installing when the RPM version is unavailable'
    dnf() { echo 'No matching packages'; return 1; }
    When call install_trivy AzureLinux 3.0
    The status should be failure
    The stderr should include 'Failed to query trivy versions'
  End

  Describe 'GitHub fallback'
    Parameters
      Flatcar 1
      AzureContainerLinux 3.0
      AzureLinuxOSGuard 3.0
      CBLMariner 2.0
      Ubuntu unsupported
    End

    It "preserves the GitHub fallback for $1 $2"
      When call install_trivy "$1" "$2"
      The status should be success
      The line 1 of output should include 'downloading from GitHub'
      The line 2 of output should eq 'GitHub fallback'
    End
  End
End
