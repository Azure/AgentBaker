#!/bin/bash

Describe 'Azure Linux live patching VHD delivery'
    validate_inputs() {
        local template
        test -f parts/root/shared/opt/azure/containers/mariner-package-update.sh || return 1
        test -f parts/root/shared/opt/azure/containers/security-update.sh || return 1
        test -f parts/root/shared/opt/azure/containers/npd-update.sh || return 1
        test -f parts/root/shared/opt/azure/image-build/install-npd.sh || return 1
        for template in vhdbuilder/packer/vhd-image-builder-mariner{,-cvm,-arm64}.json; do
            jq -e '[.. | objects | .source? // empty] as $sources |
                any($sources[]; startswith("aks-node-controller/bin/")) and
                ($sources | index("parts/root/shared/opt/azure/image-build/install-npd.sh")) == null' "${template}" > /dev/null || return 1
        done
        grep -Fq 'source /opt/azure/containers/security-update.sh' parts/root/shared/opt/azure/containers/mariner-package-update.sh || return 1
        grep -Fq 'source /opt/azure/containers/npd-update.sh' parts/root/shared/opt/azure/containers/mariner-package-update.sh
    }

    It 'stages and installs the dispatcher, security handler and NPD handler'
        When call validate_inputs
        The status should be success
    End

    It 'preserves the systemd entrypoint'
        When run grep -Fx 'ExecStart=/opt/azure/containers/mariner-package-update.sh' parts/root/shared/etc/systemd/system/snapshot-update.azurelinux.service
        The status should be success
        The output should equal 'ExecStart=/opt/azure/containers/mariner-package-update.sh'
    End
End
