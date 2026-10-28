#!/bin/bash

Describe 'Azure Linux live patching VHD delivery'
    validate_inputs() {
        local template
        for template in vhdbuilder/packer/vhd-image-builder-mariner{,-cvm,-arm64}.json; do
            jq -e '[.. | objects | .source? // empty] as $sources |
                ($sources | index("parts/linux/cloud-init/artifacts/mariner/mariner-package-update.sh")) != null and
                ($sources | index("parts/linux/cloud-init/artifacts/mariner/security-update.sh")) != null and
                ($sources | index("parts/linux/cloud-init/artifacts/npd-update.sh")) != null and
                ($sources | index("vhdbuilder/packer/install-npd.sh")) != null' "${template}" > /dev/null || return 1
        done
        grep -Fq 'cpAndMode /home/packer/security-update.sh /opt/azure/containers/security-update.sh 544' vhdbuilder/packer/packer_source.sh || return 1
        grep -Fq 'cpAndMode $NPD_UPDATE_SH_SRC $NPD_UPDATE_SH_DEST 544' vhdbuilder/packer/packer_source.sh || return 1
        grep -Fq 'source /opt/azure/containers/security-update.sh' parts/linux/cloud-init/artifacts/mariner/mariner-package-update.sh || return 1
        grep -Fq 'source /opt/azure/containers/npd-update.sh' parts/linux/cloud-init/artifacts/mariner/mariner-package-update.sh
    }

    It 'stages and installs the dispatcher, security handler and NPD handler'
        When call validate_inputs
        The status should be success
    End

    It 'preserves the systemd entrypoint'
        When run grep -Fx 'ExecStart=/opt/azure/containers/mariner-package-update.sh' parts/linux/cloud-init/artifacts/mariner/package-update.service
        The status should be success
        The output should equal 'ExecStart=/opt/azure/containers/mariner-package-update.sh'
    End
End
