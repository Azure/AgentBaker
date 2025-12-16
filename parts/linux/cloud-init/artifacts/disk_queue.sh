#!/bin/bash

# the "/dev/disk/azure/os" link is added via the azure-vm-utils package for NVMe disks
# and is backwards-compatible with SCSI disks: https://learn.microsoft.com/en-us/azure/virtual-machines/linux/azure-virtual-machine-utilities
ROOT_DEV=$(readlink -f /dev/disk/azure/os)
DEV_NAME=$(basename "$ROOT_DEV")

echo "resolved root device: $DEV_NAME, will apply settings to /sys/block/$DEV_NAME/queue/nr_requests"

if [ ! -d "/sys/block/$DEV_NAME/queue" ]; then
    echo "queue settings directory for device: $DEV_NAME does not exist, unable to apply desired settings"
    exit 1
fi

# 128 is the default queue depth, so this is effectively enforcing the default value...
echo 128 > "/sys/block/$DEV_NAME/queue/nr_requests"
