#!/bin/bash

Describe 'snapshot update service commands'
    validate_ubuntu_packer_inputs() {
        local packer_file

        for packer_file in \
            vhdbuilder/packer/vhd-image-builder-base.json \
            vhdbuilder/packer/vhd-image-builder-cvm.json \
            vhdbuilder/packer/vhd-image-builder-arm64-gen2.json \
            vhdbuilder/packer/vhd-image-builder-arm64-gb.json
        do
            jq -e '
                [.. | objects | .source? // empty] as $sources |
                ($sources | index("parts/linux/cloud-init/artifacts/ubuntu/security-update.sh")) != null and
                ($sources | index("parts/linux/cloud-init/artifacts/ubuntu/ubuntu-snapshot-update.sh")) != null and
                ($sources | index("parts/linux/cloud-init/artifacts/ubuntu/snapshot-update.service")) != null and
                ($sources | index("parts/linux/cloud-init/artifacts/ubuntu/snapshot-update.timer")) != null and
                ($sources | index("parts/linux/cloud-init/artifacts/ubuntu/knead-component.sh")) == null
            ' "${packer_file}" > /dev/null || return 1
        done
    }

    validate_mariner_packer_inputs() {
        local packer_file

        for packer_file in \
            vhdbuilder/packer/vhd-image-builder-mariner.json \
            vhdbuilder/packer/vhd-image-builder-mariner-cvm.json \
            vhdbuilder/packer/vhd-image-builder-mariner-arm64.json
        do
            jq -e '
                [.. | objects | .source? // empty] as $sources |
                ($sources | index("parts/linux/cloud-init/artifacts/mariner/mariner-package-update.sh")) != null and
                ($sources | index("parts/linux/cloud-init/artifacts/mariner/package-update.service")) != null and
                ($sources | index("parts/linux/cloud-init/artifacts/mariner/package-update.timer")) != null
            ' "${packer_file}" > /dev/null || return 1
        done
    }

    validate_ubuntu_hotfix_transition() {
        local template="parts/linux/cloud-init/nodecustomdata.yml"
        local generator="hotfix/hotfix_generate.py"
        local key

        for key in snapshotUpdateScript securityUpdateScript
        do
            grep -Fq "GetVariableProperty \"cloudInitData\" \"${key}\"" "${template}" || return 1
            grep -Fq ": \"${key}\"" "${generator}" || return 1
        done

        # Existing VHD-baked units remain unchanged; hotfixes update scripts only.
        ! grep -Fq 'GetVariableProperty "cloudInitData" "snapshotUpdateService"' "${template}" || return 1
        ! grep -Fq 'GetVariableProperty "cloudInitData" "snapshotUpdateTimer"' "${template}"
    }

    It 'keeps the Ubuntu snapshot service unchanged'
        When run grep -Fx 'ExecStart=/opt/azure/containers/ubuntu-snapshot-update.sh' parts/linux/cloud-init/artifacts/ubuntu/snapshot-update.service
        The status should be success
        The output should equal 'ExecStart=/opt/azure/containers/ubuntu-snapshot-update.sh'
    End

    It 'keeps the Azure Linux package updater'
        When run grep -Fx 'ExecStart=/opt/azure/containers/mariner-package-update.sh' parts/linux/cloud-init/artifacts/mariner/package-update.service
        The status should be success
        The output should equal 'ExecStart=/opt/azure/containers/mariner-package-update.sh'
    End

    It 'stages the generic updater, handler, and existing units for Ubuntu images'
        When call validate_ubuntu_packer_inputs
        The status should be success
    End

    It 'preserves Azure Linux package updater inputs'
        When call validate_mariner_packer_inputs
        The status should be success
    End

    It 'hotfix-delivers the generic updater and security handler without switching units'
        When call validate_ubuntu_hotfix_transition
        The status should be success
    End
End