#!/bin/bash

Describe 'service execution telemetry'
    Include './vhdbuilder/packer/packer_source.sh'

    setup_service_execution_telemetry_test() {
        TEST_ROOT=$(mktemp -d)
        EVENTS_ROOT="${TEST_ROOT}/events"
        PEAK_ROOT="${TEST_ROOT}/peaks"
        CGROUP_ROOT_DIR="${TEST_ROOT}/cgroup"
        CGROUP_PROC="${TEST_ROOT}/proc-cgroup"
        SYSTEMCTL_OUTPUT="${TEST_ROOT}/systemctl-output"
        SYSTEMCTL_MOCK="${TEST_ROOT}/systemctl"
        export MEMORY_PEAK_DIR="${PEAK_ROOT}"

        cat > "${SYSTEMCTL_MOCK}" <<'EOF'
#!/bin/bash
if [[ "$*" == *"--property=MemoryPeak"* ]]; then
    printf '%s\n' "${SYSTEMCTL_MEMORY_PEAK:-}"
    exit "${SYSTEMCTL_MEMORY_PEAK_EXIT_CODE:-0}"
fi
cat "${SYSTEMCTL_OUTPUT}"
exit "${SYSTEMCTL_EXIT_CODE:-0}"
EOF
        chmod +x "${SYSTEMCTL_MOCK}"
    }

    cleanup_service_execution_telemetry_test() {
        rm -rf "${TEST_ROOT}"
        unset MEMORY_PEAK_DIR
    }

    write_cgroup_memory_peak() {
        local unit="$1"
        local peak="$2"
        mkdir -p "${CGROUP_ROOT_DIR}/system.slice/${unit}"
        printf '%s\n' "${peak}" > "${CGROUP_ROOT_DIR}/system.slice/${unit}/memory.peak"
        printf '0::/system.slice/%s\n' "${unit}" > "${CGROUP_PROC}"
    }

    write_systemctl_output() {
        cat > "${SYSTEMCTL_OUTPUT}" <<EOF
ExecMainStartTimestamp=Thu 2026-09-17 10:00:00 UTC
ExecMainExitTimestamp=Thu 2026-09-17 10:00:02 UTC
ExecMainStartTimestampMonotonic=1000000
ExecMainExitTimestampMonotonic=3500000
Result=success
ExecMainCode=1
ExecMainStatus=0
CPUUsageNSec=250000000
EOF
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

    BeforeEach 'setup_service_execution_telemetry_test'
    AfterEach 'cleanup_service_execution_telemetry_test'

    It 'names the event file so WALinuxAgent collects it'
        write_systemctl_output

        When run env \
            SYSTEMCTL_BIN="${SYSTEMCTL_MOCK}" \
            SYSTEMCTL_OUTPUT="${SYSTEMCTL_OUTPUT}" \
            EVENTS_LOGGING_DIR="${EVENTS_ROOT}" \
            bash ./parts/linux/cloud-init/artifacts/service-execution-telemetry.sh cgroup-memory-telemetry.service
        The status should be success
        The result of function emitted_event_file_name_class should equal collectable
    End

    It 'emits execution metrics including a supported memory peak'
        write_systemctl_output

        When run env \
            SYSTEMCTL_BIN="${SYSTEMCTL_MOCK}" \
            SYSTEMCTL_OUTPUT="${SYSTEMCTL_OUTPUT}" \
            SYSTEMCTL_MEMORY_PEAK=1048576 \
            EVENTS_LOGGING_DIR="${EVENTS_ROOT}" \
            bash ./parts/linux/cloud-init/artifacts/service-execution-telemetry.sh cgroup-memory-telemetry.service
        The status should be success
        The contents of file "${EVENTS_ROOT}"/* should include '\"ServiceName\":\"cgroup-memory-telemetry.service\"'
        The contents of file "${EVENTS_ROOT}"/* should include '\"DurationUsec\":2500000'
        The contents of file "${EVENTS_ROOT}"/* should include '\"Result\":\"success\"'
        The contents of file "${EVENTS_ROOT}"/* should include '\"ExecMainCode\":1'
        The contents of file "${EVENTS_ROOT}"/* should include '\"ExecMainStatus\":0'
        The contents of file "${EVENTS_ROOT}"/* should include '\"CPUUsageNSec\":250000000'
        The contents of file "${EVENTS_ROOT}"/* should include '\"MemoryPeakAvailable\":true'
        The contents of file "${EVENTS_ROOT}"/* should include '\"MemoryPeakBytes\":1048576'
    End

    It 'omits the memory peak when systemd does not expose the property'
        write_systemctl_output

        When run env \
            SYSTEMCTL_BIN="${SYSTEMCTL_MOCK}" \
            SYSTEMCTL_OUTPUT="${SYSTEMCTL_OUTPUT}" \
            SYSTEMCTL_MEMORY_PEAK_EXIT_CODE=1 \
            EVENTS_LOGGING_DIR="${EVENTS_ROOT}" \
            bash ./parts/linux/cloud-init/artifacts/service-execution-telemetry.sh cgroup-pressure-telemetry.service
        The status should be success
        The contents of file "${EVENTS_ROOT}"/* should include '\"MemoryPeakAvailable\":false'
        The value "$(jq -r '.Message | fromjson | has("MemoryPeakBytes")' "${EVENTS_ROOT}"/*)" should equal "false"
    End

    It 'emits the failed service result and exit status'
        write_systemctl_output
        sed -i \
            -e 's/Result=success/Result=exit-code/' \
            -e 's/ExecMainStatus=0/ExecMainStatus=42/' \
            "${SYSTEMCTL_OUTPUT}"

        When run env \
            SYSTEMCTL_BIN="${SYSTEMCTL_MOCK}" \
            SYSTEMCTL_OUTPUT="${SYSTEMCTL_OUTPUT}" \
            EVENTS_LOGGING_DIR="${EVENTS_ROOT}" \
            bash ./parts/linux/cloud-init/artifacts/service-execution-telemetry.sh aks-log-collector.service
        The status should be success
        The contents of file "${EVENTS_ROOT}"/* should include '\"Result\":\"exit-code\"'
        The contents of file "${EVENTS_ROOT}"/* should include '\"ExecMainStatus\":42'
    End

    It 'falls back to the ExecStopPost snapshot when systemd retains no peak'
        write_systemctl_output
        mkdir -p "${PEAK_ROOT}"
        printf '%s\n' 2097152 > "${PEAK_ROOT}/cgroup-memory-telemetry.service.peak"

        When run env \
            SYSTEMCTL_BIN="${SYSTEMCTL_MOCK}" \
            SYSTEMCTL_OUTPUT="${SYSTEMCTL_OUTPUT}" \
            SYSTEMCTL_MEMORY_PEAK_EXIT_CODE=1 \
            EVENTS_LOGGING_DIR="${EVENTS_ROOT}" \
            bash ./parts/linux/cloud-init/artifacts/service-execution-telemetry.sh cgroup-memory-telemetry.service
        The status should be success
        The contents of file "${EVENTS_ROOT}"/* should include '\"MemoryPeakAvailable\":true'
        The contents of file "${EVENTS_ROOT}"/* should include '\"MemoryPeakBytes\":2097152'
        The path "${PEAK_ROOT}/cgroup-memory-telemetry.service.peak" should not be exist
    End

    It 'consumes the snapshot so a later execution cannot reuse a stale peak'
        write_systemctl_output
        mkdir -p "${PEAK_ROOT}"
        printf '%s\n' 2097152 > "${PEAK_ROOT}/cgroup-memory-telemetry.service.peak"

        When run env \
            SYSTEMCTL_BIN="${SYSTEMCTL_MOCK}" \
            SYSTEMCTL_OUTPUT="${SYSTEMCTL_OUTPUT}" \
            SYSTEMCTL_MEMORY_PEAK=1048576 \
            EVENTS_LOGGING_DIR="${EVENTS_ROOT}" \
            bash ./parts/linux/cloud-init/artifacts/service-execution-telemetry.sh cgroup-memory-telemetry.service
        The status should be success
        The contents of file "${EVENTS_ROOT}"/* should include '\"MemoryPeakBytes\":1048576'
        The path "${PEAK_ROOT}/cgroup-memory-telemetry.service.peak" should not be exist
    End

    It 'snapshots the cgroup peak memory from ExecStopPost'
        write_cgroup_memory_peak cgroup-pressure-telemetry.service 4194304

        When run env \
            CGROUP_ROOT="${CGROUP_ROOT_DIR}" \
            CGROUP_PROC_FILE="${CGROUP_PROC}" \
            bash ./parts/linux/cloud-init/artifacts/service-execution-telemetry.sh --snapshot-memory-peak cgroup-pressure-telemetry.service
        The status should be success
        The contents of file "${PEAK_ROOT}/cgroup-pressure-telemetry.service.peak" should equal 4194304
    End

    It 'writes no snapshot when the cgroup exposes no peak'
        printf '0::/system.slice/cgroup-pressure-telemetry.service\n' > "${CGROUP_PROC}"

        When run env \
            CGROUP_ROOT="${CGROUP_ROOT_DIR}" \
            CGROUP_PROC_FILE="${CGROUP_PROC}" \
            bash ./parts/linux/cloud-init/artifacts/service-execution-telemetry.sh --snapshot-memory-peak cgroup-pressure-telemetry.service
        The status should be success
        The path "${PEAK_ROOT}/cgroup-pressure-telemetry.service.peak" should not be exist
    End

    It 'refuses to snapshot a unit outside the allowlist'
        write_cgroup_memory_peak some-other.service 4194304

        When run env \
            CGROUP_ROOT="${CGROUP_ROOT_DIR}" \
            CGROUP_PROC_FILE="${CGROUP_PROC}" \
            bash ./parts/linux/cloud-init/artifacts/service-execution-telemetry.sh --snapshot-memory-peak some-other.service
        The status should be success
        The path "${PEAK_ROOT}/some-other.service.peak" should not be exist
    End

    It 'snapshots peak memory from every monitored unit before its cgroup is released'
        for unit in \
            cgroup-memory-telemetry.service \
            cgroup-pressure-telemetry.service \
            aks-log-collector.service; do
            grep -q '^ExecStopPost=-/opt/scripts/service-execution-telemetry.sh --snapshot-memory-peak %n$' \
                "./parts/linux/cloud-init/artifacts/${unit}" || exit 1
        done

        When run true
        The status should be success
    End

    It 'rejects a service outside the explicit allowlist'
        When run env \
            SYSTEMCTL_BIN="${SYSTEMCTL_MOCK}" \
            SYSTEMCTL_OUTPUT="${SYSTEMCTL_OUTPUT}" \
            EVENTS_LOGGING_DIR="${EVENTS_ROOT}" \
            bash ./parts/linux/cloud-init/artifacts/service-execution-telemetry.sh ssh.service
        The status should be failure
        The stderr should include "service 'ssh.service' is not allowlisted"
        The path "${EVENTS_ROOT}" should not be exist
    End

    It 'writes partial telemetry and reports malformed baseline properties'
        write_systemctl_output
        sed -i \
            -e 's/ExecMainExitTimestamp=.*/ExecMainExitTimestamp=/' \
            -e 's/ExecMainExitTimestampMonotonic=3500000/ExecMainExitTimestampMonotonic=invalid/' \
            -e 's/CPUUsageNSec=250000000/CPUUsageNSec=unknown/' \
            "${SYSTEMCTL_OUTPUT}"

        When run env \
            SYSTEMCTL_BIN="${SYSTEMCTL_MOCK}" \
            SYSTEMCTL_OUTPUT="${SYSTEMCTL_OUTPUT}" \
            EVENTS_LOGGING_DIR="${EVENTS_ROOT}" \
            bash ./parts/linux/cloud-init/artifacts/service-execution-telemetry.sh cgroup-memory-telemetry.service
        The status should be failure
        The stderr should include "invalid execution timestamps"
        The stderr should include "incomplete execution properties"
        The contents of file "${EVENTS_ROOT}"/* should include '\"DurationUsec\":null'
        The contents of file "${EVENTS_ROOT}"/* should include '\"CPUUsageNSec\":null'
    End

    It 'reports a systemctl failure without emitting telemetry'
        write_systemctl_output

        When run env \
            SYSTEMCTL_BIN="${SYSTEMCTL_MOCK}" \
            SYSTEMCTL_OUTPUT="${SYSTEMCTL_OUTPUT}" \
            SYSTEMCTL_EXIT_CODE=1 \
            EVENTS_LOGGING_DIR="${EVENTS_ROOT}" \
            bash ./parts/linux/cloud-init/artifacts/service-execution-telemetry.sh cgroup-memory-telemetry.service
        The status should be failure
        The stderr should include "failed to read systemd properties"
        The path "${EVENTS_ROOT}" should not be exist
    End

    It 'configures all monitored units for accounting and completion-triggered collection'
        When run bash -c '
            while read -r unit escaped_unit; do
                grep -q "^CPUAccounting=true$" "./parts/linux/cloud-init/artifacts/${unit}" || exit 1
                grep -q "^MemoryAccounting=true$" "./parts/linux/cloud-init/artifacts/${unit}" || exit 1
                grep -Fqx "OnSuccess=${escaped_unit}" "./parts/linux/cloud-init/artifacts/${unit}" || exit 1
                grep -Fqx "OnFailure=${escaped_unit}" "./parts/linux/cloud-init/artifacts/${unit}" || exit 1
            done <<EOF
cgroup-memory-telemetry.service service-execution-telemetry@cgroup\\x2dmemory\\x2dtelemetry.service.service
cgroup-pressure-telemetry.service service-execution-telemetry@cgroup\\x2dpressure\\x2dtelemetry.service.service
aks-log-collector.service service-execution-telemetry@aks\\x2dlog\\x2dcollector.service.service
EOF
            grep -q "^ExecStart=/opt/scripts/service-execution-telemetry.sh %I$" \
                ./parts/linux/cloud-init/artifacts/service-execution-telemetry@.service
        '
        The status should be success
    End

    It 'uses an asynchronous ExecStopPost fallback before systemd 249'
        CGROUP_MEMORY_TELEMETRY_SERVICE_DEST="${TEST_ROOT}/cgroup-memory-telemetry.service"
        CGROUP_PRESSURE_TELEMETRY_SERVICE_DEST="${TEST_ROOT}/cgroup-pressure-telemetry.service"
        AKS_LOG_COLLECTOR_SERVICE_DEST="${TEST_ROOT}/aks-log-collector.service"
        cp ./parts/linux/cloud-init/artifacts/cgroup-memory-telemetry.service "${CGROUP_MEMORY_TELEMETRY_SERVICE_DEST}"
        cp ./parts/linux/cloud-init/artifacts/cgroup-pressure-telemetry.service "${CGROUP_PRESSURE_TELEMETRY_SERVICE_DEST}"
        cp ./parts/linux/cloud-init/artifacts/aks-log-collector.service "${AKS_LOG_COLLECTOR_SERVICE_DEST}"

        When call configureServiceExecutionTelemetryForSystemdVersion 245
        The status should be success
        The contents of file "${CGROUP_MEMORY_TELEMETRY_SERVICE_DEST}" should not include "OnSuccess="
        The contents of file "${CGROUP_MEMORY_TELEMETRY_SERVICE_DEST}" should not include "OnFailure="
        The contents of file "${CGROUP_MEMORY_TELEMETRY_SERVICE_DEST}" should include 'ExecStopPost=-/bin/systemctl --no-block start service-execution-telemetry@cgroup\x2dmemory\x2dtelemetry.service.service'
        The contents of file "${CGROUP_PRESSURE_TELEMETRY_SERVICE_DEST}" should include 'ExecStopPost=-/bin/systemctl --no-block start service-execution-telemetry@cgroup\x2dpressure\x2dtelemetry.service.service'
        The contents of file "${AKS_LOG_COLLECTOR_SERVICE_DEST}" should include 'ExecStopPost=-/bin/systemctl --no-block start service-execution-telemetry@aks\x2dlog\x2dcollector.service.service'
    End

    It 'keeps completion handlers on systemd 249 and newer'
        CGROUP_MEMORY_TELEMETRY_SERVICE_DEST="${TEST_ROOT}/cgroup-memory-telemetry.service"
        CGROUP_PRESSURE_TELEMETRY_SERVICE_DEST="${TEST_ROOT}/cgroup-pressure-telemetry.service"
        AKS_LOG_COLLECTOR_SERVICE_DEST="${TEST_ROOT}/aks-log-collector.service"
        cp ./parts/linux/cloud-init/artifacts/cgroup-memory-telemetry.service "${CGROUP_MEMORY_TELEMETRY_SERVICE_DEST}"
        cp ./parts/linux/cloud-init/artifacts/cgroup-pressure-telemetry.service "${CGROUP_PRESSURE_TELEMETRY_SERVICE_DEST}"
        cp ./parts/linux/cloud-init/artifacts/aks-log-collector.service "${AKS_LOG_COLLECTOR_SERVICE_DEST}"

        When call configureServiceExecutionTelemetryForSystemdVersion 249
        The status should be success
        The contents of file "${CGROUP_MEMORY_TELEMETRY_SERVICE_DEST}" should include 'OnSuccess=service-execution-telemetry@cgroup\x2dmemory\x2dtelemetry.service.service'
        The contents of file "${CGROUP_MEMORY_TELEMETRY_SERVICE_DEST}" should not include "ExecStopPost=-/bin/systemctl"
        The contents of file "${CGROUP_MEMORY_TELEMETRY_SERVICE_DEST}" should include "--snapshot-memory-peak %n"
    End

    It 'removes obsolete accounting directives on systemd 258 and newer'
        CGROUP_MEMORY_TELEMETRY_SERVICE_DEST="${TEST_ROOT}/cgroup-memory-telemetry.service"
        CGROUP_PRESSURE_TELEMETRY_SERVICE_DEST="${TEST_ROOT}/cgroup-pressure-telemetry.service"
        AKS_LOG_COLLECTOR_SERVICE_DEST="${TEST_ROOT}/aks-log-collector.service"
        cp ./parts/linux/cloud-init/artifacts/cgroup-memory-telemetry.service "${CGROUP_MEMORY_TELEMETRY_SERVICE_DEST}"
        cp ./parts/linux/cloud-init/artifacts/cgroup-pressure-telemetry.service "${CGROUP_PRESSURE_TELEMETRY_SERVICE_DEST}"
        cp ./parts/linux/cloud-init/artifacts/aks-log-collector.service "${AKS_LOG_COLLECTOR_SERVICE_DEST}"

        When call configureServiceExecutionTelemetryForSystemdVersion 258
        The status should be success
        The contents of file "${CGROUP_MEMORY_TELEMETRY_SERVICE_DEST}" should not include "CPUAccounting="
        The contents of file "${CGROUP_MEMORY_TELEMETRY_SERVICE_DEST}" should not include "MemoryAccounting="
        The contents of file "${CGROUP_MEMORY_TELEMETRY_SERVICE_DEST}" should include 'OnSuccess=service-execution-telemetry@cgroup\x2dmemory\x2dtelemetry.service.service'
        The contents of file "${CGROUP_MEMORY_TELEMETRY_SERVICE_DEST}" should not include "ExecStopPost="
    End

    It 'drops the peak-memory snapshot when systemd retains memory accounting'
        CGROUP_MEMORY_TELEMETRY_SERVICE_DEST="${TEST_ROOT}/cgroup-memory-telemetry.service"
        CGROUP_PRESSURE_TELEMETRY_SERVICE_DEST="${TEST_ROOT}/cgroup-pressure-telemetry.service"
        AKS_LOG_COLLECTOR_SERVICE_DEST="${TEST_ROOT}/aks-log-collector.service"
        cp ./parts/linux/cloud-init/artifacts/cgroup-memory-telemetry.service "${CGROUP_MEMORY_TELEMETRY_SERVICE_DEST}"
        cp ./parts/linux/cloud-init/artifacts/cgroup-pressure-telemetry.service "${CGROUP_PRESSURE_TELEMETRY_SERVICE_DEST}"
        cp ./parts/linux/cloud-init/artifacts/aks-log-collector.service "${AKS_LOG_COLLECTOR_SERVICE_DEST}"

        When call configureServiceExecutionTelemetryForSystemdVersion 256
        The status should be success
        The contents of file "${CGROUP_MEMORY_TELEMETRY_SERVICE_DEST}" should not include "--snapshot-memory-peak"
        The contents of file "${CGROUP_PRESSURE_TELEMETRY_SERVICE_DEST}" should not include "--snapshot-memory-peak"
        The contents of file "${AKS_LOG_COLLECTOR_SERVICE_DEST}" should not include "--snapshot-memory-peak"
        The contents of file "${CGROUP_MEMORY_TELEMETRY_SERVICE_DEST}" should include "MemoryAccounting=true"
    End
End
