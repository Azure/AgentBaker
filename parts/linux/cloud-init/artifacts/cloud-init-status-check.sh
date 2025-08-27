#!/bin/bash

# Error indicating that cloud-init returned exit code 1, also reserved in cse_helpers.sh
ERR_CLOUD_INIT_FAILED=223
EVENTS_LOGGING_DIR=/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/

handleCloudInitStatus() {
    local provisionOutput="$1"
    local cloudInitMessage=""
    
    # Capture detailed cloud-init status for tracking of different errors
    cloud-init status --wait > /dev/null 2>&1;
    local cloudInitExitCode=$?;

    if [ "$cloudInitExitCode" -eq 0 ]; then
        cloudInitMessage="INFO: cloud-init succeeded"
        echo "${cloudInitMessage}" >> "${provisionOutput}";
        # we want to avoid logging more information for success case (majority of them)
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

    local fullCloudInitStatus=$(cloud-init status --long --format json)
    echo -e "Cloud-init detailed status: \"${fullCloudInitStatus}\"" >> "${provisionOutput}"
    
    # Extract only errors and recoverable_errors fields from cloud-init status in the event message, full status is still logged in the provision output
    # the top level errors and recoverable_errors fields contain errors aggregated from all cloud-init modules
    local cloudInitErrors=$(echo "${fullCloudInitStatus}" | jq -c '{errors: .errors, recoverable_errors: .recoverable_errors}')
    
    # Combine the status message with detailed status for the event message
    jsonCloudInitMessage=$( jq -n \
        --arg cloudInitMessage "${cloudInitMessage}" \
        --arg cloudInitErrors "${cloudInitErrors}" \
        '{cloudInitMessage: $cloudInitMessage, cloudInitErrors: $cloudInitErrors}'
    )

    # arg names are defined by GA and all these are required to be correctly read by GA
    # EventPid, EventTid are required to be int. No use case for them at this point.
    # based on logs_to_events but with cloud-init specific task name and event message
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
        # if cloud-init exited with code 1, we exit with ERR_CLOUD_INIT_FAILED indicating non-recoverable error in cloud init
        return $ERR_CLOUD_INIT_FAILED
    fi
    # if cloud-init exited with code 2 (recoverable errors), we return 0 to allow CSE to progress
    return 0
}