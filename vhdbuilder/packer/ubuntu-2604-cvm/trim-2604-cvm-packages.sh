#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MARKED_FOR_REMOVAL_PACKAGES_FILE="${SCRIPT_DIR}/marked-for-removal-packages.txt"
DEFERRED_SYSTEMD_PACKAGES_FILE="${SCRIPT_DIR}/deferred-systemd-packages.txt"
REQUIRED_PACKAGES_FILE="${SCRIPT_DIR}/required-packages.txt"

readPackageList() {
    sed -e 's/[[:space:]]*#.*$//' -e '/^[[:space:]]*$/d' "$1"
}

verifyRequiredPackagesInstalled() {
    local required

    while IFS= read -r required; do
        if ! dpkg-query -W -f='${db:Status-Status}\n' "${required}" 2>/dev/null | grep -Fxq "installed"; then
            echo "Required CVM package pattern is not installed: ${required}" >&2
            return 1
        fi
    done < <(readPackageList "${REQUIRED_PACKAGES_FILE}")
}

purgeInstalledPackages() {
    local package_list_file="$1"
    local package_description="$2"
    local package
    local -a purge_packages=()

    [ -s "${package_list_file}" ] || {
        echo "Package list is missing or empty: ${package_list_file}" >&2
        return 1
    }

    while IFS= read -r package; do
        if [ "$(dpkg-query -W -f='${db:Status-Status}' "${package}" 2>/dev/null || true)" = "installed" ]; then
            purge_packages+=("${package}")
        fi
    done < <(readPackageList "${package_list_file}")

    if [ "${#purge_packages[@]}" -gt 0 ]; then
        echo "Purging ${#purge_packages[@]} installed ${package_description}"
        DEBIAN_FRONTEND=noninteractive apt-get -o DPkg::Lock::Timeout=300 purge -y --no-auto-remove --allow-remove-essential "${purge_packages[@]}"
    else
        echo "No installed ${package_description} were found"
    fi
}

removeSystemdPackages() {
    purgeInstalledPackages "${DEFERRED_SYSTEMD_PACKAGES_FILE}" "deferred systemd-related server-cvm packages"
}

main() {
    [ -s "${REQUIRED_PACKAGES_FILE}" ] || {
        echo "Required package list is missing or empty: ${REQUIRED_PACKAGES_FILE}" >&2
        return 1
    }

    case "${1:-}" in
        "")
            purgeInstalledPackages "${MARKED_FOR_REMOVAL_PACKAGES_FILE}" "server-cvm packages marked for removal"
            ;;
        --systemd-packages)
            removeSystemdPackages
            ;;
        *)
            echo "Unsupported argument: $1" >&2
            return 1
            ;;
    esac

    verifyRequiredPackagesInstalled
}

main "$@"
