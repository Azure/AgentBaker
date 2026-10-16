#!/bin/bash
set -uo pipefail

EVENTS_LOGGING_DIR=/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/
BOOTSTRAP_KUBECONFIG_PATH="${BOOTSTRAP_KUBECONFIG_PATH:-/var/lib/kubelet/bootstrap-kubeconfig}"
KUBELET_CLIENT_CURRENT_PEM_PATH="${KUBELET_CLIENT_CURRENT_PEM_PATH:-/var/lib/kubelet/pki/kubelet-client-current.pem}"

logs_to_events() {
    local task=$1; shift
    local eventsFileName=$(date +%s%3N)

    local startTime=$(date +"%F %T.%3N")
    ${@}
    ret=$?
    local endTime=$(date +"%F %T.%3N")

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

    if [ "$ret" -ne 0 ]; then
      return $ret
    fi
}

removeBootstrapKubeconfig() {
    if [ ! -f "$BOOTSTRAP_KUBECONFIG_PATH" ]; then
        echo "$BOOTSTRAP_KUBECONFIG_PATH does not exist, nothing to remove"
        return 0
    fi

    # wait until kubelet-client-current.pem is a symlink to an existing file, indicating kubelet has safely finished TLS bootstrapping
    # and no longer needs the bootstrap credentials present within bootstrap-kubeconfig
    while [ ! -L "$KUBELET_CLIENT_CURRENT_PEM_PATH" ] || [ ! -f "$KUBELET_CLIENT_CURRENT_PEM_PATH" ]; do
        echo "$KUBELET_CLIENT_CURRENT_PEM_PATH does not yet exist as a file or valid symlink, waiting another 1s before attempting to remove $BOOTSTRAP_KUBECONFIG_PATH..."
        sleep 1
    done

    echo "$KUBELET_CLIENT_CURRENT_PEM_PATH exists, removing $BOOTSTRAP_KUBECONFIG_PATH..."
    rm -f "$BOOTSTRAP_KUBECONFIG_PATH"
}

${__SOURCED__:+return}

logs_to_events "AKS.Runtime.removeBootstrapKubeconfig" removeBootstrapKubeconfig
