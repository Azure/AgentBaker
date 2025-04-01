#!/bin/bash
set -uo pipefail

EVENTS_LOGGING_DIR=/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/

KUBECONFIG_PATH="${KUBECONFIG_PATH:-/var/lib/kubelet/kubeconfig}"
BOOTSTRAP_KUBECONFIG_PATH="${BOOTSTRAP_KUBECONFIG_PATH:-/var/lib/kubelet/bootstrap-kubeconfig}"

MAX_RETRIES=${CREDENTIAL_VALIDATION_MAX_RETRIES:-30}
RETRY_DELAY_SECONDS=${CREDENTIAL_VALIDATION_RETRY_DELAY_SECONDS:-2}
RETRY_TIMEOUT_SECONDS=${CREDENTIAL_VALIDATION_RETRY_TIMEOUT_SECONDS:-5}

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

<<<<<<< HEAD
<<<<<<< HEAD
validate() {
=======
=======
>>>>>>> 9656979da8 (latest test data)
=======
>>>>>>> d125047ba1 (latest test data)
function validateBootstrapKubeconfig {
>>>>>>> bf6b8e97dc (latest test data)
    local kubeconfig_path=$1
    
    token=$(grep -Po "(?<=token: ).*$" < "$kubeconfig_path")
    token="${token//\"/}"

    echo "will check credential validity against apiserver url: $CREDENTIAL_VALIDATION_APISERVER_URL"

    local retry_count=0
    while true; do
        code=$(curl -sL \
            -m $RETRY_TIMEOUT_SECONDS \
            -o /dev/null \
            -w "%{http_code}" \
            -H "Accept: application/json, */*" \
            -H "Authorization: Bearer $token" \
            --cacert "$CREDENTIAL_VALIDATION_KUBE_CA_FILE" \
            "${CREDENTIAL_VALIDATION_APISERVER_URL}/version?timeout=${RETRY_TIMEOUT_SECONDS}s")

        curl_code=$?

        if [ $code -ge 200 ] && [ $code -lt 400 ]; then
            echo "(retry=$retry_count) received valid HTTP status code from apiserver: $code"
            echo "kubelet bootstrap token credential is valid"
            break
        fi

        if [ $code -eq 000 ]; then
            echo "(retry=$retry_count) curl response code is $code, curl exited with code: $curl_code"
            echo "retrying once more to get a more detailed error response..."

            if ! curl -L \
                -m $RETRY_TIMEOUT_SECONDS \
                -H "Accept: application/json, */*" \
                -H "Authorization: Bearer $token" \
                --cacert "$CREDENTIAL_VALIDATION_KUBE_CA_FILE" \
                "${CREDENTIAL_VALIDATION_APISERVER_URL}/version?timeout=${RETRY_TIMEOUT_SECONDS}s"; then
                echo "curl exited with code: $?"
            fi

            echo "proceeding to start kubelet..."
            return 0
        fi

        echo "(retry=$retry_count) received invalid HTTP status code from apiserver: $code"

        retry_count=$(( $retry_count + 1 ))
        if [ $retry_count -eq $MAX_RETRIES ]; then
            echo "unable to validate bootstrap credentials after $retry_count attempts"
            echo "proceeding to start kubelet..."
            return 0
        fi

        sleep $RETRY_DELAY_SECONDS
    done
}

validateKubeletCredentials() {
    if [ -z "${CREDENTIAL_VALIDATION_KUBE_CA_FILE:-}" ]; then
        echo "CREDENTIAL_VALIDATION_KUBE_CA_FILE is not set, skipping kubelet credential validation"
        return 0
    fi
    if [ -z "${CREDENTIAL_VALIDATION_APISERVER_URL:-}" ]; then
        echo "CREDENTIAL_VALIDATION_APISERVER_URL is not set, skipping kubelet credential validation"
        return 0
    fi
    if [ -z "${BOOTSTRAP_KUBECONFIG_PATH:-}" ]; then
        echo "BOOTSTRAP_KUBECONFIG_PATH is not set, skipping kubelet credential validation"
        return 0
    fi
    if [ ! -f "$BOOTSTRAP_KUBECONFIG_PATH" ]; then
        echo "no bootstrap-kubeconfig found at $BOOTSTRAP_KUBECONFIG_PATH, no bootstrap credentials to validate"
        return 0
    fi
    if [ -n "$KUBECONFIG_PATH" ] && [ -f "$KUBECONFIG_PATH" ]; then
        echo "client credential already exists within kubeconfig: $KUBECONFIG_PATH, no need to validate bootstrap credentials"
        return 0
    fi
    if ! command -v curl >/dev/null 2>&1; then
        echo "curl is not available, unable to validate bootstrap credentials"
        return 0
    fi

    echo "will validate kubelet bootstrap credentials"
    validate "$BOOTSTRAP_KUBECONFIG_PATH"
}

${__SOURCED__:+return}

logs_to_events "AKS.Runtime.validateKubeletCredentials" validateKubeletCredentials