#!/usr/bin/env bash

set -o nounset
set -e

# source apt_get_update
source /opt/azure/containers/provision_source_distro.sh

# Execute unattended-upgrade
unattended_upgrade() {
  retries=10
  for i in $(seq 1 $retries); do
    unattended-upgrade -v && break
    if [ $i -eq $retries ]; then
      return 1
    else sleep 5
    fi
  done
  echo Executed unattended upgrade $i times
}

# Determinate is the given option present in the cfg file
cfg_has_option() {
    file=$1
    option=$2
    line=$(sed -n "/^$option:/ p" "$file")
    [ -n "$line" ]
}

# Set an option in a cfg file
cfg_set_option() {
    file=$1
    option=$2
    value=$3
    if ! cfg_has_option "$file" "$option"; then
        echo "$option: $value" >> "$file"
    else
        sed -i 's/'"$option"':.*$/'"$option: $value"'/g' "$file"
    fi
}

KUBECTL="/usr/local/bin/kubectl --kubeconfig /var/lib/kubelet/kubeconfig"

source_list_path=/etc/apt/sources.list
source_list_backup_path=/etc/apt/sources.list.backup
cloud_cfg_path=/etc/cloud/cloud.cfg

# At startup, we need to wait for kubelet to finish TLS bootstrapping to create the kubeconfig file.
while [ ! -f /var/lib/kubelet/kubeconfig ]; do
    echo 'Waiting for TLS bootstrapping'
    sleep 3
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

    if [[ "${golden_timestamp}" == "${current_timestamp}" ]]; then
        echo "golden and current timestamp is the same, nothing to patch"
        exit 0
    fi
fi

old_source_list=$(cat ${source_list_path})

# Network isolated cluster can't access the internet, so we deploy a live patching repo service in the cluster
# The node will use the live patching repo service to download the repo metadata and packages
# If the annotation is not set, we will use the ubuntu snapshot repo
live_patching_repo_service=$($KUBECTL get node ${node_name} -o jsonpath="{.metadata.annotations['kubernetes\.azure\.com/live-patching-repo-service']}")
# Limit the live patching repo service to private IPs in the range of 10.x.x.x, 172.16.x.x - 172.31.x.x, and 192.168.x.x
private_ip_regex="^((10\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3})|(172\.(1[6-9]|2[0-9]|3[01])\.[0-9]{1,3}\.[0-9]{1,3})|(192\.168\.[0-9]{1,3}\.[0-9]{1,3}))$"
if [[ -n "${live_patching_repo_service}" && ! "${live_patching_repo_service}" =~ $private_ip_regex ]]; then
    echo "Ignore invalid live patching repo service: ${live_patching_repo_service}"
    live_patching_repo_service=""
fi
if [ -z "${live_patching_repo_service}" ]; then
    echo "live patching repo service is not set, use ubuntu snapshot repo"
    # upgrade from base image to a timestamp
    # e.g. replace https://azure.archive.ubuntu.com/ubuntu/ with https://snapshot.ubuntu.com/ubuntu/20230727T000000Z
    sed -i 's/http:\/\/azure.archive.ubuntu.com\/ubuntu\//https:\/\/snapshot.ubuntu.com\/ubuntu\/'"${golden_timestamp}"'/g' ${source_list_path}
    # upgrade from one timestamp to another timestamp
    # e.g. replace https://snapshot.ubuntu.com/ubuntu/20250310T000000Z with https://snapshot.ubuntu.com/ubuntu/20250318T000000Z
    sed -i 's/https:\/\/snapshot.ubuntu.com\/ubuntu\/\([0-9]\{8\}T[0-9]\{6\}Z\)/https:\/\/snapshot.ubuntu.com\/ubuntu\/'"${golden_timestamp}"'/g' ${source_list_path}
    # No live patching repo service annotation, so we need to change to use the ubuntu snapshot repo
    # e.g. replace http://10.224.0.5/ubuntu/ with https://snapshot.ubuntu.com/ubuntu/20250318T000000Z
    sed -i 's/http:\/\/[0-9]\+.[0-9]\+.[0-9]\+.[0-9]\+\/ubuntu\//https:\/\/snapshot.ubuntu.com\/ubuntu\/'"${golden_timestamp}"'/g' ${source_list_path}
else
    echo "live patching repo service is: ${live_patching_repo_service}"
    # upgrade from base image to live patching repo service
    # e.g. replace https://azure.archive.ubuntu.com/ubuntu/ with http://10.224.0.5/ubuntu/
    sed -i 's/http:\/\/azure.archive.ubuntu.com\/ubuntu\//http:\/\/'"${live_patching_repo_service}"'\/ubuntu\//g' ${source_list_path}
    # upgrade from one ubuntu repo timestamp to live patching repo service
    # e.g. replace https://snapshot.ubuntu.com/ubuntu/20250310T000000Z with http://10.224.0.5/ubuntu/
    sed -i 's/https:\/\/snapshot.ubuntu.com\/ubuntu\/\([0-9]\{8\}T[0-9]\{6\}Z\)/http:\/\/'"${live_patching_repo_service}"'\/ubuntu\//g' ${source_list_path}
    # upgrade the old live patching repo service to the new one
    # e.g. replace http://10.224.0.5/ubuntu/ with http://10.224.0.6/ubuntu/
    sed -i 's/http:\/\/[0-9]\+.[0-9]\+.[0-9]\+.[0-9]\+\/ubuntu\//http:\/\/'"${live_patching_repo_service}"'\/ubuntu\//g' ${source_list_path}
fi

# preserve the sources.list changes
option=apt_preserve_sources_list
option_value=true
cfg_set_option ${cloud_cfg_path} ${option} ${option_value}

new_source_list=$(cat ${source_list_path})
if [[ "${old_source_list}" != "${new_source_list}" ]]; then
    # save old sources.list
    echo "$old_source_list" > ${source_list_backup_path}
    echo "/etc/apt/sources.list is updated:"
    diff ${source_list_backup_path} ${source_list_path} || true
fi

if ! apt_get_update; then
    echo "apt_get_update failed"
    exit 1
fi
if ! unattended_upgrade; then
    echo "unattended_upgrade failed"
    exit 1
fi

# update current timestamp
$KUBECTL annotate --overwrite node ${node_name} kubernetes.azure.com/live-patching-current-timestamp=${golden_timestamp}

echo snapshot update completed successfully
