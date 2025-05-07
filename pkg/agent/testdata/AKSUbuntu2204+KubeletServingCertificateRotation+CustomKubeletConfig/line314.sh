#!/bin/bash
set -uo pipefail

EVENTS_LOGGING_DIR=/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/

KUBECONFIG_PATH="${KUBECONFIG_PATH:-/var/lib/kubelet/kubeconfig}"
BOOTSTRAP_KUBECONFIG_PATH="${BOOTSTRAP_KUBECONFIG_PATH:-/var/lib/kubelet/bootstrap-kubeconfig}"

MAX_RETRIES=${VALIDATE_KUBELET_CREDENTIALS_MAX_RETRIES:-30}
RETRY_DELAY_SECONDS=${VALIDATE_KUBELET_CREDENTIALS_RETRY_DELAY_SECONDS:-2}
RETRY_TIMEOUT_SECONDS=${VALIDATE_KUBELET_CREDENTIALS_RETRY_TIMEOUT_SECONDS:-5}

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

validateBootstrapKubeconfig() {
    local kubeconfig_path=$1

    cacert=$(grep -Po "(?<=certificate-authority: ).*$" < "$kubeconfig_path")
    apiserver_url=$(grep -Po "(?<=server: ).*$" < "$kubeconfig_path")
    bootstrap_token=$(grep -Po "(?<=token: ).*$" < "$kubeconfig_path")

    if [ -z "$cacert" ]; then
        echo "could not read cluster CA file path from $kubeconfig_path, unable to validate bootstrap credentials"
        exit 0
    fi
    if [ -z "$apiserver_url" ]; then
        echo "could not read apiserver URL from $kubeconfig_path, unable to validate bootstrap credentials"
        exit 0
    fi
    if [ -z "$bootstrap_token" ]; then
        echo "could not read bootstrap token from $kubeconfig_path, unable to validate bootstrap credentials"
        exit 0
    fi

    echo "will check credential validity against apiserver url: $apiserver_url"

    local retry_count=0
    while true; do
        code=$(curl -sL \
            -m $RETRY_TIMEOUT_SECONDS \
            -o /dev/null \
            -w "%{http_code}" \
            -H "Accept: application/json, */*" \
            -H "Authorization: Bearer ${bootstrap_token//\"/}" \
            --cacert "$cacert" \
            "${apiserver_url}/version?timeout=${RETRY_TIMEOUT_SECONDS}s")

        curl_code=$?

        if [ $code -ge 200 ] && [ $code -lt 400 ]; then
            echo "(retry=$retry_count) received valid HTTP status code from apiserver: $code"
            break
        fi

        if [ $code -eq 000 ]; then
            echo "(retry=$retry_count) curl response code is $code, curl exited with code: $curl_code"
            echo "retrying once more to get a more detailed error response..."

            curl -L \
                -m $RETRY_TIMEOUT_SECONDS \
                -H "Accept: application/json, */*" \
                -H "Authorization: Bearer ${bootstrap_token//\"/}" \
                --cacert "$cacert" \
                "${apiserver_url}/version?timeout=${RETRY_TIMEOUT_SECONDS}s"

            echo "proceeding to start kubelet..."
            exit 0
        fi

        echo "(retry=$retry_count) received invalid HTTP status code from apiserver: $code"

        retry_count=$(( $retry_count + 1 ))
        if [ $retry_count -eq $MAX_RETRIES ]; then
            echo "unable to validate bootstrap credentials after $retry_count attempts"
            echo "proceeding to start kubelet..."
            exit 0
        fi

        sleep $RETRY_DELAY_SECONDS
    done
}

function validateKubeletCredentials {
    if [ -f "$KUBECONFIG_PATH" ]; then
        echo "client credential already exists within kubeconfig: $KUBECONFIG_PATH, no need to validate bootstrap credentials"
        exit 0
    fi

    if [ ! -f "$BOOTSTRAP_KUBECONFIG_PATH" ]; then
        echo "no bootstrap-kubeconfig found at $BOOTSTRAP_KUBECONFIG_PATH, no bootstrap credentials to validate"
        exit 0
    fi

    if ! which curl >/dev/null 2>&1; then
        echo "curl is not available, unable to validate bootstrap credentials"
        exit 0
    fi

    echo "will validate bootstrap-kubeconfig: $BOOTSTRAP_KUBECONFIG_PATH"
    validateBootstrapKubeconfig "$BOOTSTRAP_KUBECONFIG_PATH"
    echo "kubelet bootstrap token credential is valid"
}

logs_to_events "AKS.Runtime.validateKubeletCredentials" validateKubeletCredentials