#!/bin/bash

Describe 'node-exporter-startup.sh hardware arguments'
    NODE_EXPORTER_STARTUP_SOURCE_ONLY=true
    Include './parts/linux/cloud-init/artifacts/node-exporter/node-exporter-startup.sh'

    setup_pci_devices() {
        PCI_DEVICES_PATH="$(mktemp -d)"
        MANA_OBSERVED_FILE="${PCI_DEVICES_PATH}/mana-observed"
        NODE_EXPORTER_EXTRA_ARGS=''
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

    It 'recognizes the MANA PF device ID'
        add_pci_device '7870:00:00.0' '0x1414' '0x00b9'

        When call getNodeExporterHardwareArgs
        The status should be success
        The output should equal '--no-collector.infiniband'
    End

    It 'recognizes the MANA PF2 device ID'
        add_pci_device '7870:00:00.1' '0x1414' '0x00c1'

        When call getNodeExporterHardwareArgs
        The status should be success
        The output should equal '--no-collector.infiniband'
    End

    It 'does not match another vendor with the same device ID'
        add_pci_device '7870:00:00.0' '0x15b3' '0x00ba'

        When call getNodeExporterHardwareArgs
        The status should be success
        The output should equal ''
    End

    It 'disables the InfiniBand collector on mixed MANA and non-MANA devices'
        add_pci_device '0000:00:02.0' '0x15b3' '0x1017'
        add_pci_device '7870:00:00.0' '0x1414' '0x00ba'

        When call getNodeExporterHardwareArgs
        The status should be success
        The output should equal '--no-collector.infiniband'
    End

    run_startup() {
        NODE_EXPORTER_STARTUP_SOURCE_ONLY=false
        NODE_EXPORTER_TLS_ENABLED=false
        hostname() { printf '%s\n' '192.0.2.1'; }
        exec() { printf '%s\n' "$@"; }
        . ./parts/linux/cloud-init/artifacts/node-exporter/node-exporter-startup.sh
    }

    It 'passes the hardware flag to the final exporter invocation'
        add_pci_device '7870:00:00.0' '0x1414' '0x00ba'

        When run run_startup
        The status should be success
        The line 1 of output should equal '/opt/bin/node-exporter'
        The output should include '--web.listen-address=192.0.2.1:19100'
        The output should include '--no-collector.infiniband'
        The path "$MANA_OBSERVED_FILE" should be file
    End

    It 'leaves InfiniBand enabled in the final invocation without MANA'
        When run run_startup
        The status should be success
        The output should include '/opt/bin/node-exporter'
        The output should not include '--no-collector.infiniband'
    End

    It 'replaces conflicting InfiniBand toggles with exactly one MANA override'
        add_pci_device '7870:00:00.0' '0x1414' '0x00ba'
        NODE_EXPORTER_EXTRA_ARGS='--collector.infiniband --collector.infiniband=true --no-collector.infiniband --no-collector.infiniband=false --collector.systemd'

        When run run_startup
        The status should be success
        The output should not include '--collector.infiniband'
        The output should not include '--no-collector.infiniband='
        The output should include '--collector.systemd'
        The output should end with '--no-collector.infiniband'
        The lines of output should equal 12
    End

    It 'preserves explicit InfiniBand arguments on non-MANA nodes'
        NODE_EXPORTER_EXTRA_ARGS='--collector.infiniband'

        When run run_startup
        The status should be success
        The output should end with '--collector.infiniband'
        The output should not include '--no-collector.infiniband'
    End

    restart_during_vf_absence() {
        getNodeExporterHardwareArgs >/dev/null
        rm -rf "${PCI_DEVICES_PATH}/7870:00:00.0"
        run_startup
    }

    It 'keeps suppression after an observed VF disappears and exporter restarts'
        add_pci_device '7870:00:00.0' '0x1414' '0x00ba'

        When run restart_during_vf_absence
        The status should be success
        The output should include '--no-collector.infiniband'
    End

    systemctl() {
        # The event must be recorded before systemd can run the next ExecStart.
        [ -f "$MANA_OBSERVED_FILE" ] || return 1
        if [ "$1" = 'show' ]; then
            printf '%s\n' '0'
        else
            printf '%s\n' "$*"
        fi
    }

    run_attach_handler() {
        NODE_EXPORTER_STARTUP_SOURCE_ONLY=false
        . ./parts/linux/cloud-init/artifacts/node-exporter/node-exporter-startup.sh --mana-added
    }

    It 'records late attachment and queues only a nonblocking conditional restart'
        When run run_attach_handler
        The status should be success
        The output should equal '--no-block try-restart node-exporter.service'
        The path "$MANA_OBSERVED_FILE" should be file
    End

    attach_after_startup() {
        run_startup
        nodeExporterMANAAdded
        # The event marker is sufficient even if the VF vanishes before restart.
        run_startup
    }

    It 'disables collection on restart after attachment was missed at startup'
        When run attach_after_startup
        The status should be success
        The output should include '--no-block try-restart node-exporter.service'
        The output should include '--no-collector.infiniband'
    End

    It 'does not request a restart if recording the attachment fails'
        MANA_OBSERVED_FILE="${PCI_DEVICES_PATH}/missing/marker"

        When run run_attach_handler
        The status should be failure
        The stderr should include 'No such file or directory'
        The output should equal ''
    End

    It 'does not restart again when the running exporter already disabled InfiniBand'
        systemctl() {
            if [ "$1" = 'show' ]; then printf '%s\n' '123'; else return 1; fi
        }
        grep() {
            [ "$*" = '-zFxq -- --no-collector.infiniband /proc/123/cmdline' ]
        }

        When run run_attach_handler
        The status should be success
        The output should equal ''
    End

    It 'requests a restart even with an existing marker if startup has not applied it'
        touch "$MANA_OBSERVED_FILE"

        When run run_attach_handler
        The status should be success
        The output should equal '--no-block try-restart node-exporter.service'
    End

    It 'does not inspect procfs for an invalid MainPID'
        systemctl() {
            if [ "$1" = 'show' ]; then printf '%s\n' '../123'; else printf '%s\n' "$*"; fi
        }
        grep() { printf '%s\n' 'unexpected procfs read' >&2; return 1; }

        When run run_attach_handler
        The status should be success
        The output should equal '--no-block try-restart node-exporter.service'
        The stderr should equal ''
    End
End
