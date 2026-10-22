#!/bin/bash
# shellcheck disable=SC2329

Describe 'Ubuntu PIS GPU cleanup helper VHD installation'
  Include './vhdbuilder/packer/packer_source.sh'

  isUbuntu() {
    [ "$1" = UBUNTU ]
  }
  cpAndMode() {
    printf '%s\n' "$*"
  }

  It 'copies the helper into Ubuntu images'
    OS=UBUNTU
    When call copyUbuntuGPUCleanupHelper
    The status should be success
    The output should equal '/home/packer/provision_installs_ubuntu_gpu.sh /opt/azure/containers/provision_installs_ubuntu_gpu.sh 0744'
  End

  Parameters
    MARINER
    AZURELINUX
    FLATCAR
  End
  It "does not require the Ubuntu helper on $1"
    OS="$1"
    When call copyUbuntuGPUCleanupHelper
    The status should be success
    The output should be blank
  End

  It 'installs the helper when copying Packer files'
    When call sed -n '/^copyPackerFiles()/,/^}/p' vhdbuilder/packer/packer_source.sh
    The output should include '  copyUbuntuGPUCleanupHelper'
  End
End
