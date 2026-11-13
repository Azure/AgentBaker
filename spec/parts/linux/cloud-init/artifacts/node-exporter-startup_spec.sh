#!/bin/bash

Describe 'node-exporter-startup.sh hardware arguments'
    NODE_EXPORTER_STARTUP_SOURCE_ONLY=true
    Include './parts/linux/cloud-init/artifacts/node-exporter/node-exporter-startup.sh'

    setup_pci_devices() {
        PCI_DEVICES_PATH="$(mktemp -d)"
        MANA_OBSERVED_FILE="${PCI_DEVICES_PATH}/mana-observed"
        NODE_EXPORTER_EXTRA_ARGS=''
        KUBELET_DEFAULT_FILE="${PCI_DEVICES_PATH}/kubelet-default"
        printf '%s\n' 'KUBELET_FLAGS=--address=0.0.0.0 --node-ip=192.0.2.1 --pod-max-pids=-1' > "$KUBELET_DEFAULT_FILE"
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

Describe 'node-exporter-startup.sh listen address'
    NODE_EXPORTER_STARTUP_SOURCE_ONLY=true
    Include './parts/linux/cloud-init/artifacts/node-exporter/node-exporter-startup.sh'

    setup() {
        TEST_DIR="$(mktemp -d)"
        KUBELET_DEFAULT_FILE="${TEST_DIR}/kubelet-default"
        PCI_DEVICES_PATH="${TEST_DIR}/pci"
        MANA_OBSERVED_FILE="${TEST_DIR}/mana-observed"
        NODE_EXPORTER_EXTRA_ARGS=''
        mkdir "$PCI_DEVICES_PATH"
    }
    BeforeEach 'setup'
    AfterEach 'rm -rf "$TEST_DIR"'

    kubelet_flags() { printf 'KUBELET_FLAGS=--address=0.0.0.0 %s --pod-max-pids=-1\n' "$1" > "$KUBELET_DEFAULT_FILE"; }
    # Route lookup used when kubelet has no --node-ip.
    ip() {
        [ "$*" = '-4 -o route get 168.63.129.16' ] || return 1
        printf '%s\n' '168.63.129.16 via 10.224.0.1 dev eth0 src 10.224.0.9 uid 0 \    cache '
    }

    It 'uses the kubelet --node-ip address'
        kubelet_flags '--node-ip=10.224.0.4'
        When call getNodeExporterListenIP
        The status should be success
        The output should equal '10.224.0.4'
    End

    It 'uses the IPv4 address of a dual-stack --node-ip in either order'
        kubelet_flags '--node-ip=fd00:10:224::4,10.224.0.4'
        When call getNodeExporterListenIP
        The status should be success
        The output should equal '10.224.0.4'
    End

    It 'falls back to the primary NIC source address without --node-ip'
        kubelet_flags ''
        When call getNodeExporterListenIP
        The status should be success
        The output should equal '10.224.0.9'
    End

    It 'falls back to the route when --node-ip is IPv6 only'
        kubelet_flags '--node-ip=fd00:10:224::4'
        When call getNodeExporterListenIP
        The status should be success
        The output should equal '10.224.0.9'
    End

    It 'falls back to the route when the kubelet defaults file is missing'
        When call getNodeExporterListenIP
        The status should be success
        The output should equal '10.224.0.9'
    End

    It 'ignores --node-ip text inside node labels'
        printf '%s\n' 'KUBELET_FLAGS=--address=0.0.0.0 --node-ip=10.224.0.4' \
            'KUBELET_NODE_LABELS=example.com/custom--node-ip=10.224.0.99,agentpool=x' > "$KUBELET_DEFAULT_FILE"
        When call getNodeExporterListenIP
        The status should be success
        The output should equal '10.224.0.4'
    End

    It 'falls back to the route when only a label mentions --node-ip'
        printf '%s\n' 'KUBELET_FLAGS=--address=0.0.0.0' \
            'KUBELET_NODE_LABELS=example.com/custom--node-ip=10.224.0.99' > "$KUBELET_DEFAULT_FILE"
        When call getNodeExporterListenIP
        The status should be success
        The output should equal '10.224.0.9'
    End

    It 'reads a quoted KUBELET_FLAGS value'
        printf '%s\n' 'KUBELET_FLAGS="--address=0.0.0.0 --node-ip=10.224.0.4"' > "$KUBELET_DEFAULT_FILE"
        When call getNodeExporterListenIP
        The status should be success
        The output should equal '10.224.0.4'
    End

    It 'never uses the unspecified or loopback address from --node-ip'
        kubelet_flags '--node-ip=0.0.0.0'
        When call getNodeExporterListenIP
        The status should be success
        The output should equal '10.224.0.9'
    End

    It 'rejects an unspecified route source address'
        kubelet_flags '--node-ip=127.0.0.1'
        ip() { printf '%s\n' '168.63.129.16 dev eth0 src 0.0.0.0 uid 0'; }
        When call getNodeExporterListenIP
        The status should be failure
        The output should equal ''
    End

    It 'fails instead of returning an empty or wildcard address'
        ip() { return 2; }
        When call getNodeExporterListenIP
        The status should be failure
        The output should equal ''
    End

    run_startup() {
        NODE_EXPORTER_STARTUP_SOURCE_ONLY=false
        NODE_EXPORTER_TLS_ENABLED=false
        exec() { printf '%s\n' "$@"; }
        . ./parts/linux/cloud-init/artifacts/node-exporter/node-exporter-startup.sh
    }

    It 'listens only on the node IP'
        kubelet_flags '--node-ip=10.224.0.4,fd00:10:224::4'
        When run run_startup
        The status should be success
        The output should include '--web.listen-address=10.224.0.4:19100'
    End

    It 'exits without starting the exporter when no node IP is available'
        ip() { return 2; }
        When run run_startup
        The status should equal 1
        The output should equal ''
        The stderr should include 'refusing to listen on all interfaces'
    End
End
