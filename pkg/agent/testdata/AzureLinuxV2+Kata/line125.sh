#!/usr/bin/env bash

set -o nounset
set -e

source /opt/azure/containers/provision_source_distro.sh

KUBECTL="/usr/local/bin/kubectl --kubeconfig /var/lib/kubelet/kubeconfig"

n=0
while [ ! -f /var/lib/kubelet/kubeconfig ]; do
    echo 'Waiting for TLS bootstrapping'
    if [[ $n -lt 100 ]]; then
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

node_name=$(echo "$node_name" | tr '[:upper:]' '[:lower:]')

golden_timestamp=$($KUBECTL get node ${node_name} -o jsonpath="{.metadata.annotations['kubernetes\.azure\.com/live-patching-golden-timestamp']}")
if [ -z "${golden_timestamp}" ]; then
    echo "golden timestamp is not set, skip live patching"
    exit 0
fi
echo "golden timestamp is: ${golden_timestamp}"

current_timestamp=$($KUBECTL get node ${node_name} -o jsonpath="{.metadata.annotations['kubernetes\.azure\.com/live-patching-current-timestamp']}")
if [ -n "${current_timestamp}" ]; then
    echo "current timestamp is: ${current_timestamp}"

    if [[ "${golden_timestamp}" == "${current_timestamp}" ]]; then
        echo "golden and current timestamp is the same, nothing to patch"
        exit 0
    fi
fi

live_patching_repo_service=$($KUBECTL get node ${node_name} -o jsonpath="{.metadata.annotations['kubernetes\.azure\.com/live-patching-repo-service']}")
private_ip_regex="^((10\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3})|(172\.(1[6-9]|2[0-9]|3[01])\.[0-9]{1,3}\.[0-9]{1,3})|(192\.168\.[0-9]{1,3}\.[0-9]{1,3}))$"
if [[ -n "${live_patching_repo_service}" && ! "${live_patching_repo_service}" =~ $private_ip_regex ]]; then
    echo "Ignore invalid live patching repo service: ${live_patching_repo_service}"
    live_patching_repo_service=""
fi
for repo in mariner-official-base.repo \
            mariner-microsoft.repo \
            mariner-extras.repo \
            mariner-nvidia.repo \
            azurelinux-official-base.repo \
            azurelinux-ms-non-oss.repo \
            azurelinux-ms-oss.repo \
            azurelinux-nvidia.repo; do
    repo_path="/etc/yum.repos.d/${repo}"
    if [ -f ${repo_path} ]; then
        old_repo=$(cat ${repo_path})
        if [ -z "${live_patching_repo_service}" ]; then
            echo "live patching repo service is not set, use PMC repo"
            sed -i 's/http:\/\/[0-9]\+.[0-9]\+.[0-9]\+.[0-9]\+/https:\/\/packages.microsoft.com/g' ${repo_path}
        else
            echo "live patching repo service is: ${live_patching_repo_service}, use it to replace PMC repo" 
            sed -i 's/https:\/\/packages.microsoft.com/http:\/\/'"${live_patching_repo_service}"'/g' ${repo_path}
        fi
        new_repo=$(cat ${repo_path})
        if [[ "${old_repo}" != "${new_repo}" ]]; then
            echo "${repo_path} is updated"
        fi
    fi
done

if ! dnf_update; then
    echo "dnf_update failed"
    exit 1
fi

$KUBECTL annotate --overwrite node ${node_name} kubernetes.azure.com/live-patching-current-timestamp=${golden_timestamp}

echo "package update completed successfully"
