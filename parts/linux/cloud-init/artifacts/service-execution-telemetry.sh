#!/bin/bash
set -uo pipefail

isAllowedService() {
    case "$1" in
        cgroup-memory-telemetry.service|cgroup-pressure-telemetry.service|aks-log-collector.service) return 0 ;;
        *) return 1 ;;
    esac
}

numericOrNull() {
    case "$1" in
        ''|*[!0-9]*) echo "null" ;;
        *) echo "$1" ;;
    esac
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

    if [[ "${start_timestamp_monotonic}" =~ ^[0-9]+$ ]] &&
        [[ "${exit_timestamp_monotonic}" =~ ^[0-9]+$ ]] &&
        (( exit_timestamp_monotonic >= start_timestamp_monotonic )); then
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

    if [[ "${memory_peak}" =~ ^[0-9]+$ ]]; then
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
    event_file_name="$(date +%s%3N)-service-execution-${service_name}.json"
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

main "$@"
