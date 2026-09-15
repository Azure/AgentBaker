#!/bin/bash
# shellcheck disable=SC3010 # This script is invoked explicitly with bash by the CVM Packer template.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MARKED_FOR_REMOVAL_PACKAGES_FILE="${MARKED_FOR_REMOVAL_PACKAGES_FILE:-${SCRIPT_DIR}/marked-for-removal-packages.txt}"
REQUIRED_PACKAGES_FILE="${REQUIRED_PACKAGES_FILE:-${SCRIPT_DIR}/required-packages.txt}"
OS_RELEASE_FILE="${OS_RELEASE_FILE:-/etc/os-release}"

feature_flag_enabled() {
    local expected_flag="$1"
    tr ',' '\n' <<<"${FEATURE_FLAGS:-}" | grep -Fxq "${expected_flag}"
}

should_prune_ubuntu_2604_server_cvm() {
    local os_id os_version

    [ -r "${OS_RELEASE_FILE}" ] || return 1
    # shellcheck disable=SC1090
    . "${OS_RELEASE_FILE}"
    os_id="${ID:-}"
    os_version="${VERSION_ID:-}"

    [ "${os_id,,}" = "ubuntu" ] &&
        [ "${os_version}" = "26.04" ] &&
        [ "${IMG_SKU:-}" = "server-cvm" ] &&
        feature_flag_enabled "cvm"
}

read_package_list() {
    local package_file="$1"
    sed -e 's/[[:space:]]*#.*$//' -e '/^[[:space:]]*$/d' "${package_file}"
}

validate_package_lists() {
    local marked_for_removal package required

    [ -s "${MARKED_FOR_REMOVAL_PACKAGES_FILE}" ] || {
        echo "Marked-for-removal package list is missing or empty: ${MARKED_FOR_REMOVAL_PACKAGES_FILE}" >&2
        return 1
    }
    [ -s "${REQUIRED_PACKAGES_FILE}" ] || {
        echo "Required package list is missing or empty: ${REQUIRED_PACKAGES_FILE}" >&2
        return 1
    }

    while IFS= read -r marked_for_removal; do
        if [[ ! "${marked_for_removal}" =~ ^[a-z0-9][a-z0-9+.-]*$ ]]; then
            echo "Invalid marked-for-removal package name: ${marked_for_removal}" >&2
            return 1
        fi
    done < <(read_package_list "${MARKED_FOR_REMOVAL_PACKAGES_FILE}")

    while IFS= read -r required; do
        if [[ ! "${required}" =~ ^[a-z0-9][a-z0-9+.*-]*$ ]]; then
            echo "Invalid required package pattern: ${required}" >&2
            return 1
        fi
        while IFS= read -r marked_for_removal; do
            # shellcheck disable=SC2053 # required entries intentionally support package-name globs.
            if [[ "${marked_for_removal}" == ${required} ]]; then
                echo "Marked-for-removal package is protected by required pattern ${required}: ${marked_for_removal}" >&2
                return 1
            fi
        done < <(read_package_list "${MARKED_FOR_REMOVAL_PACKAGES_FILE}")
    done < <(read_package_list "${REQUIRED_PACKAGES_FILE}")
}

normalize_package_name() {
    printf '%s\n' "${1%%:*}"
}

package_is_marked_for_removal() {
    local package
    package="$(normalize_package_name "$1")"
    grep -Fxq "${package}" < <(read_package_list "${MARKED_FOR_REMOVAL_PACKAGES_FILE}")
}

package_is_required() {
    local package required
    package="$(normalize_package_name "$1")"
    while IFS= read -r required; do
        # shellcheck disable=SC2053 # required entries intentionally support package-name globs.
        if [[ "${package}" == ${required} ]]; then
            return 0
        fi
    done < <(read_package_list "${REQUIRED_PACKAGES_FILE}")
    return 1
}

list_installed_packages() {
    dpkg-query -W -f='${binary:Package}\t${db:Status-Status}\n' |
        while IFS=$'\t' read -r package status; do
            [ "${status}" = "installed" ] && printf '%s\n' "${package}"
        done
}

verify_required_packages_installed() {
    local installed_file="$1"
    local required package matched

    while IFS= read -r required; do
        matched=false
        while IFS= read -r package; do
            package="$(normalize_package_name "${package}")"
            # shellcheck disable=SC2053 # required entries intentionally support package-name globs.
            if [[ "${package}" == ${required} ]]; then
                matched=true
                break
            fi
        done < "${installed_file}"
        if [ "${matched}" != "true" ]; then
            echo "Required CVM package pattern is not installed: ${required}" >&2
            return 1
        fi
    done < <(read_package_list "${REQUIRED_PACKAGES_FILE}")
}

parse_simulated_removals() {
    local action package
    while read -r action package _; do
        case "${action}" in
            Remv | Purg)
                [ -n "${package}" ] && printf '%s\n' "${package}"
                ;;
        esac
    done
}

