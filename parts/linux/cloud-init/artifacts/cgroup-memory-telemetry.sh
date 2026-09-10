#!/bin/bash
set -uo pipefail

getServiceMemory() {
    local memory_stat="$1"
    local cgroup_version="$2"

    if [ ! -f "${memory_stat}" ]; then
        echo "Not Found"
        return
    fi

    if [ "${cgroup_version}" = "cgroupv2" ]; then
        expr "$(awk '/^file /{print $2}' "${memory_stat}")" + "$(awk '/^anon /{print $2}' "${memory_stat}")"
    else
        expr "$(awk '/^total_cache /{print $2}' "${memory_stat}")" + "$(awk '/^total_rss /{print $2}' "${memory_stat}")"
    fi
}

EVENTS_LOGGING_DIR=/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/
EVENTS_FILE_NAME=$(date +%s%3N)
STARTTIME_FORMATTED=$(date +"%F %T.%3N")
ENDTIME_FORMATTED=$(date +"%F %T.%3N")
CGROUP_VERSION=$(stat -fc %T /sys/fs/cgroup)
eventlevel="Microsoft.Azure.Extensions.CustomScript-1.23"

CSLICE=$(systemctl show containerd -p Slice | cut -d= -f2)
KSLICE=$(systemctl show kubelet -p Slice | cut -d= -f2)

if [ "$CGROUP_VERSION" = "cgroup2fs" ]; then
    VERSION="cgroupv2"
    TASK_NAME="AKS.Runtime.memory_telemetry_cgroupv2"
    CGROUP="/sys/fs/cgroup"

    memory_string=$( jq -n \
        --arg SYSTEM_SLICE_MEMORY "$(if [ -f "${CGROUP}/system.slice/memory.stat" ]; then echo $(expr $(cat ${CGROUP}/system.slice/memory.stat | awk '/^file /{print $2}') + $(cat ${CGROUP}/system.slice/memory.stat | awk '/^anon /{print $2}')); else echo "Not Found"; fi)" \
        --arg AZURE_SLICE_MEMORY "$(if [ -f "${CGROUP}/azure.slice/memory.stat" ]; then echo $(expr $(cat ${CGROUP}/azure.slice/memory.stat | awk '/^file /{print $2}') + $(cat ${CGROUP}/azure.slice/memory.stat | awk '/^anon /{print $2}')); else echo "Not Found"; fi)" \
        --arg KUBEPODS_SLICE_MEMORY "$(if [ -f "${CGROUP}/kubepods.slice/memory.stat" ]; then echo $(expr $(cat ${CGROUP}/kubepods.slice/memory.stat | awk '/^file /{print $2}') + $(cat ${CGROUP}/kubepods.slice/memory.stat | awk '/^anon /{print $2}')); else echo "Not Found"; fi)" \
        --arg USER_SLICE_MEMORY "$(if [ -f "${CGROUP}/user.slice/memory.stat" ]; then echo $(expr $(cat ${CGROUP}/user.slice/memory.stat | awk '/^file /{print $2}') + $(cat ${CGROUP}/user.slice/memory.stat | awk '/^anon /{print $2}')); else echo "Not Found"; fi)" \
        --arg CONTAINERD_MEMORY "$(getServiceMemory "${CGROUP}/${CSLICE}/containerd.service/memory.stat" "${VERSION}")" \
        --arg KUBELET_MEMORY "$(getServiceMemory "${CGROUP}/${KSLICE}/kubelet.service/memory.stat" "${VERSION}")" \
        --arg NODE_PROBLEM_DETECTOR_MEMORY "$(getServiceMemory "${CGROUP}/system.slice/node-problem-detector.service/memory.stat" "${VERSION}")" \
        --arg NODE_EXPORTER_MEMORY "$(getServiceMemory "${CGROUP}/system.slice/node-exporter.service/memory.stat" "${VERSION}")" \
        --arg SYNC_CONTAINER_LOGS_MEMORY "$(getServiceMemory "${CGROUP}/system.slice/sync-container-logs.service/memory.stat" "${VERSION}")" \
        --arg LOCALDNS_MEMORY "$(getServiceMemory "${CGROUP}/localdns.slice/localdns.service/memory.stat" "${VERSION}")" \
        --arg EMPLOYED_MEMORY "$(if [ -f "${CGROUP}/memory.stat" ]; then echo $(expr $(cat ${CGROUP}/memory.stat | awk '/^file /{print $2}') + $(cat ${CGROUP}/memory.stat | awk '/^anon /{print $2}')); else echo "Not Found"; fi)" \
        --arg CAPACITY_MEMORY "$(grep MemTotal /proc/meminfo | awk '{print $2}' | awk '{print $1 * 1000}')" \
        --arg KUBEPODS_CGROUP_MEMORY_MAX "$(if [ -f "${CGROUP}/kubepods.slice/memory.max" ]; then cat ${CGROUP}/kubepods.slice/memory.max; else echo "Not Found"; fi)" \
        '{ system_slice_memory: $SYSTEM_SLICE_MEMORY, azure_slice_memory: $AZURE_SLICE_MEMORY, kubepods_slice_memory: $KUBEPODS_SLICE_MEMORY, user_slice_memory: $USER_SLICE_MEMORY, containerd_service_memory: $CONTAINERD_MEMORY, kubelet_service_memory: $KUBELET_MEMORY, node_problem_detector_service_memory: $NODE_PROBLEM_DETECTOR_MEMORY, node_exporter_service_memory: $NODE_EXPORTER_MEMORY, sync_container_logs_service_memory: $SYNC_CONTAINER_LOGS_MEMORY, localdns_service_memory: $LOCALDNS_MEMORY, cgroup_memory: $EMPLOYED_MEMORY, cgroup_capacity_memory: $CAPACITY_MEMORY, kubepods_max_memory: $KUBEPODS_CGROUP_MEMORY_MAX } | tostring'
    )
