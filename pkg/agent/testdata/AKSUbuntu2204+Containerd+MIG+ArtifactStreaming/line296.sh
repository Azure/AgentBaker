#!/bin/bash

ERR_CLOUD_INIT_FAILED=223
EVENTS_LOGGING_DIR=/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/

handleCloudInitStatus() {
    local provisionOutput="$1"
    local cloudInitMessage=""
    
    cloud-init status --wait > /dev/null 2>&1;
    local cloudInitExitCode=$?;

    if [ "$cloudInitExitCode" -eq 0 ]; then
        cloudInitMessage="INFO: cloud-init succeeded"
        echo "${cloudInitMessage}" >> "${provisionOutput}";
        return 0
    elif [ "$cloudInitExitCode" -eq 1 ]; then
        cloudInitMessage="ERROR: cloud-init finished with fatal error (exit code 1)"
        echo "${cloudInitMessage}" >> "${provisionOutput}";
    elif [ "$cloudInitExitCode" -eq 2 ]; then
        cloudInitMessage="WARNING: cloud-init finished with recoverable errors (exit code 2)"
        echo "${cloudInitMessage}" >> "${provisionOutput}";
    else
        cloudInitMessage="WARNING: cloud-init exited with unexpected code: $cloudInitExitCode"
        echo "${cloudInitMessage}" >> "${provisionOutput}";
    fi

    local cloudInitLongStatus=$(cloud-init status --long --format json)
    echo -e "Cloud-init detailed status: \"${cloudInitLongStatus}\"" >> "${provisionOutput}"
    
    jsonCloudInitMessage=$( jq -n \
        --arg cloudInitMessage "${cloudInitMessage}" \
        --arg cloudInitLongStatus "${cloudInitLongStatus}" \
        '{cloudInitMessage: $cloudInitMessage, cloudInitLongStatus: $cloudInitLongStatus}'
    )

    local startTime=$(date +"%F %T.%3N")
    local endTime=$(date +"%F %T.%3N")
    local task="AKS.CSE.CloudInitStatusCheck"
    local eventsFileName=$(date +%s%3N)
    jsonString=$( jq -n \
        --arg Timestamp   "${startTime}" \
        --arg OperationId "${endTime}" \
        --arg Version     "1.23" \
        --arg TaskName    "${task}" \
        --arg EventLevel  "Informational" \
        --arg Message     "${jsonCloudInitMessage}" \
        --arg EventPid    "0" \
        --arg EventTid    "0" \
        '{Timestamp: $Timestamp, OperationId: $OperationId, Version: $Version, TaskName: $TaskName, EventLevel: $EventLevel, Message: $Message, EventPid: $EventPid, EventTid: $EventTid}'
    )
    mkdir -p ${EVENTS_LOGGING_DIR}
    echo ${jsonString} > ${EVENTS_LOGGING_DIR}${eventsFileName}.json

    if [ "$cloudInitExitCode" -eq 1 ]; then 
        return $ERR_CLOUD_INIT_FAILED
    else
        return 0
    fi
}