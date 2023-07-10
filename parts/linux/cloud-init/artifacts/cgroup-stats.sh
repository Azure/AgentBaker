#!/bin/bash
set -o errexit

EVENTS_LOGGING_DIR=/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/
EVENTS_FILE_NAME=$(date +%s%3N)
CSE_STARTTIME=$(date)
CSE_STARTTIME_FORMATTED=$(date +"%F %T.%3N")
CSE_ENDTIME_FORMATTED=$(date +"%F %T.%3N")
NODEPOOL_NAME=$(curl -s -H Metadata:true --noproxy "*" "http://169.254.169.254/metadata/instance?api-version=2021-02-01" | jq -r '.compute.osProfile.computerName')
VM_TYPE=$(kubectl describe node $NODEPOOL_NAME --kubeconfig var/lib/kubelet/kubeconfig | grep "beta.kubernetes.io/instance-type=" | cut -d '=' -f 2)

memory_string=$( jq -n \
--arg USER_SLICE_MEMORY "$(cat /sys/fs/cgroup/user.slice/memory.current)" \
--arg AZURE_SLICE_MEMORY "$(cat /sys/fs/cgroup/azure.slice/memory.current)" \
--arg KUBEPODS_SLICE_MEMORY "$(cat /sys/fs/cgroup/kubepods.slice/memory.current)" \
--arg SYSTEM_SLICE_MEMORY "$(cat /sys/fs/cgroup/system.slice/memory.current)" \
--arg CONTAINERD_MEMORY "$(cat /sys/fs/cgroup/system.slice/containerd.service/memory.current)" \
--arg KUBELET_MEMORY "$(cat /sys/fs/cgroup/system.slice/kubelet.service/memory.current)" \
--arg ALLOCATABLE_MEMORY "$(cat /sys/fs/cgroup/kubepods.slice/memory.max)" \
--arg EMPLOYED_MEMORY "$(( $(cat /sys/fs/cgroup/user.slice/memory.current) + $(cat /sys/fs/cgroup/azure.slice/memory.current) + $(cat /sys/fs/cgroup/kubepods.slice/memory.current) + $(cat /sys/fs/cgroup/system.slice/memory.current) ))" \
--arg CAPACITY_MEMORY "$(kubectl describe node $NODEPOOL_NAME --kubeconfig var/lib/kubelet/kubeconfig | grep "memory:" | head -n 1 | awk '{print $2*1024}' | sed 's/[^0-9]*//g')" \
'{ UserSliceMemory: $USER_SLICE_MEMORY, AzureSliceMemory: $AZURE_SLICE_MEMORY, KubepodsSliceMemory: $KUBEPODS_SLICE_MEMORY, SystemSliceMemory: $SYSTEM_SLICE_MEMORY, ContainerdMemory: $CONTAINERD_MEMORY, KubeletMemory: $KUBELET_MEMORY, AllocatableMemory: $ALLOCATABLE_MEMORY, EmployedMemory: $EMPLOYED_MEMORY, CapacityMemory: $CAPACITY_MEMORY } | tostring'
)

pressure_string=$( jq -n \
--arg MEMORY_PRESSURE "$(cat /sys/fs/cgroup/memory.pressure)" \
--arg IO_PRESSURE "$(cat /sys/fs/cgroup/io.pressure)" \
--arg CPU_PRESSURE "$(cat /sys/fs/cgroup/cpu.pressure)" \
'{ MemoryPressure: $MEMORY_PRESSURE, IoPressure: $IO_PRESSURE, CpuPressure: $CPU_PRESSURE } | tostring'
)

memory_string=$(echo $memory_string | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')
pressure_string=$(echo $pressure_string | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')

message_string=$( jq -n \
    --argjson Memory    "$(echo $memory_string)" \
    --argjson Pressure  "$(echo $pressure_string)" \
    '{ Memory: $Memory, Pressure: $Pressure } | tostring'
)

message_string=$(echo $message_string | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')

EVENT_JSON=$( jq -n \
    --arg Timestamp     "${CSE_STARTTIME_FORMATTED}" \
    --arg OperationId   "${CSE_ENDTIME_FORMATTED}" \
    --arg Version       "1.23" \
    --arg TaskName      "AKS.CSE.system_slice" \
    --arg EventLevel    "${eventlevel}" \
    --arg Message       "${message_string}" \
    --arg EventPid      "0" \
    --arg EventTid      "0" \
    '{Timestamp: $Timestamp, OperationId: $OperationId, Version: $Version, TaskName: $TaskName, EventLevel: $EventLevel, Message: $Message, EventPid: $EventPid, EventTid: $EventTid}'
)

echo ${EVENT_JSON} > ${EVENTS_LOGGING_DIR}${EVENTS_FILE_NAME}.json
cat ${EVENTS_LOGGING_DIR}${EVENTS_FILE_NAME}.json