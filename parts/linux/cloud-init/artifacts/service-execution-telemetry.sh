#!/bin/bash
set -uo pipefail

isAllowedService() {
    case "$1" in
        cgroup-memory-telemetry.service|cgroup-pressure-telemetry.service|aks-log-collector.service) return 0 ;;
        *) return 1 ;;
    esac
}

isNonNegativeInteger() {
    case "$1" in
        ''|*[!0-9]*) return 1 ;;
        *) return 0 ;;
    esac
}

numericOrNull() {
    if isNonNegativeInteger "$1"; then
        echo "$1"
    else
        echo "null"
    fi
}

# Runs from ExecStopPost of a monitored unit, inside that unit's own cgroup and
# before systemd releases it. Systemd keeps CPU accounting of an exited unit but
# only retains memory accounting from version 256, so on older systemd the peak
# has to be read straight from the cgroup while it still exists.
snapshotMemoryPeak() {
    local unit_name="${1:-}"
    local snapshot_dir="${MEMORY_PEAK_DIR:-/run/aks-service-telemetry}"
    local cgroup_proc_file="${CGROUP_PROC_FILE:-/proc/self/cgroup}"
    local cgroup_root="${CGROUP_ROOT:-/sys/fs/cgroup}"
    local cgroup_path
    local peak

    isAllowedService "${unit_name}" || return 0

    cgroup_path=$(sed -n 's|^0::||p' "${cgroup_proc_file}" 2>/dev/null)
    [ -n "${cgroup_path}" ] || return 0

    peak=$(cat "${cgroup_root%/}${cgroup_path}/memory.peak" 2>/dev/null)
    isNonNegativeInteger "${peak}" || return 0

    mkdir -p "${snapshot_dir}" 2>/dev/null || return 0
    printf '%s\n' "${peak}" > "${snapshot_dir%/}/${unit_name}.peak" 2>/dev/null || return 0
}

