#!/bin/bash
set -uo pipefail

isNonNegativeInteger() {
    case "$1" in
        ''|*[!0-9]*) return 1 ;;
        *) return 0 ;;
    esac
}

readCounter() {
    local file="$1"
    local key="$2"
    local value

    if [ ! -f "${file}" ]; then
        echo "Not Found"
        return
    fi

    value=$(awk -v key="${key}" '$1 == key { print $2; exit }' "${file}")
    if isNonNegativeInteger "${value}"; then
        echo "${value}"
    else
        echo "Not Found"
    fi
}

getServiceCPUUsage() {
    local service_cgroup="$1"
    local cpu_stat="${CPU_CGROUP}/${service_cgroup}/cpu.stat"

    if [ ! -d "${CPU_CGROUP}/${service_cgroup}" ]; then
        echo '"Not Found"'
        return
    fi

    jq -c -n \
        --arg USAGE_USEC "$(readCounter "${cpu_stat}" usage_usec)" \
        --arg USER_USEC "$(readCounter "${cpu_stat}" user_usec)" \
        --arg SYSTEM_USEC "$(readCounter "${cpu_stat}" system_usec)" \
        --arg NR_PERIODS "$(readCounter "${cpu_stat}" nr_periods)" \
        --arg NR_THROTTLED "$(readCounter "${cpu_stat}" nr_throttled)" \
        --arg THROTTLED_USEC "$(readCounter "${cpu_stat}" throttled_usec)" \
        '{ usage_usec: $USAGE_USEC, user_usec: $USER_USEC, system_usec: $SYSTEM_USEC, nr_periods: $NR_PERIODS, nr_throttled: $NR_THROTTLED, throttled_usec: $THROTTLED_USEC }'
}

EVENTS_LOGGING_DIR=/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/
# WALinuxAgent only collects event files matching ^[0-9]+\.json$ and silently drops
# anything else, so the name must stay digits-only.
EVENTS_FILE_NAME=$(date +%s%3N)
STARTTIME_FORMATTED=$(date +"%F %T.%3N")
ENDTIME_FORMATTED=$(date +"%F %T.%3N")
CGROUP_VERSION=$(stat -fc %T /sys/fs/cgroup)
eventlevel="Microsoft.Azure.Extensions.CustomScript-1.23"

CSLICE=$(systemctl show containerd -p Slice | cut -d= -f2)
KSLICE=$(systemctl show kubelet -p Slice | cut -d= -f2)

if [ "${CGROUP_VERSION}" != "cgroup2fs" ]; then
    echo "cgroup v2 is required. Exiting"
    exit 1
fi

VERSION="cgroupv2"
TASK_NAME="AKS.Runtime.cpu_usage_telemetry_cgroupv2"
CPU_CGROUP="/sys/fs/cgroup"

cpu_usage=$(jq -c -n '{
    counter_units: {
        usage_usec: "microseconds",
        user_usec: "microseconds",
        system_usec: "microseconds",
        nr_periods: "count",
        nr_throttled: "count",
        throttled_usec: "microseconds"
    },
    counter_reset_behavior: "Counters reset when the service cgroup is recreated or the node reboots; downstream rates must discard negative deltas."
}')

while read -r service_key service_cgroup; do
    service_cpu_usage=$(getServiceCPUUsage "${service_cgroup}")
    cpu_usage=$(jq -c \
        --arg SERVICE_KEY "${service_key}" \
        --argjson SERVICE_CPU_USAGE "${service_cpu_usage}" \
        '. + {($SERVICE_KEY): $SERVICE_CPU_USAGE}' <<< "${cpu_usage}")
done <<EOF
containerd_service_cpu_usage ${CSLICE}/containerd.service
kubelet_service_cpu_usage ${KSLICE}/kubelet.service
node_problem_detector_service_cpu_usage system.slice/node-problem-detector.service
node_exporter_service_cpu_usage system.slice/node-exporter.service
sync_container_logs_service_cpu_usage system.slice/sync-container-logs.service
localdns_service_cpu_usage localdns.slice/localdns.service
nvidia_persistenced_service_cpu_usage system.slice/nvidia-persistenced.service
nvidia_gridd_service_cpu_usage system.slice/nvidia-gridd.service
nvidia_fabricmanager_service_cpu_usage system.slice/nvidia-fabricmanager.service
nvidia_device_plugin_service_cpu_usage system.slice/nvidia-device-plugin.service
dra_driver_nvidia_gpu_service_cpu_usage system.slice/dra-driver-nvidia-gpu.service
nvidia_dcgm_service_cpu_usage system.slice/nvidia-dcgm.service
nvidia_dcgm_exporter_service_cpu_usage system.slice/nvidia-dcgm-exporter.service
openibd_service_cpu_usage system.slice/openibd.service
EOF

message_string=$(jq -c -n \
    --arg CGROUPV "${VERSION}" \
    --argjson CPU_USAGE "${cpu_usage}" \
    '{ CgroupVersion: $CGROUPV, CPUUsage: $CPU_USAGE }')

EVENT_JSON=$(jq -n \
    --arg Timestamp "${STARTTIME_FORMATTED}" \
    --arg OperationId "${ENDTIME_FORMATTED}" \
    --arg Version "1.23" \
    --arg TaskName "${TASK_NAME}" \
    --arg EventLevel "${eventlevel}" \
    --arg Message "${message_string}" \
    --arg EventPid "0" \
    --arg EventTid "0" \
    '{Timestamp: $Timestamp, OperationId: $OperationId, Version: $Version, TaskName: $TaskName, EventLevel: $EventLevel, Message: $Message, EventPid: $EventPid, EventTid: $EventTid}')

mkdir -p "${EVENTS_LOGGING_DIR}"
echo "${EVENT_JSON}" > "${EVENTS_LOGGING_DIR}${EVENTS_FILE_NAME}.json"
