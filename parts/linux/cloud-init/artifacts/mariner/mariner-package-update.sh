#!/usr/bin/env bash

set -o nounset
set -e

# source dnf_update
source /opt/azure/containers/provision_source_distro.sh

KUBECTL="/usr/local/bin/kubectl --kubeconfig /var/lib/kubelet/kubeconfig"

# At startup, we need to wait for kubelet to finish TLS bootstrapping to create the kubeconfig file.
n=0
while [ ! -f /var/lib/kubelet/kubeconfig ]; do
    echo 'Waiting for TLS bootstrapping'
    if [ "$n" -lt 100 ]; then
        n=$((n+1))
        sleep 3
    else
        echo "timeout waiting for kubeconfig to be present"
        exit 1
    fi
done

node_name=$(hostname)
if [ -z "${node_name}" ]; then
    echo "cannot get node name"
    exit 1
fi

# Azure cloud provider assigns node name as the lowner case of the hostname
node_name=$(echo "$node_name" | tr '[:upper:]' '[:lower:]')

# retrieve golden timestamp from node annotation
golden_timestamp=$($KUBECTL get node ${node_name} -o jsonpath="{.metadata.annotations['kubernetes\.azure\.com/live-patching-golden-timestamp']}")
if [ -z "${golden_timestamp}" ]; then
    echo "golden timestamp is not set, skip live patching"
    exit 0
fi
echo "golden timestamp is: ${golden_timestamp}"

current_timestamp=$($KUBECTL get node ${node_name} -o jsonpath="{.metadata.annotations['kubernetes\.azure\.com/live-patching-current-timestamp']}")
if [ -n "${current_timestamp}" ]; then
    echo "current timestamp is: ${current_timestamp}"

    if [ "${golden_timestamp}" = "${current_timestamp}" ]; then
        echo "golden and current timestamp is the same, nothing to patch"
        exit 0
    fi
fi

if ! dnf_update; then
    echo "dnf_update failed"
    exit 1
fi

# update current timestamp
$KUBECTL annotate --overwrite node ${node_name} kubernetes.azure.com/live-patching-current-timestamp=${golden_timestamp}

echo "package update completed successfully"
