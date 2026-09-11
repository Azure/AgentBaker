#!/bin/bash

set -euo pipefail

UBUNTU_RELEASE="$(. /etc/os-release && echo "${VERSION_ID}")"
KERNEL_IMAGE="linux-image-azure-fde-lts-${UBUNTU_RELEASE}"
KERNEL_PACKAGES=(
    "${KERNEL_IMAGE}"
    "linux-tools-azure-lts-${UBUNTU_RELEASE}"
    "linux-cloud-tools-azure-lts-${UBUNTU_RELEASE}"
    "linux-headers-azure-lts-${UBUNTU_RELEASE}"
)
MODULES_EXTRA_PKG="linux-modules-extra-azure-lts-${UBUNTU_RELEASE}"

waitForAptLocks() {
    while fuser /var/lib/dpkg/lock /var/lib/apt/lists/lock /var/cache/apt/archives/lock /var/lib/dpkg/lock-frontend >/dev/null 2>&1; do
        echo "Waiting for release of apt locks"
        sleep 3
    done
}

waitForAptLocks
DEBIAN_FRONTEND=noninteractive apt-get update

if ! apt-cache show "${KERNEL_IMAGE}" &>/dev/null; then
    echo "Kernel package ${KERNEL_IMAGE} is not available" >&2
    exit 1
fi

if apt-cache show "${MODULES_EXTRA_PKG}" &>/dev/null; then
    KERNEL_PACKAGES+=("${MODULES_EXTRA_PKG}")
fi

waitForAptLocks
DEBIAN_FRONTEND=noninteractive apt-get remove --purge -y --allow-remove-essential nullboot

waitForAptLocks
DEBIAN_FRONTEND=noninteractive apt-get remove --purge -y $(dpkg-query -W 'linux-*azure*' | awk '$2 != "" { print $1 }' | paste -s)

waitForAptLocks
DEBIAN_FRONTEND=noninteractive apt-get install --no-install-recommends -y "${KERNEL_PACKAGES[@]}"

waitForAptLocks
DEBIAN_FRONTEND=noninteractive apt-get autoremove -y
DEBIAN_FRONTEND=noninteractive apt-get clean

update-grub
