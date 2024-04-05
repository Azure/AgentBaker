#!/bin/bash

set -uxo pipefail

RETRY_PERIOD_SECONDS=180 # 3 minutes
RETRY_WAIT_SECONDS=5

CLIENT_BINARY_PATH="${1:-/opt/azure/tlsbootstrap/tls-bootstrap-client}"
KUBECONFIG_PATH="${2:-/var/lib/kubelet/kubeconfig}"
APISERVER_FQDN="${3:-""}"
AZURE_CONFIG_PATH="${4:-/etc/kubernetes/azure.json}"
CLUSTER_CA_FILE_PATH="${5:-/etc/kubernetes/certs/ca.crt}"
CUSTOM_AAD_RESOURCE="${6:-""}"

bootstrap() {
    if [ -z "$APISERVER_FQDN" ]; then
        echo "ERROR: missing apiserver FQDN, cannot continue bootstrapping"
        exit 1
    fi
    if [ ! -f "$CLIENT_BINARY_PATH" ]; then
        echo "ERROR: bootstrap client binary does not exist at path $CLIENT_BINARY_PATH"
        exit 1
    fi

    AAD_RESOURCE="6dae42f8-4368-4678-94ff-3960e28e3630"
    [ -n "$CUSTOM_AAD_RESOURCE" ] && AAD_RESOURCE="$CUSTOM_AAD_RESOURCE"

    chmod +x "$CLIENT_BINARY_PATH"

    deadline=$(($(date +%s) + RETRY_PERIOD_SECONDS))
    while true; do
        now=$(date +%s)
        if [ $((now - deadline)) -ge 0 ]; then
            echo "ERROR: bootstrapping deadline exceeded"
            exit 1
        fi

        $CLIENT_BINARY_PATH bootstrap \
         --aad-resource="$AAD_RESOURCE" \
         --apiserver-fqdn="${APISERVER_FQDN}:443" \
         --cluster-ca-file="$CLUSTER_CA_FILE_PATH" \
         --azure-config="$AZURE_CONFIG_PATH" \
         --next-proto="aks-tls-bootstrap" \
         --kubeconfig="$KUBECONFIG_PATH"

        [ $? -eq 0 ] && exit 0

        sleep $RETRY_WAIT_SECONDS
    done
}

bootstrap "$@"