#!/bin/bash

setup_nvme() {
  RAID_DEVICE="/dev/md0"
  FILESYSTEM_TYPE="xfs"
  MOUNT_POINT="/mnt/nvme-raid"

  ALL_NVME_DEVICES=$(ls /dev/nvme*n* 2>/dev/null)
  UNUSED_DEVICES=()

  if [ -e ${RAID_DEVICE} ]; then
    echo "RAID device already exists, proceeding to bind mount setup"
  else
    if ! command -v mdadm &> /dev/null; then
      apt-get update >/dev/null && apt-get install -y mdadm
    fi

    for device in $ALL_NVME_DEVICES; do
      if mdadm --examine "$device" | grep -q 'RAID superblock'; then
        continue
      fi

      if findmnt -S "$device" -n &> /dev/null; then
        continue
      fi
      if blkid "$device" &> /dev/null; then
        continue
      fi
      UNUSED_DEVICES+=("$device")
    done

    echo ""
    echo "${#UNUSED_DEVICES[@]} devices will be combined into a RAID0 array:"
    for device in "${UNUSED_DEVICES[@]}"; do
      echo "  - $device"
    done
    echo ""

    mdadm --create "$RAID_DEVICE" \
      --level=0 \
      --raid-devices="${#UNUSED_DEVICES[@]}" \
      "${UNUSED_DEVICES[@]}" \
      --run \
      --force

    if [ $? -ne 0 ]; then
      echo "Error: Failed to create RAID array."
      exit 1
    fi

    echo "RAID array created successfully."
    echo "Waiting a few seconds for the device to be ready"
    sleep 5

    echo "Creating ${FILESYSTEM_TYPE} filesystem on ${RAID_DEVICE}"
    mkfs.${FILESYSTEM_TYPE} "${RAID_DEVICE}"

    if [ $? -ne 0 ]; then
      echo "Error: Failed to create filesystem"
      mdadm --stop "$RAID_DEVICE"
      exit 1
    fi

    mdadm --detail --scan | sudo tee -a /etc/mdadm/mdadm.conf
    mkdir -p ${MOUNT_POINT}
    mount ${RAID_DEVICE} ${MOUNT_POINT}
    echo "${RAID_DEVICE}	${MOUNT_POINT}	${FILESYSTEM_TYPE}	defaults,nofail,discard	0	0" >> /etc/fstab
  fi
}

