#!/bin/bash
# CVM bootstrap (Stage 1, CVM_BUILD_STAGE=bootstrap) pre-reboot provisioning.
#
# Used only by vhd-image-builder-cvm-bootstrap.json to prepare a minimal,
# bootable "intermediate" Ubuntu 26.04 image capable of running on a real
# ConfidentialVM (SecurityType=ConfidentialVM). It intentionally does NOT run
# the full AgentBaker VHD provisioning chain (pre/post-install-dependencies.sh,
# containerd, k8s binaries, CNI, CSE, etc.) -- that happens entirely in Stage 2
# against the dedicated vhd-image-builder-cvm-2604.json.
#
# Responsibilities (this half, before reboot):
#   1. Record the kernel running before any change, so Stage 1's post-reboot
#      half can later purge it (and only it) once the new kernel is confirmed
#      booted.
#   2. Install the Ubuntu Full-Disk-Encryption ("azure-fde") kernel and its
#      companion packages with --no-install-recommends.
#   3. Fail loudly if 'nullboot' is present (as a dependency or otherwise).
#      nullboot manages a UKI-only (systemd-boot) Secure Boot trust chain that
#      bypasses GRUB entirely. If it slips in, the GRUB configuration below
#      would silently have no effect on the resulting boot chain, so we would
#      rather fail the build than ship an unvalidated hybrid boot config.
#   4. Configure GRUB so the new azure-fde kernel is the default boot entry.
#
# The packer template reboots the VM immediately after this script runs; see
# cvm-bootstrap-verify-and-cleanup.sh for the post-reboot half.

set -euo pipefail

ORIGINAL_KERNEL_MARKER="${ORIGINAL_KERNEL_MARKER:-/opt/azure/cvm-bootstrap-original-kernel}"

# waitForAptLocks blocks until no other process holds an apt/dpkg lock file.
waitForAptLocks() {
    while fuser /var/lib/dpkg/lock /var/lib/apt/lists/lock /var/cache/apt/archives/lock /var/lib/dpkg/lock-frontend >/dev/null 2>&1; do
        echo "Waiting for release of apt locks"
        sleep 3
    done
}

# isNullbootInstalled returns success (0) if the nullboot package is currently
# installed (dpkg status "install ok installed"), failure (1) otherwise.
isNullbootInstalled() {
    dpkg-query -W -f='${Status}' nullboot 2>/dev/null | grep -q "install ok installed"
}

# failIfNullbootPresent exits the build with a clear diagnostic if nullboot is
# installed. $1 is a short label describing when the check ran, used only for
# the error message.
failIfNullbootPresent() {
    local context="$1"
    if isNullbootInstalled; then
        echo "ERROR: nullboot is installed (${context})." >&2
        echo "       The CVM bootstrap image intentionally uses a GRUB-managed boot chain," >&2
        echo "       not nullboot's UKI/systemd-boot trust chain. Investigate why nullboot" >&2
        echo "       was pulled in (likely a kernel metapackage Depends/Recommends change)" >&2
        echo "       before re-running the bootstrap build." >&2
        exit 1
    fi
    echo "nullboot check (${context}): not installed, continuing"
}

# getUbuntuRelease prints the Ubuntu release (VERSION_ID from /etc/os-release),
# e.g. "26.04".
getUbuntuRelease() {
    (. /etc/os-release 2>/dev/null && echo "${VERSION_ID:-}")
}

# buildFdeKernelPackageList prints the list of packages (one per line) needed
# to install the azure-fde kernel for the given Ubuntu release. Mirrors the
# package naming used for the existing 22.04/24.04 CVM builds in
# pre-install-dependencies.sh: only the *image* package carries the "-fde-"
# infix, the tools/cloud-tools/headers metapackages are shared with the
# vanilla azure kernel line.
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

# configureGrubForNewKernel regenerates grub.cfg so the newly-installed
# azure-fde kernel becomes the default boot entry, and fails if grub.cfg does
# not end up referencing an azure-fde kernel entry afterwards.
configureGrubForNewKernel() {
    if ! command -v update-grub &>/dev/null; then
        echo "ERROR: update-grub not found; the CVM bootstrap base image is expected to use GRUB" >&2
        exit 1
    fi

    # GRUB_DEFAULT=0 boots the first ("Advanced options" ordering places the
    # newest installed kernel first) menu entry; make this explicit rather
    # than relying on distro defaults.
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
    echo "Recorded original (pre-bootstrap) kernel: $(cat "${ORIGINAL_KERNEL_MARKER}")"

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

    echo "cvm-bootstrap-install-kernel.sh finished successfully; rebooting to verify azure-fde kernel boots"
}

# Allow the pure functions above to be sourced (e.g. by ShellSpec) without
# executing main.
if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
    main "$@"
fi
