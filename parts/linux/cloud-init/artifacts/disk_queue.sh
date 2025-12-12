#!/bin/bash

# note that "readlink -f /dev/disk/azure/os" also works to resolve the root device,
# though it's unclear whether this link will be present on all VM SKUs
ROOT_DEV=$(findmnt -n -o SOURCE / | sed 's/[0-9]*$//')
DEV_NAME=$(basename "$ROOT_DEV")

echo "resolved root device: $DEV_NAME, will apply settings to /sys/block/$DEV_NAME/queue/{nr_requests, depth}"

# 128 is the default queue depth, so this is effectively enforcing the default value...
echo 128 > "/sys/block/$DEV_NAME/queue/nr_requests"