validate_removal_plan() {
    local removal_file="$1"
    local essential_file="$2"
    local package normalized

    [ -s "${removal_file}" ] || {
        echo "apt simulation did not report any package removals" >&2
        return 1
    }

    while IFS= read -r package; do
        normalized="$(normalize_package_name "${package}")"
        if package_is_required "${normalized}"; then
            echo "apt simulation would remove required CVM package: ${package}" >&2
            return 1
        fi
        if ! package_is_marked_for_removal "${normalized}"; then
            echo "apt simulation would remove unspecified package: ${package}" >&2
            return 1
        fi
        if grep -Fxq "${normalized}" "${essential_file}"; then
            echo "apt simulation would remove Essential package: ${package}" >&2
            return 1
        fi
    done < "${removal_file}"
}

verify_all_marked_for_removal_planned() {
    local installed_marked_for_removal_file="$1"
    local removal_file="$2"
    local package normalized

    while IFS= read -r package; do
        normalized="$(normalize_package_name "${package}")"
        if ! grep -Fxq "${normalized}" < <(sed 's/:.*//' "${removal_file}"); then
            echo "apt simulation omitted installed marked-for-removal package: ${package}" >&2
            return 1
        fi
    done < "${installed_marked_for_removal_file}"
}

verify_no_marked_for_removal_installed() {
    local installed_file="$1"
    local package

    while IFS= read -r package; do
        if package_is_marked_for_removal "${package}"; then
            echo "Marked-for-removal package remained installed after pruning: ${package}" >&2
            return 1
        fi
    done < "${installed_file}"
}

main() {
    local work_dir installed_before installed_after installed_marked_for_removal
    local unspecified_packages essential_packages simulation_output removal_plan audit_output
    local package normalized
    local -a purge_packages=()

    if ! should_prune_ubuntu_2604_server_cvm; then
        echo "Skipping package pruning: only Ubuntu 26.04 server-cvm builds with the cvm feature are supported"
        return 0
    fi

    validate_package_lists

    work_dir="$(mktemp -d)"
    PRUNE_WORK_DIR="${work_dir}"
    trap 'rm -rf "${PRUNE_WORK_DIR}"' EXIT
    installed_before="${work_dir}/installed-before.txt"
    installed_after="${work_dir}/installed-after.txt"
    installed_marked_for_removal="${work_dir}/installed-marked-for-removal.txt"
    unspecified_packages="${work_dir}/unspecified-packages.txt"
    essential_packages="${work_dir}/essential-packages.txt"
    simulation_output="${work_dir}/apt-simulation.txt"
    removal_plan="${work_dir}/removal-plan.txt"
    audit_output="${work_dir}/dpkg-audit.txt"

    list_installed_packages | sort -u > "${installed_before}"
    verify_required_packages_installed "${installed_before}"

    while IFS= read -r package; do
        normalized="$(normalize_package_name "${package}")"
        if package_is_marked_for_removal "${normalized}"; then
            printf '%s\n' "${package}" >> "${installed_marked_for_removal}"
        else
            printf '%s\n' "${package}" >> "${unspecified_packages}"
        fi
        if [ "$(dpkg-query -W -f='${Essential}' "${package}")" = "yes" ]; then
            printf '%s\n' "${normalized}" >> "${essential_packages}"
        fi
    done < "${installed_before}"

    if [ -s "${unspecified_packages}" ]; then
        xargs -r -n 100 apt-mark -o DPkg::Lock::Timeout=300 manual < "${unspecified_packages}"
    fi

    if [ ! -s "${installed_marked_for_removal}" ]; then
        echo "No installed server-cvm packages marked for removal were found"
        dpkg --audit > "${audit_output}"
        [ ! -s "${audit_output}" ] || {
            cat "${audit_output}" >&2
            return 1
        }
        apt-get -o DPkg::Lock::Timeout=300 check
        return 0
    fi

    mapfile -t purge_packages < "${installed_marked_for_removal}"
    if ! DEBIAN_FRONTEND=noninteractive apt-get -o DPkg::Lock::Timeout=300 --simulate purge "${purge_packages[@]}" > "${simulation_output}" 2>&1; then
        cat "${simulation_output}" >&2
        echo "apt purge simulation failed" >&2
        return 1
    fi

    parse_simulated_removals < "${simulation_output}" | sort -u > "${removal_plan}"
    validate_removal_plan "${removal_plan}" "${essential_packages}"
    verify_all_marked_for_removal_planned "${installed_marked_for_removal}" "${removal_plan}"

    echo "Purging $(wc -l < "${installed_marked_for_removal}") installed server-cvm packages marked for removal"
    DEBIAN_FRONTEND=noninteractive apt-get -o DPkg::Lock::Timeout=300 purge -y --no-auto-remove "${purge_packages[@]}"

    list_installed_packages | sort -u > "${installed_after}"
    verify_required_packages_installed "${installed_after}"
    verify_no_marked_for_removal_installed "${installed_after}"

    dpkg --audit > "${audit_output}"
    if [ -s "${audit_output}" ]; then
        cat "${audit_output}" >&2
        return 1
    fi
    apt-get -o DPkg::Lock::Timeout=300 check
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    set -euo pipefail
    main "$@"
fi
