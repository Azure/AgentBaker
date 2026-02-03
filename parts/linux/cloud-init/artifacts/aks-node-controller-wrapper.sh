#!/bin/bash
set -uo pipefail

BIN_PATH="${BIN_PATH:-/opt/azure/containers/aks-node-controller}"
CONFIG_PATH="${CONFIG_PATH:-/opt/azure/containers/aks-node-controller-config.json}"
EVENTS_LOGGING_DIR="${EVENTS_LOGGING_DIR:-/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/}"
LOGGER_TAG="aks-node-controller-wrapper"

log() {
    local message="$1"
    # Emit to both journal (via logger) and stdout so systemd captures it.
    logger -t "$LOGGER_TAG" "$message"
    echo "$message"
}

createGuestAgentEvent() {
    local task=$1; startTime=$2; endTime=$3; message=$4;
    local eventsFileName
    eventsFileName=$(date +%s%3N)
    mkdir -p "${EVENTS_LOGGING_DIR}"

    local json_string
    json_string=$(jq -n \
        --arg Timestamp   "${startTime}" \
        --arg OperationId "${endTime}" \
        --arg Version     "1.23" \
        --arg TaskName    "${task}" \
        --arg EventLevel  "Error" \
        --arg Message     "${message}" \
        --arg EventPid    "0" \
        --arg EventTid    "0" \
        '{Timestamp: $Timestamp, OperationId: $OperationId, Version: $Version, TaskName: $TaskName, EventLevel: $EventLevel, Message: $Message, EventPid: $EventPid, EventTid: $EventTid}'
    )

    echo "${json_string}" > "${EVENTS_LOGGING_DIR}${eventsFileName}.json"
}

# this is to ensure that shellspec won't interpret any further lines below
${__SOURCED__:+return}

startTime=$(date +"%F %T.%3N")
log "Launching aks-node-controller with config ${CONFIG_PATH}"
"$BIN_PATH" provision --provision-config="$CONFIG_PATH" &
child_pid=$!
log "Spawned aks-node-controller (pid ${child_pid})"

wait "$child_pid"
exit_code=$?
endTime=$(date +"%F %T.%3N")

if [ "$exit_code" -eq 0 ]; then
    log "aks-node-controller completed successfully"
else
    log "aks-node-controller exited with code ${exit_code}"
    createGuestAgentEvent "AKS.AKSNodeController.UnexpectedError" "$startTime" "$endTime" "aks-node-controller exited with code ${exit_code}"
fi

exit $exit_code
