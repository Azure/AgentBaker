#!/usr/bin/env bash

set -o nounset
set -e

source /opt/azure/containers/provision_source_distro.sh

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

cfg_has_option() {
    file=$1
    option=$2
    line=$(sed -n "/^$option:/ p" "$file")
    [ -n "$line" ]
}

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

while [ ! -f /var/lib/kubelet/kubeconfig ]; do
    echo 'Waiting for TLS bootstrapping'
    sleep 3
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

old_source_list=$(cat ${source_list_path})
sed -i 's/http:\/\/azure.archive.ubuntu.com\/ubuntu\//https:\/\/snapshot.ubuntu.com\/ubuntu\/'"${golden_timestamp}"'/g' ${source_list_path}
sed -i 's/https:\/\/snapshot.ubuntu.com\/ubuntu\/\([0-9]\{8\}T[0-9]\{6\}Z\)/https:\/\/snapshot.ubuntu.com\/ubuntu\/'"${golden_timestamp}"'/g' ${source_list_path}
option=apt_preserve_sources_list
option_value=true
cfg_set_option ${cloud_cfg_path} ${option} ${option_value}

new_source_list=$(cat ${source_list_path})
if [[ "${old_source_list}" != "${new_source_list}" ]]; then
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

$KUBECTL annotate --overwrite node ${node_name} kubernetes.azure.com/live-patching-current-timestamp=${golden_timestamp}

echo snapshot update completed successfully
