#!/bin/bash

Describe 'cgroup telemetry'
    setup_cgroup_telemetry_test() {
        TEST_ROOT=$(mktemp -d)
        CGROUP_ROOT="${TEST_ROOT}/cgroup"
        CPUACCT_ROOT="${TEST_ROOT}/cpuacct"
        EVENTS_ROOT="${TEST_ROOT}/events"
        mkdir -p "${CGROUP_ROOT}" "${CPUACCT_ROOT}" "${EVENTS_ROOT}"
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

    create_memory_stat_v1() {
        local cgroup_path="$1"
        local total_rss="$2"
        local total_cache="$3"
        mkdir -p "${CGROUP_ROOT}/${cgroup_path}"
        printf 'total_rss %s\ntotal_cache %s\n' "${total_rss}" "${total_cache}" > "${CGROUP_ROOT}/${cgroup_path}/memory.stat"
    }

    create_pressure_files() {
        local cgroup_path="$1"
        mkdir -p "${CGROUP_ROOT}/${cgroup_path}"
        printf 'some avg10=1.00 avg60=2.00 avg300=3.00 total=4\nfull avg10=21.00 avg60=22.00 avg300=23.00 total=24\n' > "${CGROUP_ROOT}/${cgroup_path}/cpu.pressure"
        printf 'some avg10=5.00 avg60=6.00 avg300=7.00 total=8\nfull avg10=9.00 avg60=10.00 avg300=11.00 total=12\n' > "${CGROUP_ROOT}/${cgroup_path}/memory.pressure"
        printf 'some avg10=13.00 avg60=14.00 avg300=15.00 total=16\nfull avg10=17.00 avg60=18.00 avg300=19.00 total=20\n' > "${CGROUP_ROOT}/${cgroup_path}/io.pressure"
    }

    create_cpu_stat_v2() {
        local cgroup_path="$1"
        mkdir -p "${CGROUP_ROOT}/${cgroup_path}"
        printf 'usage_usec 100\nuser_usec 60\nsystem_usec 40\nnr_periods 20\nnr_throttled 3\nthrottled_usec 7\n' > "${CGROUP_ROOT}/${cgroup_path}/cpu.stat"
    }

    create_cpu_stat_v1() {
        local cgroup_path="$1"
        mkdir -p "${CGROUP_ROOT}/${cgroup_path}" "${CPUACCT_ROOT}/${cgroup_path}"
        printf '100000\n' > "${CPUACCT_ROOT}/${cgroup_path}/cpuacct.usage"
        printf 'user 6\nsystem 4\n' > "${CPUACCT_ROOT}/${cgroup_path}/cpuacct.stat"
        printf 'nr_periods 20\nnr_throttled 3\nthrottled_time 7000\n' > "${CGROUP_ROOT}/${cgroup_path}/cpu.stat"
    }

    prepare_memory_script() {
        local cgroup_version="${1:-cgroup2fs}"
        sed \
            -e "s|EVENTS_LOGGING_DIR=/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/|EVENTS_LOGGING_DIR=${EVENTS_ROOT}/|" \
            -e "s|CGROUP_VERSION=\$(stat -fc %T /sys/fs/cgroup)|CGROUP_VERSION=${cgroup_version}|" \
            -e 's@CSLICE=$(systemctl show containerd -p Slice | cut -d= -f2)@CSLICE=system.slice@' \
            -e 's@KSLICE=$(systemctl show kubelet -p Slice | cut -d= -f2)@KSLICE=system.slice@' \
            -e "s|CGROUP=\"/sys/fs/cgroup\"|CGROUP=\"${CGROUP_ROOT}\"|" \
            -e "s|CGROUP=\"/sys/fs/cgroup/memory\"|CGROUP=\"${CGROUP_ROOT}\"|" \
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

    prepare_cpu_script() {
        local cgroup_version="${1:-cgroup2fs}"
        sed \
            -e "s|EVENTS_LOGGING_DIR=/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/|EVENTS_LOGGING_DIR=${EVENTS_ROOT}/|" \
            -e "s|CGROUP_VERSION=\$(stat -fc %T /sys/fs/cgroup)|CGROUP_VERSION=${cgroup_version}|" \
            -e 's@CSLICE=$(systemctl show containerd -p Slice | cut -d= -f2)@CSLICE=system.slice@' \
            -e 's@KSLICE=$(systemctl show kubelet -p Slice | cut -d= -f2)@KSLICE=system.slice@' \
            -e "s|CPU_CGROUP=\"/sys/fs/cgroup\"|CPU_CGROUP=\"${CGROUP_ROOT}\"|" \
            -e "s|CPU_CGROUP=\"/sys/fs/cgroup/cpu,cpuacct\"|CPU_CGROUP=\"${CGROUP_ROOT}\"|" \
            -e "s|CPU_CGROUP=\"/sys/fs/cgroup/cpuacct,cpu\"|CPU_CGROUP=\"${CGROUP_ROOT}\"|" \
            -e "s|CPU_CGROUP=\"/sys/fs/cgroup/cpu\"|CPU_CGROUP=\"${CGROUP_ROOT}\"|" \
            -e "s|CPUACCT_CGROUP=\"/sys/fs/cgroup/cpuacct\"|CPUACCT_CGROUP=\"${CPUACCT_ROOT}\"|" \
            -e "s|CPUACCT_CGROUP=\"\${CPU_CGROUP}\"|CPUACCT_CGROUP=\"${CPUACCT_ROOT}\"|" \
            -e 's/CPU_TICKS_PER_SECOND=$(getconf CLK_TCK)/CPU_TICKS_PER_SECOND=100/' \
            ./parts/linux/cloud-init/artifacts/cgroup-cpu-telemetry.sh > "${TEST_ROOT}/cgroup-cpu-telemetry.sh"
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

    It 'emits cgroup v1 memory for an available service and tolerates an unavailable service'
        for cgroup_path in \
            . \
            system.slice \
            azure.slice \
            kubepods \
            user.slice \
            system.slice/containerd.service \
            system.slice/kubelet.service; do
            create_memory_stat_v1 "${cgroup_path}" 1 2
        done
        printf 'max\n' > "${CGROUP_ROOT}/kubepods/memory.limit_in_bytes"
        create_memory_stat_v1 system.slice/node-exporter.service 30 40
        prepare_memory_script tmpfs

        When run bash "${TEST_ROOT}/cgroup-memory-telemetry.sh"
        The status should be success
        The contents of file "${EVENTS_ROOT}"/* should include '\"CgroupVersion\":\"cgroupv1\"'
        The contents of file "${EVENTS_ROOT}"/* should include '\"containerd_service_memory\":\"3\"'
        The contents of file "${EVENTS_ROOT}"/* should include '\"node_exporter_service_memory\":\"70\"'
        The contents of file "${EVENTS_ROOT}"/* should include '\"sync_container_logs_service_memory\":\"Not Found\"'
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
        The contents of file "${EVENTS_ROOT}"/* should include '\"CPUPressure\":{\"some_avg10\":\"1.00\",\"some_avg60\":\"2.00\",\"some_avg300\":\"3.00\",\"some_total\":\"4\"}'
        The contents of file "${EVENTS_ROOT}"/* should include '\"sync_container_logs_service_pressure\":{'
        The contents of file "${EVENTS_ROOT}"/* should include '\"localdns_service_pressure\":{'
    End

    It 'emits cgroup v2 CPU usage and throttling counters with explicit units'
        create_cpu_stat_v2 system.slice/containerd.service
        create_cpu_stat_v2 system.slice/kubelet.service
        prepare_cpu_script

        When run bash "${TEST_ROOT}/cgroup-cpu-telemetry.sh"
        The status should be success
        The contents of file "${EVENTS_ROOT}"/* should include '\"CgroupVersion\":\"cgroupv2\"'
        The contents of file "${EVENTS_ROOT}"/* should include '\"counter_units\":{\"usage_usec\":\"microseconds\"'
        The contents of file "${EVENTS_ROOT}"/* should include '\"containerd_service_cpu_usage\":{\"usage_usec\":\"100\",\"user_usec\":\"60\",\"system_usec\":\"40\",\"nr_periods\":\"20\",\"nr_throttled\":\"3\",\"throttled_usec\":\"7\"}'
        The contents of file "${EVENTS_ROOT}"/* should include '\"node_exporter_service_cpu_usage\":\"Not Found\"'
        The contents of file "${EVENTS_ROOT}"/* should include 'downstream rates must discard negative deltas'
    End

    It 'emits normalized cgroup v1 CPU counters'
        create_cpu_stat_v1 system.slice/containerd.service
        create_cpu_stat_v1 system.slice/kubelet.service
        prepare_cpu_script tmpfs

        When run bash "${TEST_ROOT}/cgroup-cpu-telemetry.sh"
        The status should be success
        The contents of file "${EVENTS_ROOT}"/* should include '\"CgroupVersion\":\"cgroupv1\"'
        The contents of file "${EVENTS_ROOT}"/* should include '\"containerd_service_cpu_usage\":{\"usage_usec\":\"100\",\"user_usec\":\"60000\",\"system_usec\":\"40000\",\"nr_periods\":\"20\",\"nr_throttled\":\"3\",\"throttled_usec\":\"7\"}'
    End

    It 'marks missing and malformed CPU counters without failing the observation'
        mkdir -p "${CGROUP_ROOT}/system.slice/containerd.service"
        printf 'usage_usec invalid\nuser_usec 60\nnr_periods nope\nnr_throttled 3\n' > "${CGROUP_ROOT}/system.slice/containerd.service/cpu.stat"
        create_cpu_stat_v2 system.slice/kubelet.service
        prepare_cpu_script

        When run bash "${TEST_ROOT}/cgroup-cpu-telemetry.sh"
        The status should be success
        The contents of file "${EVENTS_ROOT}"/* should include '\"containerd_service_cpu_usage\":{\"usage_usec\":\"Not Found\",\"user_usec\":\"60\",\"system_usec\":\"Not Found\",\"nr_periods\":\"Not Found\",\"nr_throttled\":\"3\",\"throttled_usec\":\"Not Found\"}'
    End
End
