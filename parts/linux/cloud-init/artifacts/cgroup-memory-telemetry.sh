EVENTS_LOGGING_DIR=/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/
EVENTS_FILE_NAME=$(date +%s%3N)
STARTTIME=$(date)
STARTTIME_FORMATTED=$(date +"%F %T.%3N")
ENDTIME_FORMATTED=$(date +"%F %T.%3N")
CGROUP_VERSION=$(stat -fc %T /sys/fs/cgroup)

CGROUP="/sys/fs/cgroup"
CSLICE=$(systemctl show containerd -p Slice | cut -d= -f2)
KSLICE=$(systemctl show kubelet -p Slice | cut -d= -f2)

if [ "$CGROUP_VERSION" = "cgroup2fs" ]; then

    memory_string=$( jq -n \
        --arg SYSTEM_SLICE_MEMORY "$(($(cat ${CGROUP}/system.slice/memory.stat | awk '/^file /{print $2}') + $(cat ${CGROUP}/system.slice/memory.stat | awk '/^anon /{print $2}')))" \
        --arg AZURE_SLICE_MEMORY "$(($(cat ${CGROUP}/azure.slice/memory.stat | awk '/^file /{print $2}') + $(cat ${CGROUP}/azure.slice/memory.stat | awk '/^anon /{print $2}')))" \
        --arg KUBEPODS_SLICE_MEMORY "$(($(cat ${CGROUP}/kubepods.slice/memory.stat | awk '/^file /{print $2}') + $(cat ${CGROUP}/kubepods.slice/memory.stat | awk '/^anon /{print $2}')))" \
        --arg USER_SLICE_MEMORY "$(($(cat ${CGROUP}/user.slice/memory.stat | awk '/^file /{print $2}') + $(cat ${CGROUP}/user.slice/memory.stat | awk '/^anon /{print $2}')))" \
        --arg CONTAINERD_MEMORY "$(($(cat ${CGROUP}/${CSLICE}/containerd.service/memory.stat | awk '/^file /{print $2}') + $(cat ${CGROUP}/${CSLICE}/containerd.service/memory.stat | awk '/^anon /{print $2}')))" \
        --arg KUBELET_MEMORY "$(($(cat ${CGROUP}/${KSLICE}/kubelet.service/memory.stat | awk '/^file /{print $2}') + $(cat ${CGROUP}/${KSLICE}/kubelet.service/memory.stat | awk '/^anon /{print $2}')))" \
        --arg EMPLOYED_MEMORY "$(($(cat ${CGROUP}/memory.stat | awk '/^file /{print $2}') + $(cat ${CGROUP}/memory.stat | awk '/^anon /{print $2}')))" \
        --arg CAPACITY_MEMORY "$(grep MemTotal /proc/meminfo | awk '{print $2}')" \
        --arg KUBEPODS_CGROUP_MEMORY_MAX "$(cat ${CGROUP}/kubepods.slice/memory.max)" \
        '{ system_slice_memory: $SYSTEM_SLICE_MEMORY, azure_slice_memory: $AZURE_SLICE_MEMORY, kubepods_slice_memory: $KUBEPODS_SLICE_MEMORY, user_slice_memory: $USER_SLICE_MEMORY, containerd_service_memory: $CONTAINERD_MEMORY, kubelet_service_memory: $KUBELET_MEMORY, cgroup_memory: $EMPLOYED_MEMORY, cgroup_capacity_memory: $CAPACITY_MEMORY, kubepods_max_memory: $KUBEPODS_CGROUP_MEMORY_MAX } | tostring'
    )
    
