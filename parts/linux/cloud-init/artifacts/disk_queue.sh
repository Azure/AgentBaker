#!/bin/bash

# SCSI systems will always have a "root" device link.
# NVMe systems will always have an "os" device link.
# In cases where both a "root" and "os" device link exist, they should always point to the same device.
# details: https://learn.microsoft.com/en-us/azure/virtual-machines/linux/azure-virtual-machine-utilities
if [ -L "/dev/disk/azure/root" ]; then
    LINK_PATH="/dev/disk/azure/root"
elif [ -L "/dev/disk/azure/os" ]; then
    LINK_PATH="/dev/disk/azure/os"
else
    echo "no root or os device link found within /dev/disk/azure, cannot apply disk tuning"
    exit 1
fi

echo "found device link: $LINK_PATH"
DEV_NAME=$(basename "$(readlink -f "$LINK_PATH")")
echo "resolved root device: $DEV_NAME"

if [[ "${DEV_NAME,,}" == *"nvme"* ]]; then
    # Disk tuning doesn't currently work as expected on NVMe devices - namely that the /device/queue_depth parameter
    # doesn't seem to be a settable option, and that the default /queue/nr_requests can actually be higher than what we
    # currently set on SCSI (128), which could end up hurting IO performance rather than optimize it.
    # TODO: reach out to NVMe team to see how we can better tune queue settings on NVMe devices.
    echo "$DEV_NAME is an NVMe device, will not apply disk tuning"
    exit 0
fi

if [ ! -d "/sys/block/$DEV_NAME/queue" ]; then
    echo "queue settings directory for device: $DEV_NAME does not exist, unable to apply desired settings"
    exit 1
fi

if [ ! -d "/sys/block/$DEV_NAME/device" ]; then
    echo "device settings directory for device: $DEV_NAME does not exist, unable to apply desired settings"
    exit 1
fi

echo "will apply settings to /sys/block/$DEV_NAME/queue/nr_requests and /sys/block/$DEV_NAME/device/queue_depth"
echo 128 > "/sys/block/$DEV_NAME/queue/nr_requests"
echo 128 > "/sys/block/$DEV_NAME/device/queue_depth"
