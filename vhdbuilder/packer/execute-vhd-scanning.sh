#!/usr/bin/env bash
set -euxo pipefail

mkdir -p /mnt/sda1
mount /dev/sda1 /mnt/sda1

/home/azureuser/packer/trivy-scan.sh

umount /mnt/sda1
rmdir /mnt/sda1