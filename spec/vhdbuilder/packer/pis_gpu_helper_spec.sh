#!/bin/bash

Describe 'Ubuntu PIS GPU cleanup helper VHD installation'
  Include './vhdbuilder/packer/packer_source.sh'

  isUbuntu() { [ "$1" = UBUNTU ]; }
  cpAndMode() { printf '%s\n' "$*"; }

  It 'copies the helper into Ubuntu images'
    OS=UBUNTU
    When call copyUbuntuGPUCleanupHelper
    The output should equal '/home/packer/provision_installs_ubuntu_gpu.sh /opt/azure/containers/provision_installs_ubuntu_gpu.sh 0744'
  End

  It 'calls the helper copy during Packer file installation'
    When call sed -n '/^copyPackerFiles()/,/^}/p' vhdbuilder/packer/packer_source.sh
    The output should include '  copyUbuntuGPUCleanupHelper'
  End
End
