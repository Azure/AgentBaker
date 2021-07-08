#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail
set -x

# Bind mount kubelet to ephemeral storage on startup, as necessary.
#
# This fixes an issue with kubelet's ability to detect allocatable
# capacity for Node ephemeral-storage. On Azure, ephemeral-storage
# should correspond to the temp disk if a VM has one. This script makes
# that true by bind mounting the temp disk to /var/lib/kubelet, so
# kubelet thinks it's located on the temp disk (/dev/sdb). This results
# in correct calculation of ephemeral-storage capacity.

{{if eq GetKubeletDiskType "Temporary"}}
MOUNT_POINT="/mnt/aks"
{{end}}

KUBELET_MOUNT_POINT="${MOUNT_POINT}/kubelet"
KUBELET_DIR="/var/lib/kubelet/pods"

mkdir -p "${MOUNT_POINT}"

# only move the kubelet directory to alternate location on first boot.
SENTINEL_FILE="/opt/azure/containers/bind-sentinel"
if [ ! -e "$SENTINEL_FILE" ]; then
    mv "$KUBELET_DIR" "$MOUNT_POINT"
    touch "$SENTINEL_FILE"
fi

# on every boot, bind mount the kubelet directory back to the expected
# location before kubelet itself may start.
mkdir -p "${KUBELET_DIR}"
mount --bind "${KUBELET_MOUNT_POINT}" "${KUBELET_DIR}" 
chmod a+w "${KUBELET_DIR}"
