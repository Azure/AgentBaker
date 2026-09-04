#!/bin/bash

Describe 'node-exporter-startup.sh hardware arguments'
    NODE_EXPORTER_STARTUP_SOURCE_ONLY=true
    Include './parts/linux/cloud-init/artifacts/node-exporter/node-exporter-startup.sh'

    setup_pci_devices() {
        PCI_DEVICES_PATH="$(mktemp -d)"
    }

    add_pci_device() {
        local name="$1"
        local vendor="$2"
        local device="$3"
        mkdir "${PCI_DEVICES_PATH}/${name}"
        printf '%s\n' "$vendor" > "${PCI_DEVICES_PATH}/${name}/vendor"
        printf '%s\n' "$device" > "${PCI_DEVICES_PATH}/${name}/device"
    }

    BeforeEach 'setup_pci_devices'
    AfterEach 'rm -rf "$PCI_DEVICES_PATH"'

    It 'adds no argument without PCI devices'
        When call getNodeExporterHardwareArgs
        The status should be success
        The output should equal ''
    End

    It 'keeps the InfiniBand collector enabled for non-MANA PCI devices'
        add_pci_device '0000:00:02.0' '0x15b3' '0x1017'

        When call getNodeExporterHardwareArgs
        The status should be success
        The output should equal ''
    End

    It 'disables the InfiniBand collector for MANA devices'
        add_pci_device '7870:00:00.0' '0x1414' '0x00ba'

        When call getNodeExporterHardwareArgs
        The status should be success
        The output should equal '--no-collector.infiniband'
    End

    It 'disables the InfiniBand collector on mixed MANA and non-MANA devices'
        add_pci_device '0000:00:02.0' '0x15b3' '0x1017'
        add_pci_device '7870:00:00.0' '0x1414' '0x00ba'

        When call getNodeExporterHardwareArgs
        The status should be success
        The output should equal '--no-collector.infiniband'
    End
End
