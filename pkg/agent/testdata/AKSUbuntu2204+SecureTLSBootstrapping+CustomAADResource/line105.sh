#!/bin/bash

set -euxo pipefail

EVENTS_LOGGING_DIR="/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events"
NEXT_PROTO_VALUE="aks-tls-bootstrap"

RETRY_PERIOD_SECONDS=300 
RETRY_WAIT_SECONDS=3

AAD_RESOURCE="${SECURE_TLS_BOOTSTRAP_AAD_RESOURCE:-""}"
API_SERVER_NAME="${API_SERVER_NAME:-""}"

CLIENT_BINARY_PATH="${SECURE_TLS_BOOTSTRAP_CLIENT_BINARY_PATH:-/opt/azure/tlsbootstrap/tls-bootstrap-client}"
KUBECONFIG_PATH="${SECURE_TLS_BOOTSTRAP_KUBECONFIG_PATH:-/var/lib/kubelet/kubeconfig}"
CLIENT_CERT_PATH="${SECURE_TLS_BOOTSTRAP_CLIENT_CERT_PATH:-/etc/kubernetes/certs/client.crt}"
CLIENT_KEY_PATH="${SECURE_TLS_BOOTSTRAP_CLIENT_KEY_PATH:-/etc/kubernetes/certs/client.key}"
AZURE_CONFIG_PATH="${SECURE_TLS_BOOTSTRAP_ZURE_CONFIG_PATH:-/etc/kubernetes/azure.json}"
CLUSTER_CA_FILE_PATH="${SECURE_TLS_BOOTSTRAP_CLUSTER_CA_FILE_PATH:-/etc/kubernetes/certs/ca.crt}"
LOG_FILE_PATH="${SECURE_TLS_BOOTSTRAP_LOG_FILE_PATH:-/var/log/azure/aks/secure-tls-bootstrap.log}"

logs_to_events() {
    local task=$1; shift
    local eventsFileName=$(date +%s%3N)

    local startTime=$(date +"%F %T.%3N")
    ${@}
    ret=$?
    local endTime=$(date +"%F %T.%3N")

    msg_string=$(jq -n --arg Status "Succeeded" --arg Hostname "$(uname -n)" '{Status: $Status, Hostname: $Hostname}')
    if [ "$ret" != "0" ]; then
        msg_string=$(jq -n --arg Status "Failed" --arg Hostname "$(uname -n)" --arg LogTail "$(tail -n 20 $LOG_FILE_PATH)" '{Status: $Status, Hostname: $Hostname, LogTail: $LogTail}')
    fi

    json_string=$( jq -n \
        --arg Timestamp   "${startTime}" \
        --arg OperationId "${endTime}" \
        --arg Version     "1.23" \
        --arg TaskName    "${task}" \
        --arg EventLevel  "Informational" \
        --arg Message     "${msg_string}" \
        --arg EventPid    "0" \
        --arg EventTid    "0" \
        '{Timestamp: $Timestamp, OperationId: $OperationId, Version: $Version, TaskName: $TaskName, EventLevel: $EventLevel, Message: $Message, EventPid: $EventPid, EventTid: $EventTid}'
    )
    echo ${json_string} > "${EVENTS_LOGGING_DIR}/${eventsFileName}.json"

    if [ "$ret" != "0" ]; then
      return $ret
    fi
}

bootstrap() {
    if [ -z "$API_SERVER_NAME" ]; then
        echo "ERROR: missing apiserver FQDN, cannot continue bootstrapping"
        return 1
    fi
    if [ ! -f "$CLIENT_BINARY_PATH" ]; then
        echo "ERROR: bootstrap client binary does not exist at path $CLIENT_BINARY_PATH"
        return 1
    fi

    chmod +x $CLIENT_BINARY_PATH

    deadline=$(($(date +%s) + RETRY_PERIOD_SECONDS))
    while true; do
        now=$(date +%s)
        if [ $((now - deadline)) -ge 0 ]; then
            echo "ERROR: bootstrapping deadline exceeded"
            return 1
        fi

        $CLIENT_BINARY_PATH bootstrap \
         --aad-resource="$AAD_RESOURCE" \
         --apiserver-fqdn="$API_SERVER_NAME" \
         --cluster-ca-file="$CLUSTER_CA_FILE_PATH" \
         --azure-config="$AZURE_CONFIG_PATH" \
         --cert-file="$CLIENT_CERT_PATH" \
         --key-file="$CLIENT_KEY_PATH" \
         --next-proto="$NEXT_PROTO_VALUE" \
         --kubeconfig="$KUBECONFIG_PATH" \
         --log-file="$LOG_FILE_PATH"

        [ $? -eq 0 ] && break

        sleep $RETRY_WAIT_SECONDS
    done
}

logs_to_events "AKS.performSecureTLSBootstrapping" bootstrap || exit $?

#EOF