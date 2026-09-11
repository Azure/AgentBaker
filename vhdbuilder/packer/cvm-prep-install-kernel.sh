#!/bin/bash

set -euo pipefail

ORIGINAL_KERNEL_MARKER="${ORIGINAL_KERNEL_MARKER:-/opt/azure/cvm-prep-original-kernel}"

waitForAptLocks() {
    while fuser /var/lib/dpkg/lock /var/lib/apt/lists/lock /var/cache/apt/archives/lock /var/lib/dpkg/lock-frontend >/dev/null 2>&1; do
        echo "Waiting for release of apt locks"
        sleep 3
    done
}

isNullbootInstalled() {
    dpkg-query -W -f='${Status}' nullboot 2>/dev/null | grep -q "install ok installed"
}

failIfNullbootPresent() {
    local context="$1"
    if isNullbootInstalled; then
        echo "ERROR: nullboot is installed (${context})." >&2
        echo "       The CVM prep image intentionally uses a GRUB-managed boot chain," >&2
        echo "       not nullboot's UKI/systemd-boot trust chain. Investigate why nullboot" >&2
        echo "       was pulled in (likely a kernel metapackage Depends/Recommends change)" >&2
        echo "       before re-running the prep build." >&2
        exit 1
    fi
    echo "nullboot check (${context}): not installed, continuing"
}

getUbuntuRelease() {
    (. /etc/os-release 2>/dev/null && echo "${VERSION_ID:-}")
}

buildFdeKernelPackageList() {
    local ubuntu_release="$1"
    local modules_extra_pkg="linux-modules-extra-azure-lts-${ubuntu_release}"

    printf '%s\n' \
        "linux-image-azure-fde-lts-${ubuntu_release}" \
        "linux-tools-azure-lts-${ubuntu_release}" \
        "linux-cloud-tools-azure-lts-${ubuntu_release}" \
        "linux-headers-azure-lts-${ubuntu_release}"

    if apt-cache show "${modules_extra_pkg}" &>/dev/null; then
        printf '%s\n' "${modules_extra_pkg}"
    else
        echo "Package ${modules_extra_pkg} not available - skipping" >&2
    fi
}

configureGrubForNewKernel() {
    if ! command -v update-grub &>/dev/null; then
        echo "ERROR: update-grub not found; the CVM prep base image is expected to use GRUB" >&2
        exit 1
    fi

    if [ -f /etc/default/grub ]; then
        if grep -q '^GRUB_DEFAULT=' /etc/default/grub; then
            sed -i 's/^GRUB_DEFAULT=.*/GRUB_DEFAULT=0/' /etc/default/grub
        else
            echo 'GRUB_DEFAULT=0' >> /etc/default/grub
        fi
    fi

    update-grub

    if [ ! -f /boot/grub/grub.cfg ] || ! grep -q "azure-fde" /boot/grub/grub.cfg; then
        echo "ERROR: /boot/grub/grub.cfg does not reference an azure-fde kernel entry after update-grub" >&2
        exit 1
    fi
    echo "GRUB configured; azure-fde kernel entry present in /boot/grub/grub.cfg"
}

main() {
    local ubuntu_release
    local -a kernel_packages=()
    local pkg

    ubuntu_release="$(getUbuntuRelease)"
    if [ -z "${ubuntu_release}" ]; then
        echo "ERROR: unable to determine Ubuntu release from /etc/os-release" >&2
        exit 1
    fi

    mkdir -p "$(dirname "${ORIGINAL_KERNEL_MARKER}")"
    uname -r > "${ORIGINAL_KERNEL_MARKER}"
    echo "Recorded original kernel: $(cat "${ORIGINAL_KERNEL_MARKER}")"

    failIfNullbootPresent "before kernel install"

    while IFS= read -r pkg; do
        [ -n "${pkg}" ] && kernel_packages+=("${pkg}")
    done < <(buildFdeKernelPackageList "${ubuntu_release}")

    echo "Installing azure-fde kernel packages for Ubuntu ${ubuntu_release}: ${kernel_packages[*]}"
    waitForAptLocks
    DEBIAN_FRONTEND=noninteractive apt-get update
    waitForAptLocks
    DEBIAN_FRONTEND=noninteractive apt-get install --no-install-recommends -y "${kernel_packages[@]}"

    failIfNullbootPresent "after kernel install"

    configureGrubForNewKernel

    echo "cvm-prep-install-kernel.sh finished successfully; rebooting to verify azure-fde kernel boots"
}

if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
    main "$@"
fi
