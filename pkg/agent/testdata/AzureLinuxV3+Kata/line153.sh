#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail
set -x

#

MOUNT_POINT="/mnt/aks"

KUBELET_MOUNT_POINT="${MOUNT_POINT}/kubelet"
KUBELET_DIR="/var/lib/kubelet"

mkdir -p "${MOUNT_POINT}"

SENTINEL_FILE="/opt/azure/containers/bind-sentinel"
if [ ! -e "$SENTINEL_FILE" ]; then
    mv "$KUBELET_DIR" "$MOUNT_POINT"
    touch "$SENTINEL_FILE"
fi

mkdir -p "${KUBELET_DIR}"
mount --bind "${KUBELET_MOUNT_POINT}" "${KUBELET_DIR}" 
chmod a+w "${KUBELET_DIR}"