elif [ "$CGROUP_VERSION" = "tmpfs" ]; then
    VERSION="cgroupv1"
    TASK_NAME="AKS.Runtime.memory_telemetry_cgroupv1"
    CGROUP="/sys/fs/cgroup/memory"

    memory_string=$( jq -n \
        --arg SYSTEM_SLICE_MEMORY "$(if [ -f ${CGROUP}/system.slice/memory.stat ]; then expr $(cat ${CGROUP}/system.slice/memory.stat | awk '/^total_cache /{print $2}') + $(cat ${CGROUP}/system.slice/memory.stat | awk '/^total_rss /{print $2}'); else echo "Not Found"; fi)" \
        --arg AZURE_SLICE_MEMORY "$(if [ -f ${CGROUP}/azure.slice/memory.stat ]; then expr $(cat ${CGROUP}/azure.slice/memory.stat | awk '/^total_cache /{print $2}') + $(cat ${CGROUP}/azure.slice/memory.stat | awk '/^total_rss /{print $2}'); else echo "Not Found"; fi)" \
        --arg KUBEPODS_SLICE_MEMORY "$(if [ -f ${CGROUP}/kubepods/memory.stat ]; then expr $(cat ${CGROUP}/kubepods/memory.stat | awk '/^total_cache /{print $2}') + $(cat ${CGROUP}/kubepods/memory.stat | awk '/^total_rss /{print $2}'); else echo "Not Found"; fi)" \
        --arg USER_SLICE_MEMORY "$(if [ -f ${CGROUP}/user.slice/memory.stat ]; then expr $(cat ${CGROUP}/user.slice/memory.stat | awk '/^total_cache /{print $2}') + $(cat ${CGROUP}/user.slice/memory.stat | awk '/^total_rss /{print $2}'); else echo "Not Found"; fi)" \
        --arg CONTAINERD_MEMORY "$(getServiceMemory "${CGROUP}/${CSLICE}/containerd.service/memory.stat" "${VERSION}")" \
        --arg KUBELET_MEMORY "$(getServiceMemory "${CGROUP}/${KSLICE}/kubelet.service/memory.stat" "${VERSION}")" \
        --arg NODE_PROBLEM_DETECTOR_MEMORY "$(getServiceMemory "${CGROUP}/system.slice/node-problem-detector.service/memory.stat" "${VERSION}")" \
        --arg NODE_EXPORTER_MEMORY "$(getServiceMemory "${CGROUP}/system.slice/node-exporter.service/memory.stat" "${VERSION}")" \
        --arg SYNC_CONTAINER_LOGS_MEMORY "$(getServiceMemory "${CGROUP}/system.slice/sync-container-logs.service/memory.stat" "${VERSION}")" \
        --arg LOCALDNS_MEMORY "$(getServiceMemory "${CGROUP}/localdns.slice/localdns.service/memory.stat" "${VERSION}")" \
        --arg EMPLOYED_MEMORY "$(if [ -f ${CGROUP}/memory.stat ]; then expr $(cat ${CGROUP}/memory.stat | awk '/^total_cache /{print $2}') + $(cat ${CGROUP}/memory.stat | awk '/^total_rss /{print $2}'); else echo "Not Found"; fi)" \
        --arg CAPACITY_MEMORY "$(grep MemTotal /proc/meminfo | awk '{print $2}' | awk '{print $1 * 1000}')" \
        --arg KUBEPODS_CGROUP_MEMORY_MAX "$(if [ -f ${CGROUP}/kubepods/memory.limit_in_bytes ]; then cat ${CGROUP}/kubepods/memory.limit_in_bytes; else echo "Not Found"; fi)" \
        '{ system_slice_memory: $SYSTEM_SLICE_MEMORY, azure_slice_memory: $AZURE_SLICE_MEMORY, kubepods_slice_memory: $KUBEPODS_SLICE_MEMORY, user_slice_memory: $USER_SLICE_MEMORY, containerd_service_memory: $CONTAINERD_MEMORY, kubelet_service_memory: $KUBELET_MEMORY, node_problem_detector_service_memory: $NODE_PROBLEM_DETECTOR_MEMORY, node_exporter_service_memory: $NODE_EXPORTER_MEMORY, sync_container_logs_service_memory: $SYNC_CONTAINER_LOGS_MEMORY, localdns_service_memory: $LOCALDNS_MEMORY, cgroup_memory: $EMPLOYED_MEMORY, cgroup_capacity_memory: $CAPACITY_MEMORY, kubepods_max_memory: $KUBEPODS_CGROUP_MEMORY_MAX } | tostring'
    )
else
    echo "Unexpected cgroup type. Exiting"
    exit 1
fi

memory_string=$(echo $memory_string | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')

message_string=$( jq -n \
    --arg CGROUPV "${VERSION}" \
    --argjson MEMORY "$(echo $memory_string)" \
    '{ CgroupVersion: $CGROUPV, Memory: $MEMORY } | tostring'
)

message_string=$(echo $message_string | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')

EVENT_JSON=$( jq -n \
    --arg Timestamp     "${STARTTIME_FORMATTED}" \
    --arg OperationId   "${ENDTIME_FORMATTED}" \
    --arg Version       "1.23" \
    --arg TaskName      "${TASK_NAME}" \
    --arg EventLevel    "${eventlevel}" \
    --arg Message    "${message_string}" \
    --arg EventPid      "0" \
    --arg EventTid      "0" \
    '{Timestamp: $Timestamp, OperationId: $OperationId, Version: $Version, TaskName: $TaskName, EventLevel: $EventLevel, Message: $Message, EventPid: $EventPid, EventTid: $EventTid}'
)

mkdir -p "${EVENTS_LOGGING_DIR}"
echo ${EVENT_JSON} > ${EVENTS_LOGGING_DIR}${EVENTS_FILE_NAME}.json
