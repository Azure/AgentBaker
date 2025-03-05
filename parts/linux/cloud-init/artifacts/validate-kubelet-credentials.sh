#!/bin/bash
set -euo pipefail

# this gives us logs_to_events and retry wrappers
source /opt/azure/containers/provision_source.sh 

KUBECONFIG_PATH="${KUBECONFIG_PATH:-/var/lib/kubelet/kubeconfig}"
BOOTSTRAP_KUBECONFIG_PATH="${BOOTSTRAP_KUBECONFIG_PATH:-/var/lib/kubelet/bootstrap-kubeconfig}"

VALIDATE_KUBELET_CREDENTIALS_MAX_RETRIES=${VALIDATE_KUBELET_CREDENTIALS_MAX_RETRIES:-10}
VALIDATE_KUBELET_CREDENTIALS_RETRY_DELAY_SECONDS=${VALIDATE_KUBELET_CREDENTIALS_RETRY_DELAY_SECONDS:-3}
VALIDATE_KUBELET_CREDENTIALS_RETRY_TIMEOUT_SECONDS=${VALIDATE_KUBELET_CREDENTIALS_RETRY_TIMEOUT_SECONDS:-5}

function validateKubeconfig {
    local kubeconfig_path=$1

    command="kubectl auth whoami --kubeconfig $kubeconfig_path >/dev/null"
    if ! grep -i "whoami" <<< "$(kubectl auth 2>&1)" >/dev/null 2>&1; then
        echo "kubectl auth whoami is not supported, will use can-i create certificatesigningrequests instead"
        command="kubectl auth can-i create certificatesigningrequests --kubeconfig $kubeconfig_path >/dev/null"
    fi

    if ! retrycmd_if_failure $VALIDATE_KUBELET_CREDENTIALS_MAX_RETRIES \
        $VALIDATE_KUBELET_CREDENTIALS_RETRY_DELAY_SECONDS \
        $VALIDATE_KUBELET_CREDENTIALS_RETRY_TIMEOUT_SECONDS $command; then

        # for now we simply exit 0 here to prevent provisioning failures in cases where the credential
        # doesn't become valid until after we've exhausted our retries - kubelet should still eventually be able to register
        echo "kubelet credential validation failed, will still attempt to start kubelet"
        exit 0
    fi
}

function validateKubeletCredentials {
    if [ ! -f "$KUBECONFIG_PATH" ] && [ ! -f "$BOOTSTRAP_KUBECONFIG_PATH" ]; then
        echo "both kubeconfig: $KUBECONFIG_PATH and bootstrap-kubeconfig: $BOOTSTRAP_KUBECONFIG_PATH do not exist, unable to start kubelet"
        exit 1
    fi

    if ! which kubectl >/dev/null 2>&1; then
        echo "kubectl not found, will skip kubelet credential validation"
        exit 0
    fi

    if [ -f "$KUBECONFIG_PATH" ]; then
        echo "will validate kubeconfig: $KUBECONFIG_PATH"
        validateKubeconfig "$KUBECONFIG_PATH"
        echo "kubelet client credential is valid"
        exit 0
    fi

    echo "will validate bootstrap-kubeconfig: $BOOTSTRAP_KUBECONFIG_PATH"
    validateKubeconfig "$BOOTSTRAP_KUBECONFIG_PATH"
    echo "kubelet bootstrap token credential is valid"
}

logs_to_events "AKS.Runtime.validateKubeletCredentials" validateKubeletCredentials