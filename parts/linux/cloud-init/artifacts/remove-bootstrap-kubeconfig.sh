#!/bin/bash
set -uo pipefail

EVENTS_LOGGING_DIR=/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/

KUBECONFIG_PATH="${KUBECONFIG_PATH:-/var/lib/kubelet/kubeconfig}"
BOOTSTRAP_KUBECONFIG_PATH="${BOOTSTRAP_KUBECONFIG_PATH:-/var/lib/kubelet/bootstrap-kubeconfig}"

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
        return 0
    fi

    # wait until kubelet has a valid kubeconfig (TLS bootstrapping is completed at that point)
    while [ ! -f "$KUBECONFIG_PATH" ]; do
        sleep 1
    done

    rm -f "$BOOTSTRAP_KUBECONFIG_PATH"
}

${__SOURCED__:+return}

logs_to_events "AKS.Runtime.removeBootstrapKubeconfig" removeBootstrapKubeconfig
