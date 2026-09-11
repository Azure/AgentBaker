#!/bin/bash

Describe 'cgroup telemetry'
    setup_cgroup_telemetry_test() {
        TEST_ROOT=$(mktemp -d)
        CGROUP_ROOT="${TEST_ROOT}/cgroup"
        EVENTS_ROOT="${TEST_ROOT}/events"
        mkdir -p "${CGROUP_ROOT}" "${EVENTS_ROOT}"
        printf 'MemTotal: 1000 kB\n' > "${TEST_ROOT}/meminfo"
    }

    cleanup_cgroup_telemetry_test() {
        rm -rf "${TEST_ROOT}"
    }

    create_memory_stat() {
        local cgroup_path="$1"
        local anon="$2"
        local file="$3"
        mkdir -p "${CGROUP_ROOT}/${cgroup_path}"
        printf 'anon %s\nfile %s\n' "${anon}" "${file}" > "${CGROUP_ROOT}/${cgroup_path}/memory.stat"
    }

    create_pressure_files() {
        local cgroup_path="$1"
        mkdir -p "${CGROUP_ROOT}/${cgroup_path}"
        printf 'some avg10=1.00 avg60=2.00 avg300=3.00 total=4\n' > "${CGROUP_ROOT}/${cgroup_path}/cpu.pressure"
        printf 'some avg10=5.00 avg60=6.00 avg300=7.00 total=8\nfull avg10=9.00 avg60=10.00 avg300=11.00 total=12\n' > "${CGROUP_ROOT}/${cgroup_path}/memory.pressure"
        printf 'some avg10=13.00 avg60=14.00 avg300=15.00 total=16\nfull avg10=17.00 avg60=18.00 avg300=19.00 total=20\n' > "${CGROUP_ROOT}/${cgroup_path}/io.pressure"
    }

    prepare_memory_script() {
        sed \
            -e "s|EVENTS_LOGGING_DIR=/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/|EVENTS_LOGGING_DIR=${EVENTS_ROOT}/|" \
            -e 's|CGROUP_VERSION=$(stat -fc %T /sys/fs/cgroup)|CGROUP_VERSION=cgroup2fs|' \
            -e 's@CSLICE=$(systemctl show containerd -p Slice | cut -d= -f2)@CSLICE=system.slice@' \
            -e 's@KSLICE=$(systemctl show kubelet -p Slice | cut -d= -f2)@KSLICE=system.slice@' \
            -e "s|CGROUP=\"/sys/fs/cgroup\"|CGROUP=\"${CGROUP_ROOT}\"|" \
            -e "s|/proc/meminfo|${TEST_ROOT}/meminfo|" \
            ./parts/linux/cloud-init/artifacts/cgroup-memory-telemetry.sh > "${TEST_ROOT}/cgroup-memory-telemetry.sh"
    }

    prepare_pressure_script() {
        sed \
            -e "s|EVENTS_LOGGING_DIR=/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/|EVENTS_LOGGING_DIR=${EVENTS_ROOT}/|" \
            -e 's|CGROUP_VERSION=$(stat -fc %T /sys/fs/cgroup)|CGROUP_VERSION=cgroup2fs|' \
            -e "s|CGROUP=\"/sys/fs/cgroup\"|CGROUP=\"${CGROUP_ROOT}\"|" \
            -e 's@CSLICE=$(systemctl show containerd -p Slice | cut -d= -f2)@CSLICE=system.slice@' \
            -e 's@KSLICE=$(systemctl show kubelet -p Slice | cut -d= -f2)@KSLICE=system.slice@' \
            ./parts/linux/cloud-init/artifacts/cgroup-pressure-telemetry.sh > "${TEST_ROOT}/cgroup-pressure-telemetry.sh"
    }

    BeforeEach 'setup_cgroup_telemetry_test'
    AfterEach 'cleanup_cgroup_telemetry_test'

    It 'emits memory for the additional services and tolerates an unavailable service'
        for cgroup_path in \
            memory.stat \
            system.slice/memory.stat \
            azure.slice/memory.stat \
            kubepods.slice/memory.stat \
            user.slice/memory.stat \
            system.slice/containerd.service/memory.stat \
            system.slice/kubelet.service/memory.stat; do
            mkdir -p "${CGROUP_ROOT}/$(dirname "${cgroup_path}")"
            printf 'anon 1\nfile 2\n' > "${CGROUP_ROOT}/${cgroup_path}"
        done
        printf 'max\n' > "${CGROUP_ROOT}/kubepods.slice/memory.max"
        create_memory_stat system.slice/node-problem-detector.service 10 20
        create_memory_stat system.slice/node-exporter.service 30 40
        create_memory_stat localdns.slice/localdns.service 50 60
        prepare_memory_script

        When run bash "${TEST_ROOT}/cgroup-memory-telemetry.sh"
        The status should be success
        The contents of file "${EVENTS_ROOT}"/* should include '\"containerd_service_memory\":\"3\"'
        The contents of file "${EVENTS_ROOT}"/* should include '\"kubelet_service_memory\":\"3\"'
        The contents of file "${EVENTS_ROOT}"/* should include 'node_problem_detector_service_memory'
        The contents of file "${EVENTS_ROOT}"/* should include '\"node_problem_detector_service_memory\":\"30\"'
        The contents of file "${EVENTS_ROOT}"/* should include '\"node_exporter_service_memory\":\"70\"'
        The contents of file "${EVENTS_ROOT}"/* should include '\"sync_container_logs_service_memory\":\"Not Found\"'
        The contents of file "${EVENTS_ROOT}"/* should include '\"localdns_service_memory\":\"110\"'
    End

    It 'emits pressure for available services when another service is unavailable'
        for cgroup_path in \
            . \
            system.slice \
            azure.slice \
            kubepods.slice \
            system.slice/kubelet.service \
            system.slice/containerd.service \
            system.slice/node-exporter.service \
            system.slice/sync-container-logs.service \
            localdns.slice/localdns.service; do
            create_pressure_files "${cgroup_path}"
        done
        prepare_pressure_script

        When run bash "${TEST_ROOT}/cgroup-pressure-telemetry.sh"
        The status should be success
        The contents of file "${EVENTS_ROOT}"/* should include '\"kubelet_service_pressure\":{\"CPUPressure\":{'
        The contents of file "${EVENTS_ROOT}"/* should include '\"containerd_service_pressure\":{\"CPUPressure\":{'
        The contents of file "${EVENTS_ROOT}"/* should include 'node_problem_detector_service_pressure'
        The contents of file "${EVENTS_ROOT}"/* should include '\"node_problem_detector_service_pressure\":\"Not Found\"'
        The contents of file "${EVENTS_ROOT}"/* should include '\"node_exporter_service_pressure\":{'
        The contents of file "${EVENTS_ROOT}"/* should include '\"sync_container_logs_service_pressure\":{'
        The contents of file "${EVENTS_ROOT}"/* should include '\"localdns_service_pressure\":{'
    End
End
