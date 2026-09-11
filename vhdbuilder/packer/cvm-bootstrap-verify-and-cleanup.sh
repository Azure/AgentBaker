#!/bin/bash

set -euo pipefail

ORIGINAL_KERNEL_MARKER="${ORIGINAL_KERNEL_MARKER:-/opt/azure/cvm-bootstrap-original-kernel}"

assertRunningAzureFdeKernel() {
    local current_kernel="$1"
    case "${current_kernel}" in
        *-azure-fde)
            echo "Running kernel ${current_kernel} has the expected azure-fde suffix"
            ;;
        *)
            echo "ERROR: running kernel '${current_kernel}' does not have the expected '-azure-fde' suffix" >&2
            echo "       The reboot into the azure-fde kernel did not take effect; refusing to continue." >&2
            exit 1
            ;;
    esac
}

# shellcheck disable=SC2120
assertEfiBoot() {
    local efi_path="${1:-/sys/firmware/efi}"
    if [ ! -d "${efi_path}" ]; then
        echo "ERROR: ${efi_path} is missing; the VM did not boot in UEFI mode" >&2
        exit 1
    fi
    echo "${efi_path} present: VM booted in UEFI mode"
}

assertPackageStateClean() {
    local audit_output
    audit_output="$(dpkg --audit 2>&1 || true)"
    if [ -n "${audit_output}" ]; then
        echo "ERROR: dpkg --audit reported packages in an inconsistent state:" >&2
        echo "${audit_output}" >&2
        exit 1
    fi
    echo "dpkg --audit: clean"

    if ! apt-get check; then
        echo "ERROR: apt-get check reported broken dependencies" >&2
        exit 1
    fi
    echo "apt-get check: clean"
}

computeSafeKernelPurgeList() {
    local prior_kernel="$1"
    local current_kernel="$2"
    local ubuntu_release="$3"
    local pkg
    local -a candidates=()

    if [ -z "${prior_kernel}" ]; then
        echo "ERROR: prior kernel version is empty; refusing to compute a purge list" >&2
        return 1
    fi
    if [ "${prior_kernel}" = "${current_kernel}" ]; then
        echo "ERROR: prior kernel (${prior_kernel}) equals the currently running kernel; refusing to purge anything" >&2
        return 1
    fi

    while IFS= read -r pkg; do
        [ -n "${pkg}" ] && candidates+=("${pkg}")
    done < <(dpkg-query -W -f='${Package}\n' 'linux-image-*' 'linux-modules-*' 'linux-headers-*' 'linux-tools-*' 'linux-cloud-tools-*' 2>/dev/null | grep -F -- "${prior_kernel}" || true)

    for pkg in "linux-azure-lts-${ubuntu_release}" "linux-image-azure-lts-${ubuntu_release}"; do
        if dpkg-query -W -f='${Status}' "${pkg}" 2>/dev/null | grep -q "install ok installed"; then
            candidates+=("${pkg}")
        fi
    done

    for pkg in "${candidates[@]}"; do
        case "${pkg}" in
            *"${current_kernel}"*)
                echo "Refusing to purge ${pkg}: matches the currently running kernel (${current_kernel})" >&2
                ;;
            *)
                printf '%s\n' "${pkg}"
                ;;
        esac
    done
}

main() {
    local current_kernel
    local prior_kernel
    local ubuntu_release
    local -a purge_list=()
    local pkg

    current_kernel="$(uname -r)"
    assertRunningAzureFdeKernel "${current_kernel}"

    if [ ! -f "${ORIGINAL_KERNEL_MARKER}" ]; then
        echo "ERROR: original kernel marker ${ORIGINAL_KERNEL_MARKER} not found; Stage 1 install step did not run first" >&2
        exit 1
    fi
    prior_kernel="$(cat "${ORIGINAL_KERNEL_MARKER}")"
    ubuntu_release="$(. /etc/os-release 2>/dev/null && echo "${VERSION_ID:-}")"

    while IFS= read -r pkg; do
        [ -n "${pkg}" ] && purge_list+=("${pkg}")
    done < <(computeSafeKernelPurgeList "${prior_kernel}" "${current_kernel}" "${ubuntu_release}")

    if [ "${#purge_list[@]}" -gt 0 ]; then
        echo "Purging obsolete prior-kernel / non-FDE kernel packages: ${purge_list[*]}"
        while fuser /var/lib/dpkg/lock /var/lib/apt/lists/lock /var/cache/apt/archives/lock /var/lib/dpkg/lock-frontend >/dev/null 2>&1; do
            echo "Waiting for release of apt locks"
            sleep 3
        done
        DEBIAN_FRONTEND=noninteractive apt-get remove --purge -y "${purge_list[@]}"
        DEBIAN_FRONTEND=noninteractive apt-get autoremove --purge -y
        DEBIAN_FRONTEND=noninteractive apt-get clean
    else
        echo "No obsolete prior-kernel or non-FDE kernel metapackages found to purge"
    fi

    rm -f "${ORIGINAL_KERNEL_MARKER}"

    # shellcheck disable=SC2119
    assertEfiBoot
    assertPackageStateClean

    echo "cvm-bootstrap-verify-and-cleanup.sh finished successfully; image is ready to be deprovisioned and captured"
}

if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
    main "$@"
fi
