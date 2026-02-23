#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail
set -x

# Bind mount kubelet to local NVMe storage specifically on startup.
MOUNT_POINT="/mnt/aks"


KUBELET_MOUNT_POINT="${MOUNT_POINT}/kubelet"
KUBELET_DIR="/var/lib/kubelet"

mkdir -p "${MOUNT_POINT}"

SENTINEL_FILE="/opt/azure/containers/bind-sentinel"
if [ ! -e "${SENTINEL_FILE}" ]; then
    # Bond (via software RAID) and format the NVMe disks if that's not already done.
    if [ -e /dev/disk/azure/local/by-index/1 ] && [ ! -e /dev/md0 ]; then
        mdadm --create --verbose /dev/md0 --level=0 --raid-devices=4 /dev/disk/azure/local/by-index/1 /dev/disk/azure/local/by-index/2 /dev/disk/azure/local/by-index/3 /dev/disk/azure/local/by-index/4
        mkfs.ext4 -F /dev/md0
    fi
    mount /dev/md0 "${MOUNT_POINT}"
    mv "${KUBELET_DIR}" "${KUBELET_MOUNT_POINT}"
    touch "${SENTINEL_FILE}"
else
    # On subsequent boots, the disk should already be partitioned and formatted, so just mount it.
    mount /dev/md0 "${MOUNT_POINT}"
fi

# on every boot, bind mount the kubelet directory back to the expected
# location before kubelet itself may start.
mkdir -p "${KUBELET_DIR}"
mount --bind "${KUBELET_MOUNT_POINT}" "${KUBELET_DIR}"
chmod a+w "${KUBELET_DIR}"
