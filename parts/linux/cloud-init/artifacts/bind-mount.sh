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

# if aks ever supports alternatives besides temp disk
# this mount point will need to be updated
MOUNT_POINT="/mnt/aks"

KUBELET_MOUNT_POINT="${MOUNT_POINT}/kubelet"
KUBELET_DIR="/var/lib/kubelet"
SENTINEL_FILE="/opt/azure/containers/bind-sentinel"

prepareKubeletDisk() {
    mkdir -p "${MOUNT_POINT}"

    # A reimage replaces the OS disk and its sentinel, but can retain kubelet
    # state on the temporary disk. Reset that stale state before migrating the
    # fresh OS-disk kubelet directory.
    if [ ! -e "$SENTINEL_FILE" ]; then
        rm -rf --one-file-system "${KUBELET_MOUNT_POINT}"
        mv "${KUBELET_DIR}" "${KUBELET_MOUNT_POINT}"
        touch "${SENTINEL_FILE}"
    fi

    # On every boot, bind mount the kubelet directory back to the expected
    # location before kubelet itself may start.
    mkdir -p "${KUBELET_DIR}"
    mount --bind "${KUBELET_MOUNT_POINT}" "${KUBELET_DIR}"
    chmod a+w "${KUBELET_DIR}"
}

# Prevent ShellSpec from executing the production entry point when sourcing.
${__SOURCED__:+return}

prepareKubeletDisk
