#!/bin/bash

Describe 'cgroup telemetry'
    setup_cgroup_telemetry_test() {
        TEST_ROOT=$(mktemp -d)
        CGROUP_ROOT="${TEST_ROOT}/cgroup"
        EVENTS_ROOT="${TEST_ROOT}/events"
        export MOCK_WALINUXAGENT_CGROUP="/system.slice/walinuxagent.service"
        export MOCK_SYSTEMCTL_STATUS=0
        export MOCK_WAAGENT_CGROUP=""
        export MOCK_WAAGENT_SYSTEMCTL_STATUS=0
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
            ./parts/linux/cloud-init/artifacts/cgroup-cpu-telemetry.sh > "${TEST_ROOT}/cgroup-cpu-telemetry.sh"
    }

    emitted_event_file_name_class() {
        local name
        name="$(basename "$(ls "${EVENTS_ROOT}"/*)")"
        if printf '%s' "${name}" | grep -Eq '^[0-9]+\.json$'; then
            echo collectable
        else
            echo "not-collectable:${name}"
        fi
    }

    emitted_events() {
        cat "${EVENTS_ROOT}"/*.json
    }

    validate_pressure_events() {
        jq -se '
            def utf8bytelength:
                reduce explode[] as $codepoint (0;
                    . + (if $codepoint < 128 then 1 elif $codepoint < 2048 then 2
                         elif $codepoint < 65536 then 3 else 4 end));
            length > 1 and length <= 6 and
            all(.[]; (.Message | utf8bytelength) <= 3072 and (tojson | utf8bytelength) <= 6144) and
            ([.[].Message | fromjson | .Pressure | keys[]] | length == 18) and
            ([.[].Message | fromjson | .Pressure | keys[]] | unique | length == 18) and
            ([.[].Message | fromjson] | sort_by(.TelemetryChunk.index) |
                length as $count |
                (map(.TelemetryChunk.id) | unique | length == 1) and
                (map(.TelemetryChunk.index) == [range(0; $count)]) and
                all(.[]; .TelemetryChunk.encoding == "json" and .TelemetryChunk.count == $count))
        ' "${EVENTS_ROOT}"/*.json
    }

    BeforeEach 'setup_cgroup_telemetry_test'
    AfterEach 'cleanup_cgroup_telemetry_test'

    Mock systemctl
        case "$*" in
            "show walinuxagent.service -p ControlGroup --value")
                printf '%s\n' "${MOCK_WALINUXAGENT_CGROUP}"
                exit "${MOCK_SYSTEMCTL_STATUS}"
                ;;
            "show waagent.service -p ControlGroup --value")
                printf '%s\n' "${MOCK_WAAGENT_CGROUP}"
                exit "${MOCK_WAAGENT_SYSTEMCTL_STATUS}"
                ;;
            *)
                echo "Unexpected systemctl arguments: $*" >&2
                exit 1
                ;;
        esac
    End

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
        create_memory_stat system.slice/walinuxagent.service 20 30
        MOCK_WAAGENT_CGROUP="/azure.slice/waagent.service"
        create_memory_stat azure.slice/waagent.service 200 300
        create_memory_stat localdns.slice/localdns.service 50 60
        prepare_memory_script

        When run bash "${TEST_ROOT}/cgroup-memory-telemetry.sh"
        The status should be success
        The contents of file "${EVENTS_ROOT}"/* should include '\"containerd_service_memory\":\"3\"'
        The contents of file "${EVENTS_ROOT}"/* should include '\"kubelet_service_memory\":\"3\"'
        The contents of file "${EVENTS_ROOT}"/* should include '\"walinuxagent_service_memory\":\"50\"'
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
        MOCK_WALINUXAGENT_CGROUP="/azure.slice/walinuxagent.service"
        create_memory_stat_v1 azure.slice/walinuxagent.service 20 30
        MOCK_WAAGENT_CGROUP="/azure.slice/waagent.service"
        create_memory_stat_v1 azure.slice/waagent.service 200 300
        prepare_memory_script tmpfs

        When run bash "${TEST_ROOT}/cgroup-memory-telemetry.sh"
        The status should be success
        The contents of file "${EVENTS_ROOT}"/* should include '\"CgroupVersion\":\"cgroupv1\"'
        The contents of file "${EVENTS_ROOT}"/* should include '\"containerd_service_memory\":\"3\"'
        The contents of file "${EVENTS_ROOT}"/* should include '\"walinuxagent_service_memory\":\"50\"'
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
        The result of function emitted_events should include '\"kubelet_service_pressure\":{\"CPUPressure\":{'
        The result of function emitted_events should include '\"containerd_service_pressure\":{\"CPUPressure\":{'
        The result of function emitted_events should include '\"node_problem_detector_service_pressure\":\"Not Found\"'
        The result of function emitted_events should include '\"node_exporter_service_pressure\":{'
        The result of function emitted_events should include '\"CPUPressure\":{\"some_avg10\":\"1.00\",\"some_avg60\":\"2.00\",\"some_avg300\":\"3.00\",\"some_total\":\"4\"}'
        The result of function emitted_events should include '\"sync_container_logs_service_pressure\":{'
        The result of function emitted_events should include '\"localdns_service_pressure\":{'
        The result of function validate_pressure_events should equal true
    End

    It 'bounds pressure events with every supported service and large counters'
        for cgroup_path in \
            . system.slice azure.slice kubepods.slice \
            system.slice/kubelet.service system.slice/containerd.service \
            system.slice/node-problem-detector.service system.slice/node-exporter.service \
            system.slice/sync-container-logs.service localdns.slice/localdns.service \
            system.slice/nvidia-persistenced.service system.slice/nvidia-gridd.service \
            system.slice/nvidia-fabricmanager.service system.slice/nvidia-device-plugin.service \
            system.slice/dra-driver-nvidia-gpu.service system.slice/nvidia-dcgm.service \
            system.slice/nvidia-dcgm-exporter.service system.slice/openibd.service; do
            create_pressure_files "${cgroup_path}"
            sed -i -E 's/total=[0-9]+/total=18446744073709551615/g' "${CGROUP_ROOT}/${cgroup_path}/"*.pressure
        done
        prepare_pressure_script
        validate_service_values() {
            jq -se '[.[].Message | fromjson | .Pressure[]] |
                all(.[]; .CPUPressure.some_total == "18446744073709551615" and
                    .MemoryPressure.full_total == "18446744073709551615" and
                    .IOPressure.full_total == "18446744073709551615")' "${EVENTS_ROOT}"/*.json
        }
        When run bash "${TEST_ROOT}/cgroup-pressure-telemetry.sh"
        The status should be success
        The result of function validate_pressure_events should equal true
        The result of function validate_service_values should equal true
    End

    It 'enforces exact message and serialized event byte limits with ASCII and Unicode'
        check_pressure_size_boundaries() {
            local fits_filter
            fits_filter=$(sed -n '/^    def fits:/p' ./parts/linux/cloud-init/artifacts/cgroup-pressure-telemetry.sh)
            [ -n "${fits_filter}" ] || return 1
            jq -ne "${fits_filter}"'
                def repeat_text($text; $count): [range(0; $count) | $text] | join("");
                [
                    ([3071, 3072, 3073][] as $bytes |
                        {Message: repeat_text("a"; $bytes)} |
                        fits == ($bytes <= 3072)),
                    (["\u00e9", "\u20ac", "\ud83d\ude00"][] as $text |
                        (if $text == "\u00e9" then 1536 elif $text == "\u20ac" then 1024 else 768 end) as $count |
                        repeat_text($text; $count) as $message |
                        ({Message: $message} | fits) and
                        ({Message: ($message + "a")} | fits | not)),
                    ([6143, 6144, 6145][] as $bytes |
                        {Message: "", Padding: ""} |
                        .Padding = repeat_text("a"; $bytes - (tojson | length)) |
                        fits == ($bytes <= 6144)),
                    ({Message: repeat_text("\""; 3072)} | fits | not)
                ] | all
            '
        }

        When call check_pressure_size_boundaries
        The status should be success
        The output should equal true
    End

    Context 'when WALinuxAgent cleans the queue before publication'
        Mock ln
            if [ ! -f "${MOCK_EVENTS_ROOT}/../cleaned" ]; then
                # Match WALinuxAgent's cleanup, including hidden staging files.
                find "${MOCK_EVENTS_ROOT}" -maxdepth 1 -type f -delete
                touch "${MOCK_EVENTS_ROOT}/../cleaned"
            fi
            /bin/ln "$@"
        End

        It 'publishes every pressure fragment despite the concurrent cleanup'
            export MOCK_EVENTS_ROOT="${EVENTS_ROOT}"
            touch "${EVENTS_ROOT}/.residue"
            for cgroup_path in \
                . system.slice azure.slice kubepods.slice \
                system.slice/kubelet.service system.slice/containerd.service; do
                create_pressure_files "${cgroup_path}"
            done
            prepare_pressure_script
            remaining_staging_files() {
                find "${TEST_ROOT}" -maxdepth 1 -name '.pressure-event.*' -print
            }

            When run bash "${TEST_ROOT}/cgroup-pressure-telemetry.sh"
            The status should be success
            The stderr should equal ""
            The file "${TEST_ROOT}/cleaned" should be exist
            The file "${EVENTS_ROOT}/.residue" should not be exist
            The result of function validate_pressure_events should equal true
            The result of function remaining_staging_files should equal ""
        End
    End

    It 'emits cgroup v2 CPU usage and throttling counters with explicit units'
        create_cpu_stat_v2 system.slice/containerd.service
        create_cpu_stat_v2 system.slice/kubelet.service
        MOCK_WALINUXAGENT_CGROUP="/azure.slice/walinuxagent.service"
        create_cpu_stat_v2 azure.slice/walinuxagent.service
        MOCK_WAAGENT_CGROUP="/azure.slice/waagent.service"
        create_cpu_stat_v2 azure.slice/waagent.service
        printf 'usage_usec 999\n' > "${CGROUP_ROOT}/azure.slice/waagent.service/cpu.stat"
        prepare_cpu_script

        When run bash "${TEST_ROOT}/cgroup-cpu-telemetry.sh"
        The status should be success
        The contents of file "${EVENTS_ROOT}"/* should include '\"CgroupVersion\":\"cgroupv2\"'
        The contents of file "${EVENTS_ROOT}"/* should include '\"counter_units\":{\"usage_usec\":\"microseconds\"'
        The contents of file "${EVENTS_ROOT}"/* should include '\"containerd_service_cpu_usage\":{\"usage_usec\":\"100\",\"user_usec\":\"60\",\"system_usec\":\"40\",\"nr_periods\":\"20\",\"nr_throttled\":\"3\",\"throttled_usec\":\"7\"}'
        The contents of file "${EVENTS_ROOT}"/* should include '\"walinuxagent_service_cpu_usage\":{\"usage_usec\":\"100\",\"user_usec\":\"60\",\"system_usec\":\"40\",\"nr_periods\":\"20\",\"nr_throttled\":\"3\",\"throttled_usec\":\"7\"}'
        The contents of file "${EVENTS_ROOT}"/* should include '\"node_exporter_service_cpu_usage\":\"Not Found\"'
        The contents of file "${EVENTS_ROOT}"/* should include 'downstream rates must discard negative deltas'
    End

    It 'names the event file so WALinuxAgent collects it'
        create_cpu_stat_v2 system.slice/containerd.service
        prepare_cpu_script

        When run bash "${TEST_ROOT}/cgroup-cpu-telemetry.sh"
        The status should be success
        # WALinuxAgent only collects event files matching ^(\d+)\.json$; any other name is dropped
        # silently and the observation never reaches telemetry.
        The result of function emitted_event_file_name_class should equal collectable
    End

    It 'rejects cgroup v1'
        prepare_cpu_script tmpfs

        When run bash "${TEST_ROOT}/cgroup-cpu-telemetry.sh"
        The status should be failure
        The output should include 'cgroup v2 is required'
        The path "${EVENTS_ROOT}" should be empty directory
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

    Describe 'fallback WALinuxAgent unit name'
        Parameters
            '' 0
            '/' 0
            '' 1
            '/azure.slice/rejected.service' 1
        End

        BeforeEach 'setup_waagent_fallback'

        setup_waagent_fallback() {
            MOCK_WALINUXAGENT_CGROUP="$1"
            MOCK_SYSTEMCTL_STATUS="$2"
            MOCK_WAAGENT_CGROUP="/azure.slice/waagent.service"
        }

        It 'emits CPU counters from waagent.service'
            create_cpu_stat_v2 azure.slice/waagent.service
            prepare_cpu_script

            When run bash "${TEST_ROOT}/cgroup-cpu-telemetry.sh"
            The status should be success
            The contents of file "${EVENTS_ROOT}"/* should include '\"walinuxagent_service_cpu_usage\":{\"usage_usec\":\"100\",\"user_usec\":\"60\",\"system_usec\":\"40\",\"nr_periods\":\"20\",\"nr_throttled\":\"3\",\"throttled_usec\":\"7\"}'
        End

        It 'emits cgroup v2 memory from waagent.service'
            create_memory_stat azure.slice/waagent.service 20 30
            prepare_memory_script

            When run bash "${TEST_ROOT}/cgroup-memory-telemetry.sh"
            The status should be success
            The contents of file "${EVENTS_ROOT}"/* should include '\"walinuxagent_service_memory\":\"50\"'
        End

        It 'emits cgroup v1 memory from waagent.service'
            create_memory_stat_v1 azure.slice/waagent.service 20 30
            prepare_memory_script tmpfs

            When run bash "${TEST_ROOT}/cgroup-memory-telemetry.sh"
            The status should be success
            The contents of file "${EVENTS_ROOT}"/* should include '\"walinuxagent_service_memory\":\"50\"'
        End
    End

    Describe 'unavailable WALinuxAgent cgroup'
        Parameters
            '' 0
            '/' 0
            '' 1
            '/azure.slice/rejected.service' 1
        End

        It 'does not report root cgroup CPU counters as WALinuxAgent usage'
            MOCK_WALINUXAGENT_CGROUP="$1"
            MOCK_SYSTEMCTL_STATUS="$2"
            MOCK_WAAGENT_CGROUP="$1"
            MOCK_WAAGENT_SYSTEMCTL_STATUS="$2"
            create_cpu_stat_v2 .
            create_cpu_stat_v2 azure.slice/rejected.service
            prepare_cpu_script

            When run bash "${TEST_ROOT}/cgroup-cpu-telemetry.sh"
            The status should be success
            The contents of file "${EVENTS_ROOT}"/* should include '\"walinuxagent_service_cpu_usage\":\"Not Found\"'
        End

        It 'does not report root cgroup v2 memory as WALinuxAgent usage'
            MOCK_WALINUXAGENT_CGROUP="$1"
            MOCK_SYSTEMCTL_STATUS="$2"
            MOCK_WAAGENT_CGROUP="$1"
            MOCK_WAAGENT_SYSTEMCTL_STATUS="$2"
            create_memory_stat . 20 30
            create_memory_stat azure.slice/rejected.service 20 30
            prepare_memory_script

            When run bash "${TEST_ROOT}/cgroup-memory-telemetry.sh"
            The status should be success
            The contents of file "${EVENTS_ROOT}"/* should include '\"walinuxagent_service_memory\":\"Not Found\"'
        End

        It 'does not report root cgroup v1 memory as WALinuxAgent usage'
            MOCK_WALINUXAGENT_CGROUP="$1"
            MOCK_SYSTEMCTL_STATUS="$2"
            MOCK_WAAGENT_CGROUP="$1"
            MOCK_WAAGENT_SYSTEMCTL_STATUS="$2"
            create_memory_stat_v1 . 20 30
            create_memory_stat_v1 azure.slice/rejected.service 20 30
            prepare_memory_script tmpfs

            When run bash "${TEST_ROOT}/cgroup-memory-telemetry.sh"
            The status should be success
            The contents of file "${EVENTS_ROOT}"/* should include '\"walinuxagent_service_memory\":\"Not Found\"'
        End
    End
End
