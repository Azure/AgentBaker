#!/bin/bash

Describe 'node-exporter-startup.sh hardware arguments'
    NODE_EXPORTER_STARTUP_SOURCE_ONLY=true
    Include './parts/linux/cloud-init/artifacts/node-exporter/node-exporter-startup.sh'

    setup_infiniband_class() {
        INFINIBAND_CLASS_PATH="$(mktemp -d)"
    }

    BeforeEach 'setup_infiniband_class'
    AfterEach 'rm -rf "$INFINIBAND_CLASS_PATH"'

    It 'adds no argument without RDMA devices'
        When call getNodeExporterHardwareArgs
        The status should be success
        The output should equal ''
    End

    It 'keeps the InfiniBand collector enabled for non-MANA devices'
        mkdir "${INFINIBAND_CLASS_PATH}/mlx5_0"

        When call getNodeExporterHardwareArgs
        The status should be success
        The output should equal ''
    End

    It 'disables the InfiniBand collector for MANA devices'
        mkdir "${INFINIBAND_CLASS_PATH}/mana_0"

        When call getNodeExporterHardwareArgs
        The status should be success
        The output should equal '--no-collector.infiniband'
    End

    It 'disables the InfiniBand collector on mixed MANA and non-MANA devices'
        mkdir "${INFINIBAND_CLASS_PATH}/mana_0" "${INFINIBAND_CLASS_PATH}/mlx5_0"

        When call getNodeExporterHardwareArgs
        The status should be success
        The output should equal '--no-collector.infiniband'
    End
End
