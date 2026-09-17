#!/bin/bash

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MARKED_FOR_REMOVAL_PACKAGES_FILE="${SCRIPT_DIR}/marked-for-removal-packages.txt"
REQUIRED_PACKAGES_FILE="${SCRIPT_DIR}/required-packages.txt"

readPackageList() {
    sed -e 's/[[:space:]]*#.*$//' -e '/^[[:space:]]*$/d' "$1"
}

validatePackageList() {
    local package_file="$1"
    local description="$2"

    if [ ! -s "${package_file}" ] || [ -z "$(readPackageList "${package_file}")" ]; then
        echo "${description} is missing or empty: ${package_file}" >&2
        return 1
    fi
}

verifyRequiredPackagesInstalled() {
    local required

    validatePackageList "${REQUIRED_PACKAGES_FILE}" "Required package list" || return 1

    while IFS= read -r required; do
        if ! dpkg-query -W -f='${db:Status-Status}\n' "${required}" 2>/dev/null | grep -Fxq "installed"; then
            echo "Required CVM package pattern is not installed: ${required}" >&2
            return 1
        fi
    done < <(readPackageList "${REQUIRED_PACKAGES_FILE}")
}

main() {
    local package
    local -a purge_packages=()

    case "${1:-}" in
        --verify-only)
            verifyRequiredPackagesInstalled
            return
            ;;
        "")
            ;;
        *)
            echo "Unknown argument: $1" >&2
            return 1
            ;;
    esac

    validatePackageList "${MARKED_FOR_REMOVAL_PACKAGES_FILE}" "Marked-for-removal package list" || return 1

    while IFS= read -r package; do
        if [ "$(dpkg-query -W -f='${db:Status-Status}' "${package}" 2>/dev/null || true)" = "installed" ]; then
            purge_packages+=("${package}")
        fi
    done < <(readPackageList "${MARKED_FOR_REMOVAL_PACKAGES_FILE}")

    if [ "${#purge_packages[@]}" -gt 0 ]; then
        echo "Purging ${#purge_packages[@]} installed server-cvm packages marked for removal"
        DEBIAN_FRONTEND=noninteractive apt-get -o DPkg::Lock::Timeout=300 purge -y --no-auto-remove --allow-remove-essential "${purge_packages[@]}"
    else
        echo "No installed server-cvm packages marked for removal were found"
    fi

    apt-mark manual curl gpg jq logrotate rsyslog sudo xfsprogs
    verifyRequiredPackagesInstalled
}

if [ "${BASH_SOURCE[0]}" = "$0" ]; then
    set -euo pipefail
    main "$@"
fi
