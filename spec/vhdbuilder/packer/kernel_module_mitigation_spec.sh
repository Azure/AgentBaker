#!/bin/bash
# shellcheck disable=SC1090,SC2329

Describe 'AzureLinux vulnerable kernel module VHD bake-in'
  setup() {
    AZURELINUX_OS_NAME="AZURELINUX"
    AZURELINUX_KATA_OS_NAME="AZURELINUXKATA"
    AZURELINUX_OSGUARD_OS_VARIANT="OSGUARD"
    UBUNTU_OS_NAME="UBUNTU"
    ACL_OS_NAME="AZURECONTAINERLINUX"
    ACL_OS_VARIANT="AZURECONTAINERLINUX"
    MODPROBE_CIS_SRC="parts/linux/cloud-init/artifacts/modprobe-CIS.conf"
    MODPROBE_CIS_DEST="/etc/modprobe.d/CIS.conf"
    eval "$(sed -n '/^isACL()/,/^}/p; /^isAzureLinux()/,/^}/p; /^isAzureLinuxOSGuard()/,/^}/p; /^isUbuntu()/,/^}/p' parts/linux/cloud-init/artifacts/cse_helpers.sh)"
  }

  BeforeEach 'setup'

  bakeModprobeCISWithoutVulnerableModules() { echo "STRIP"; }
  cpAndMode() { echo "COPY:$*"; }

  bake() {
    OS="$1"
    OS_VARIANT="$2"
    OS_VERSION="$3"
    FEATURE_FLAGS="$4"
    unset IS_KATA
    # Execute the production gate, not a mirrored condition or the rest of VHD setup.
    # shellcheck disable=SC2016
    eval "$(sed -n '/^  if isUbuntu "\$OS" && ubuntuKernelIncludesVulnerableModuleFixes; then$/,/^  fi$/p' vhdbuilder/packer/packer_source.sh)"
  }

  Parameters
    "AZURELINUX" "" "3.0" "kata" "COPY:parts/linux/cloud-init/artifacts/modprobe-CIS.conf /etc/modprobe.d/CIS.conf 644"
    "AZURELINUX" "" "3.0" "cvm,kata" "COPY:parts/linux/cloud-init/artifacts/modprobe-CIS.conf /etc/modprobe.d/CIS.conf 644"
    "AZURELINUXKATA" "" "3.0" "" "COPY:parts/linux/cloud-init/artifacts/modprobe-CIS.conf /etc/modprobe.d/CIS.conf 644"
    "AZURELINUX" "OSGUARD" "3.0" "" "COPY:parts/linux/cloud-init/artifacts/modprobe-CIS.conf /etc/modprobe.d/CIS.conf 644"
    "AZURELINUX" "" "2.0" "" "COPY:parts/linux/cloud-init/artifacts/modprobe-CIS.conf /etc/modprobe.d/CIS.conf 644"
    "MARINER" "" "2.0" "kata" "COPY:parts/linux/cloud-init/artifacts/modprobe-CIS.conf /etc/modprobe.d/CIS.conf 644"
    "AZURELINUX" "" "3.0" "" "STRIP"
  End

  It 'retains the full modprobe file except on regular AzureLinux 3.0'
    When call bake "$1" "$2" "$3" "$4"
    The status should be success
    The output should equal "$5"
  End
End

Describe 'AzureLinux vulnerable kernel module VHD content validation'
  setup() {
    TEST_DIR="$(mktemp -d)"
    mkdir "$TEST_DIR/modprobe.d"
    : > "$TEST_DIR/modules"
    eval "$(sed -n '/^testVulnerableKernelModulesDisabled()/,/^}/p' vhdbuilder/packer/test/linux-vhd-content-test.sh | \
      sed "s|/etc/modprobe.d|$TEST_DIR/modprobe.d|g; s|/proc/modules|$TEST_DIR/modules|g")"
  }

  cleanup() { rm -rf "$TEST_DIR"; }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  err() { echo "FAIL:$*"; }
  modprobe() { return 1; }

  validate() {
    FEATURE_FLAGS="$3"
    if [ "$4" = "full" ]; then
      cp parts/linux/cloud-init/artifacts/modprobe-CIS.conf "$TEST_DIR/modprobe.d/CIS.conf"
    else
      grep -vE '^(install|blacklist) (algif_aead|esp4|esp6|rxrpc)( |$)' \
        parts/linux/cloud-init/artifacts/modprobe-CIS.conf > "$TEST_DIR/modprobe.d/CIS.conf"
    fi
    testVulnerableKernelModulesDisabled "$1" "$2"
  }

  Parameters
    "AzureLinux" "3.0" "kata" "full" 0 "testVulnerableKernelModulesDisabled:Finish"
    "AzureLinux" "3.0" "cvm,kata" "full" 0 "testVulnerableKernelModulesDisabled:Finish"
    "AzureLinux" "3.0" "" "stripped" 0 "testVulnerableKernelModulesDisabled:Finish"
    "AzureLinuxOSGuard" "3.0" "" "full" 0 "testVulnerableKernelModulesDisabled:Finish"
    "AzureLinux" "2.0" "kata" "full" 0 "testVulnerableKernelModulesDisabled:Finish"
    "AzureLinux" "3.0" "kata" "stripped" 1 "FAIL:"
    "AzureLinux" "3.0" "" "full" 1 "FAIL:"
  End

  It 'requires the mitigation state for the correct kernel stream'
    When call validate "$1" "$2" "$3" "$4"
    The status should equal "$5"
    The output should include "$6"
  End
End
