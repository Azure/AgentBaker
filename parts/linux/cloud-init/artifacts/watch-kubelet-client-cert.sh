#!/bin/bash
set -uo pipefail

KUBERNETES_CERT_DIR="${KUBERNETES_CERT_DIR:-/etc/kubernetes/certs}"
KUBELET_PKI_DIR="${KUBELET_PKI_DIR:-/var/lib/kubelet/pki}"
CLIENT_PEM="${CLIENT_PEM:-$KUBERNETES_CERT_DIR/client.pem}"
KUBELET_CLIENT_CURRENT_PEM="${KUBELET_CLIENT_CURRENT_PEM:-$KUBELET_PKI_DIR/kubelet-client-current.pem}"

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
        --arg Message     "Completed: $*" \
        --arg EventPid    "0" \
        --arg EventTid    "0" \
        '{Timestamp: $Timestamp, OperationId: $OperationId, Version: $Version, TaskName: $TaskName, EventLevel: $EventLevel, Message: $Message, EventPid: $EventPid, EventTid: $EventTid}'
    )
    echo ${json_string} > ${EVENTS_LOGGING_DIR}${eventsFileName}.json
}

waitForTLSBootstrapping() {
    # ensure kubelet client certificate dirs exist
    mkdir -p "$KUBERNETES_CERT_DIR"
    mkdir -p "$KUBELET_PKI_DIR"

    echo "watching for one of the following kubelet client certificate files to be created: [$CLIENT_PEM $KUBELET_CLIENT_CURRENT_PEM]"
    
    # watch for cert file creation with a 5-minute timeout
    START_TIME=$(date +"%F %T.%3N")
    inotifywait -t $WATCH_TIMEOUT_SECONDS -q -m -r -e create "$KUBERNETES_CERT_DIR" "$KUBELET_PKI_DIR" | while read -r DIR _ FILE; do
        if [ "$FILE" = "client.pem" ] || [ "$FILE" = "kubelet-client-current.pem" ]; then
            END_TIME=$(date +"%F %T.%3N")
            echo "kubelet client certificate created at: $DIR/$FILE"

            # we only create the guest agent event if the certificate was created while we were watching
            createGuestAgentEvent "AKS.Runtime.waitForTLSBootstrapping" "$START_TIME" "$END_TIME"
            exit 0
        fi
    done
}

echo "KUBERNETES_CERT_DIR: $KUBERNETES_CERT_DIR, KUBELET_PKI_DIR: $KUBELET_PKI_DIR, CLIENT_PEM: $CLIENT_PEM, KUBELET_CLIENT_CURRENT_PEM: $KUBELET_CLIENT_CURRENT_PEM"

if [ -f "$CLIENT_PEM" ]; then
    echo "kubelet client certificate already exists: $CLIENT_PEM"
    exit 0
fi

if [ -f "$KUBELET_CLIENT_CURRENT_PEM" ]; then
    echo "kubelet client certificate already exists: $KUBELET_CLIENT_CURRENT_PEM"
    exit 0
fi

waitForTLSBootstrapping