elif [ "$CGROUP_VERSION" = "tmpfs" ]; then

    memory_string=$( jq -n \
        --arg SYSTEM_SLICE_MEMORY "$(($(cat ${CGROUP}/system.slice/memory.stat | awk '/^cache /{print $2}') + $(cat ${CGROUP}/system.slice/memory.stat | awk '/^rss /{print $2}')))" \
        --arg AZURE_SLICE_MEMORY "$(($(cat ${CGROUP}/azure.slice/memory.stat | awk '/^cache /{print $2}') + $(cat ${CGROUP}/azure.slice/memory.stat | awk '/^rss /{print $2}')))" \
        --arg KUBEPODS_SLICE_MEMORY "$(($(cat ${CGROUP}/kubepods.slice/memory.stat | awk '/^cache /{print $2}') + $(cat ${CGROUP}/kubepods.slice/memory.stat | awk '/^rss /{print $2}')))" \
        --arg USER_SLICE_MEMORY "$(($(cat ${CGROUP}/user.slice/memory.stat | awk '/^cache /{print $2}') + $(cat ${CGROUP}/user.slice/memory.stat | awk '/^rss /{print $2}')))" \
        --arg CONTAINERD_MEMORY "$(($(cat ${CGROUP}/${CSLICE}/containerd.service/memory.stat | awk '/^cache /{print $2}') + $(cat ${CGROUP}/${CSLICE}/containerd.service/memory.stat | awk '/^rss /{print $2}')))" \
        --arg KUBELET_MEMORY "$(($(cat ${CGROUP}/${KSLICE}/kubelet.service/memory.stat | awk '/^cache /{print $2}') + $(cat ${CGROUP}/${KSLICE}/kubelet.service/memory.stat | awk '/^rss /{print $2}')))" \
        --arg EMPLOYED_MEMORY "$(($(cat ${CGROUP}/memory.stat | awk '/^cache /{print $2}') + $(cat ${CGROUP}/memory.stat | awk '/^rss /{print $2}')))" \
        --arg CAPACITY_MEMORY "$(grep MemTotal /proc/meminfo | awk '{print $2}')" \
        --arg KUBEPODS_CGROUP_MEMORY_MAX "$(cat ${CGROUP}/kubepods.slice/memory.max)" \
        '{ system_slice_memory: $SYSTEM_SLICE_MEMORY, azure_slice_memory: $AZURE_SLICE_MEMORY, kubepods_slice_memory: $KUBEPODS_SLICE_MEMORY, user_slice_memory: $USER_SLICE_MEMORY, containerd_service_memory: $CONTAINERD_MEMORY, kubelet_service_memory: $KUBELET_MEMORY, cgroup_memory: $EMPLOYED_MEMORY, cgroup_capacity_memory: $CAPACITY_MEMORY, kubepods_max_memory: $KUBEPODS_CGROUP_MEMORY_MAX } | tostring'
    )   

else
    echo "Unexpected cgroup type. Exiting"
    exit 1
fi

memory_string=$(echo $memory_string | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')

message_string=$( jq -n \
    --arg CGROUPV "${CGROUP_VERSION}" \
    --argjson MEMORY "$(echo $memory_string)" \
    '{ CgroupVersion: $CGROUPV, Memory: $MEMORY } | tostring'
)

message_string=$(echo $message_string | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')

EVENT_JSON=$( jq -n \
    --arg Timestamp     "${STARTTIME_FORMATTED}" \
    --arg OperationId   "${ENDTIME_FORMATTED}" \
    --arg Version       "1.23" \
    --arg TaskName      "AKS.Runtime.memory_telemetry" \
    --arg EventLevel    "${eventlevel}" \
    --argjson Message       "${message_string}" \
    --arg EventPid      "0" \
    --arg EventTid      "0" \
    '{Timestamp: $Timestamp, OperationId: $OperationId, Version: $Version, TaskName: $TaskName, EventLevel: $EventLevel, Message: $Message, EventPid: $EventPid, EventTid: $EventTid}'
)

echo ${EVENT_JSON} > ${EVENTS_LOGGING_DIR}${EVENTS_FILE_NAME}.json
cat ${EVENTS_LOGGING_DIR}${EVENTS_FILE_NAME}.json