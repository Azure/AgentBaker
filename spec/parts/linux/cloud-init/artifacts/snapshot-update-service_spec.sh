#!/bin/bash

Describe 'snapshot update service commands'
    validate_ubuntu_packer_inputs() {
        local packer_file
        test -f parts/root/shared/opt/azure/containers/security-update.ubuntu.sh || return 1
        test -f parts/root/shared/opt/azure/containers/ubuntu-snapshot-update.sh || return 1
        test -f parts/root/shared/opt/azure/containers/npd-update.sh || return 1
        test -f parts/root/shared/opt/azure/image-build/install-npd.sh || return 1
        test -f parts/root/shared/etc/systemd/system/snapshot-update.service || return 1
        test -f parts/root/shared/etc/systemd/system/snapshot-update.timer || return 1

        for packer_file in \
            vhdbuilder/packer/vhd-image-builder-base.json \
            vhdbuilder/packer/vhd-image-builder-cvm.json \
            vhdbuilder/packer/vhd-image-builder-arm64-gen2.json \
            vhdbuilder/packer/vhd-image-builder-arm64-gb.json
        do
            jq -e '
                [.. | objects | .source? // empty] as $sources |
                any($sources[]; startswith("aks-node-controller/bin/")) and
                ($sources | index("parts/root/shared/opt/azure/image-build/install-npd.sh")) == null and
                ($sources | index("parts/root/shared/etc/systemd/system/snapshot-update.service")) == null and
                ($sources | index("parts/root/shared/etc/systemd/system/snapshot-update.timer")) == null and
                ($sources | index("parts/linux/cloud-init/artifacts/ubuntu/knead-component.sh")) == null
            ' "${packer_file}" > /dev/null || return 1
        done
    }

    validate_mariner_packer_inputs() {
        local packer_file
        test -f parts/root/shared/opt/azure/containers/mariner-package-update.sh || return 1
        test -f parts/root/shared/etc/systemd/system/snapshot-update.azurelinux.service || return 1
        test -f parts/root/shared/etc/systemd/system/snapshot-update.azurelinux.timer || return 1

        for packer_file in \
            vhdbuilder/packer/vhd-image-builder-mariner.json \
            vhdbuilder/packer/vhd-image-builder-mariner-cvm.json \
            vhdbuilder/packer/vhd-image-builder-mariner-arm64.json
        do
            jq -e '
                [.. | objects | .source? // empty] as $sources |
                any($sources[]; startswith("aks-node-controller/bin/")) and
                ($sources | index("parts/root/shared/etc/systemd/system/snapshot-update.azurelinux.service")) == null and
                ($sources | index("parts/root/shared/etc/systemd/system/snapshot-update.azurelinux.timer")) == null
            ' "${packer_file}" > /dev/null || return 1
        done
    }

    validate_ubuntu_vhd_only_delivery() {
        local template="parts/linux/cloud-init/nodecustomdata.yml"
        local generator="hotfix/hotfix_generate.py"
        local key

        for key in snapshotUpdateScript securityUpdateScript npdUpdateScript
        do
            ! grep -Fq "GetVariableProperty \"cloudInitData\" \"${key}\"" "${template}" || return 1
            ! grep -Fq ": \"${key}\"" "${generator}" || return 1
        done

        # Scripts and units are delivered by the VHD to avoid custom-data growth.
        ! grep -Fq 'GetVariableProperty "cloudInitData" "snapshotUpdateService"' "${template}" || return 1
        ! grep -Fq 'GetVariableProperty "cloudInitData" "snapshotUpdateTimer"' "${template}"
    }

    It 'keeps the Ubuntu snapshot service unchanged'
        When run grep -Fx 'ExecStart=/opt/azure/containers/ubuntu-snapshot-update.sh' parts/root/shared/etc/systemd/system/snapshot-update.service
        The status should be success
        The output should equal 'ExecStart=/opt/azure/containers/ubuntu-snapshot-update.sh'
    End

    It 'keeps the Azure Linux package updater'
        When run grep -Fx 'ExecStart=/opt/azure/containers/mariner-package-update.sh' parts/root/shared/etc/systemd/system/snapshot-update.azurelinux.service
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

    It 'keeps the updater and handler out of provisioning custom data'
        When call validate_ubuntu_vhd_only_delivery
        The status should be success
    End
End
