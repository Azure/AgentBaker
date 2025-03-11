#!/bin/bash
set -euo pipefail

# this gives us logs_to_events
source /opt/azure/containers/provision_source.sh

KUBECONFIG_PATH="${KUBECONFIG_PATH:-/var/lib/kubelet/kubeconfig}"
BOOTSTRAP_KUBECONFIG_PATH="${BOOTSTRAP_KUBECONFIG_PATH:-/var/lib/kubelet/bootstrap-kubeconfig}"

MAX_RETRIES=${VALIDATE_KUBELET_CREDENTIALS_MAX_RETRIES:-15}
RETRY_DELAY_SECONDS=${VALIDATE_KUBELET_CREDENTIALS_RETRY_DELAY_SECONDS:-2}
RETRY_TIMEOUT_SECONDS=${VALIDATE_KUBELET_CREDENTIALS_RETRY_TIMEOUT_SECONDS:-5}

function validateBootstrapKubeconfig {
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

        if [ $code -ge 200 ] && [ $code -lt 400 ]; then
            echo "(retry=$retry_count) received valid HTTP status code from apiserver: $code"
            break
        fi

        echo "(retry=$retry_count) received invalid HTTP status code from apiserver: $code"

        retry_count=$(( $retry_count + 1 ))
        if [ $retry_count -eq $MAX_RETRIES ]; then
            echo "unable to validate bootstrap credentials after $retry_count attempts"
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