main() {
    local service_name="${1:-}"
    local systemctl_bin="${SYSTEMCTL_BIN:-systemctl}"
    local events_logging_dir="${EVENTS_LOGGING_DIR:-/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/}"
    local properties
    local property
    local value
    local start_timestamp=""
    local exit_timestamp=""
    local start_timestamp_monotonic=""
    local exit_timestamp_monotonic=""
    local result=""
    local exec_main_code=""
    local exec_main_status=""
    local cpu_usage_nsec=""
    local memory_peak=""
    local memory_peak_dir="${MEMORY_PEAK_DIR:-/run/aks-service-telemetry}"
    local memory_peak_file
    local duration_usec="null"
    local exec_main_code_json
    local exec_main_status_json
    local cpu_usage_nsec_json
    local memory_peak_json="null"
    local memory_peak_available="false"
    local collection_complete="true"
    local message
    local event_json
    local event_timestamp
    local event_file_name

    if ! isAllowedService "${service_name}"; then
        echo "service execution telemetry: service '${service_name}' is not allowlisted" >&2
        return 1
    fi

    if ! properties=$("${systemctl_bin}" show "${service_name}" \
        --property=ExecMainStartTimestamp \
        --property=ExecMainExitTimestamp \
        --property=ExecMainStartTimestampMonotonic \
        --property=ExecMainExitTimestampMonotonic \
        --property=Result \
        --property=ExecMainCode \
        --property=ExecMainStatus \
        --property=CPUUsageNSec); then
        echo "service execution telemetry: failed to read systemd properties for '${service_name}'" >&2
        return 1
    fi

    while IFS='=' read -r property value; do
        case "${property}" in
            ExecMainStartTimestamp) start_timestamp="${value}" ;;
            ExecMainExitTimestamp) exit_timestamp="${value}" ;;
            ExecMainStartTimestampMonotonic) start_timestamp_monotonic="${value}" ;;
            ExecMainExitTimestampMonotonic) exit_timestamp_monotonic="${value}" ;;
            Result) result="${value}" ;;
            ExecMainCode) exec_main_code="${value}" ;;
            ExecMainStatus) exec_main_status="${value}" ;;
            CPUUsageNSec) cpu_usage_nsec="${value}" ;;
        esac
    done <<< "${properties}"

    memory_peak=$("${systemctl_bin}" show "${service_name}" --property=MemoryPeak --value 2>/dev/null || true)

    # Before systemd 256 the memory accounting of a unit is discarded together with
    # its cgroup, so MemoryPeak reads empty once the unit has exited. The monitored
    # units therefore snapshot cgroup memory.peak from ExecStopPost, while the cgroup
    # still exists. Consume that snapshot when systemd itself has nothing left.
    memory_peak_file="${memory_peak_dir%/}/${service_name}.peak"
    if ! isNonNegativeInteger "${memory_peak}" && [ -r "${memory_peak_file}" ]; then
        memory_peak=$(cat "${memory_peak_file}" 2>/dev/null || true)
    fi
    rm -f "${memory_peak_file}" 2>/dev/null || true

    if isNonNegativeInteger "${start_timestamp_monotonic}" &&
        isNonNegativeInteger "${exit_timestamp_monotonic}" &&
        [ "${exit_timestamp_monotonic}" -ge "${start_timestamp_monotonic}" ]; then
        duration_usec=$((exit_timestamp_monotonic - start_timestamp_monotonic))
    else
        echo "service execution telemetry: invalid execution timestamps for '${service_name}'" >&2
        collection_complete="false"
    fi

    exec_main_code_json=$(numericOrNull "${exec_main_code}")
    exec_main_status_json=$(numericOrNull "${exec_main_status}")
    cpu_usage_nsec_json=$(numericOrNull "${cpu_usage_nsec}")

    if [ -z "${start_timestamp}" ] || [ -z "${exit_timestamp}" ] ||
        [ "${exec_main_code_json}" = "null" ] || [ "${exec_main_status_json}" = "null" ] ||
        [ "${cpu_usage_nsec_json}" = "null" ] || [ -z "${result}" ]; then
        echo "service execution telemetry: incomplete execution properties for '${service_name}'" >&2
        collection_complete="false"
    fi

    if isNonNegativeInteger "${memory_peak}"; then
        memory_peak_json="${memory_peak}"
        memory_peak_available="true"
    fi

    message=$(jq -c -n \
        --arg SERVICE_NAME "${service_name}" \
        --arg START_TIMESTAMP "${start_timestamp}" \
        --arg EXIT_TIMESTAMP "${exit_timestamp}" \
        --arg RESULT "${result}" \
        --argjson DURATION_USEC "${duration_usec}" \
        --argjson EXEC_MAIN_CODE "${exec_main_code_json}" \
        --argjson EXEC_MAIN_STATUS "${exec_main_status_json}" \
        --argjson CPU_USAGE_NSEC "${cpu_usage_nsec_json}" \
        --argjson MEMORY_PEAK_AVAILABLE "${memory_peak_available}" \
        --argjson MEMORY_PEAK_BYTES "${memory_peak_json}" \
        '{
            ServiceName: $SERVICE_NAME,
            ExecMainStartTimestamp: $START_TIMESTAMP,
            ExecMainExitTimestamp: $EXIT_TIMESTAMP,
            DurationUsec: $DURATION_USEC,
            Result: $RESULT,
            ExecMainCode: $EXEC_MAIN_CODE,
            ExecMainStatus: $EXEC_MAIN_STATUS,
            CPUUsageNSec: $CPU_USAGE_NSEC,
            MemoryPeakAvailable: $MEMORY_PEAK_AVAILABLE,
            MetricUnits: {
                DurationUsec: "microseconds",
                CPUUsageNSec: "nanoseconds",
                MemoryPeakBytes: "bytes"
            },
            ResetBehavior: "Values describe one service execution and reset when systemd recreates the service cgroup."
        } + if $MEMORY_PEAK_AVAILABLE then {MemoryPeakBytes: $MEMORY_PEAK_BYTES} else {} end')

    event_timestamp=$(date +"%F %T.%3N")
    # WALinuxAgent only collects extension event files matching ^[0-9]+\.json$ and
    # silently drops anything else, so the name must stay digits-only.
    event_file_name="$(date +%s%3N).json"
    event_json=$(jq -n \
        --arg Timestamp "${event_timestamp}" \
        --arg OperationId "${event_timestamp}" \
        --arg Version "1.23" \
        --arg TaskName "AKS.Runtime.systemd_service_execution" \
        --arg EventLevel "Microsoft.Azure.Extensions.CustomScript-1.23" \
        --arg Message "${message}" \
        --arg EventPid "0" \
        --arg EventTid "0" \
        '{Timestamp: $Timestamp, OperationId: $OperationId, Version: $Version, TaskName: $TaskName, EventLevel: $EventLevel, Message: $Message, EventPid: $EventPid, EventTid: $EventTid}')

    if ! mkdir -p "${events_logging_dir}"; then
        echo "service execution telemetry: failed to create '${events_logging_dir}'" >&2
        return 1
    fi
    if ! printf '%s\n' "${event_json}" > "${events_logging_dir%/}/${event_file_name}"; then
        echo "service execution telemetry: failed to write telemetry for '${service_name}'" >&2
        return 1
    fi

    [ "${collection_complete}" = "true" ]
}

if [ "${1:-}" = "--snapshot-memory-peak" ]; then
    snapshotMemoryPeak "${2:-}"
else
    main "$@"
fi
