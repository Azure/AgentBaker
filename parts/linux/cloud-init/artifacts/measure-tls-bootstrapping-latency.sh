#!/bin/bash
set -uo pipefail

# This script is used to watch for the kubelet TLS bootstrapping process to completed, whether that be via vanilla or secure TLS bootstrapping.
# specifically this script will watch for the creation of a kubeconfig file at the path specified by KUBECONFIG_PATH with a timeout.
# if the kubeconfig file is created within the timeout period, a guest agent event will be emitted so we can measure how long it took kubelet
# to acquire a fresh client certificate from the control plane. If a kubeconfig file is not created within the timeout period, a guest agent event will
# also be emitted as an indication that TLS bootstrapping seemingly failed.

EVENTS_LOGGING_DIR=/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/

KUBECONFIG_PATH="${KUBECONFIG_PATH:-/var/lib/kubelet/kubeconfig}"
KUBECONFIG_DIR="$(dirname "$KUBECONFIG_PATH")"
TLS_BOOTSTRAPPING_START_TIME_FILEPATH="${TLS_BOOTSTRAPPING_START_TIME_FILEPATH:-/opt/azure/containers/tls-bootstrap-start-time}"

WATCH_TIMEOUT_SECONDS=${WATCH_TIMEOUT_SECONDS:-300}  # default to 5 minutes

createGuestAgentEvent() {
    local task=$1; startTime=$2; endTime=$3;
    local eventsFileName
    eventsFileName=$(date +%s%3N)

    json_string=$( jq -n \
        --arg Timestamp   "${startTime}" \
        --arg OperationId "${endTime}" \
        --arg Version     "1.23" \
        --arg TaskName    "${task}" \
        --arg EventLevel  "Informational" \
        --arg Message     "Completed" \
        --arg EventPid    "0" \
        --arg EventTid    "0" \
        '{Timestamp: $Timestamp, OperationId: $OperationId, Version: $Version, TaskName: $TaskName, EventLevel: $EventLevel, Message: $Message, EventPid: $EventPid, EventTid: $EventTid}'
    )
    echo "${json_string}" > "${EVENTS_LOGGING_DIR}${eventsFileName}.json"
}

emitTLSBootstrappingCompletedEvent() {
    local start_time=$1
    local end_time
    end_time=$(date +"%F %T.%3N")
    createGuestAgentEvent "AKS.Runtime.waitForTLSBootstrapping" "$start_time" "$end_time"
}

waitForTLSBootstrapping() {
    # exit without doing anything if we don't have inotifywait available
    if ! command -v inotifywait >/dev/null 2>&1; then
        echo "inotifywait is not available, unable to wait for TLS bootstrapping"
        exit 0
    fi

    if [ ! -s "$TLS_BOOTSTRAPPING_START_TIME_FILEPATH" ]; then
        echo "TLS bootstrapping start time file not found at: $TLS_BOOTSTRAPPING_START_TIME_FILEPATH"
        exit 0
    fi

    START_TIME=$(cat "$TLS_BOOTSTRAPPING_START_TIME_FILEPATH")
    trap 'rm -f "$TLS_BOOTSTRAPPING_START_TIME_FILEPATH"' EXIT

    # ensure the kubeconfig dir exists
    mkdir -p "$KUBECONFIG_DIR"

    # If kubeconfig already exists, kubelet finished TLS bootstrapping before this service started.
    # Emit the completed event using the start time written immediately before kubelet startup.
    if [ -f "$KUBECONFIG_PATH" ]; then
        echo "kubeconfig already exists at: $KUBECONFIG_PATH"
        emitTLSBootstrappingCompletedEvent "$START_TIME"
        exit 0
    fi

    echo "watching for kubeconfig to be created at $KUBECONFIG_PATH with ${WATCH_TIMEOUT_SECONDS}s timeout..."

    inotifywait -t "$WATCH_TIMEOUT_SECONDS" -qme create "$KUBECONFIG_DIR" | while read -r DIR EVENT FILE; do
        if [ "${EVENT,,}" = "create" ] && [ "${DIR}${FILE}" = "$KUBECONFIG_PATH" ]; then
            echo "new kubeconfig created at: $KUBECONFIG_PATH"

            emitTLSBootstrappingCompletedEvent "$START_TIME"

            # this is ugly, but it's the best way to ensure that we don't leave inotifywait running in the background consuming resources
            kill -- -$$
        fi
    done

    # Check once more in case kubeconfig was created after the initial existence check but before
    # inotifywait started listening. This preserves the latency signal for fast kubelet startups.
    if [ -f "$KUBECONFIG_PATH" ]; then
        echo "kubeconfig now exists at: $KUBECONFIG_PATH"
        emitTLSBootstrappingCompletedEvent "$START_TIME"
    else
        END_TIME=$(date +"%F %T.%3N")
        echo "kubeconfig was not created after ${WATCH_TIMEOUT_SECONDS}s"
        createGuestAgentEvent "AKS.Runtime.waitForTLSBootstrappingTimeout" "$START_TIME" "$END_TIME"
    fi
}

# this is to ensure that shellspec won't interpret any further lines below
${__SOURCED__:+return}

waitForTLSBootstrapping
