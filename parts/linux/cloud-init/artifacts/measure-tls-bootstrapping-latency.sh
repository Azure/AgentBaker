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

WATCH_TIMEOUT_SECONDS=${WATCH_TIMEOUT_SECONDS:-300}  # default to 5 minutes

createGuestAgentEvent() {
    local task=$1; startTime=$2; endTime=$3;
    local eventsFileName=$(date +%s%3N)

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
    echo ${json_string} > ${EVENTS_LOGGING_DIR}${eventsFileName}.json
}

waitForTLSBootstrapping() {
    # exit without doing anything if we don't have inotifywait available
    if ! command -v inotifywait >/dev/null 2>&1; then
        echo "inotifywait is not available, unable to wait for TLS bootstrapping"
        exit 0
    fi

    # ensure the kubeconfig dir exists
    mkdir -p "$KUBECONFIG_DIR"

    # check if a kubeconfig already exists, in which case there's nothing to wait for or measure
    if [ -f "$KUBECONFIG_PATH" ]; then
        echo "kubeconfig already exists at: $KUBECONFIG_PATH"
        exit 0
    fi

    echo "watching for kubeconfig to be created at $KUBECONFIG_PATH with ${WATCH_TIMEOUT_SECONDS}s timeout..."

    START_TIME=$(date +"%F %T.%3N")
    inotifywait -t $WATCH_TIMEOUT_SECONDS -qme create "$KUBECONFIG_DIR" | while read -r DIR EVENT FILE; do
        if [ "${EVENT,,}" = "create" ] && [ "${DIR}${FILE}" = "$KUBECONFIG_PATH" ]; then
            END_TIME=$(date +"%F %T.%3N")
            echo "new kubeconfig created at: $KUBECONFIG_PATH"

            # we only create the guest agent event if the certificate was created while we were watching
            createGuestAgentEvent "AKS.Runtime.waitForTLSBootstrapping" "$START_TIME" "$END_TIME"

            # this is ugly, but it's the best way to ensure that we don't leave inotifywait running in the background consuming resources
            kill -- -$$
        fi
    done

    # check once more if there was a kubeconfig created after finishing the inotifywait loop
    # to avoid data skewing, we don't emit a guest agent event in this case
    # this would only happen if we hit a race condition between first checking kubeconfig existence and starting inotifywait
    if [ -f "$KUBECONFIG_PATH" ]; then
        echo "kubeconfig now exists at: $KUBECONFIG_PATH"
    else
        END_TIME=$(date +"%F %T.%3N")
        echo "kubeconfig was not created after ${WATCH_TIMEOUT_SECONDS}s"
        createGuestAgentEvent "AKS.Runtime.waitForTLSBootstrappingTimeout" "$START_TIME" "$END_TIME"
    fi
}

# this is to ensure that shellspec won't interpret any further lines below
${__SOURCED__:+return}

waitForTLSBootstrapping
