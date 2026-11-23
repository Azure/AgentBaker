#!/bin/bash

Describe 'vhdbuilder/packer/cleanup-vhd.sh'
  hook=/tmp/DeprovisioningScript.sh
  cleanup() { rm -f "$hook"; }
  BeforeEach 'cleanup'
  AfterEach 'cleanup'

  It 'installs an executable AIB deprovision hook that runs aks-node-controller generalize'
    When run bash vhdbuilder/packer/cleanup-vhd.sh
    The status should be success
    The contents of file "$hook" should equal "#!/bin/bash -eu
exec sudo /opt/azure/containers/aks-node-controller generalize"
    Assert test -x "$hook"
  End
End
