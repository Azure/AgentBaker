#!/bin/bash
# This script handles Inspektor Gadget package installation during VHD build.
# Baseline files (helper scripts, systemd service) are copied by packer_source.sh.

set -euo pipefail

IG_DEFAULT_BUILD_ROOT="/opt/azure/ig"
IG_SERVICE_NAME="ig-import-gadgets.service"
IG_SKIP_FILE="/etc/ig.d/skip_vhd_ig"

ig_extract_package_metadata() {
    local package_json="$1"
    local version="$2"

    local revision
    if ! revision=$(jq -r --arg version "${version}" '.downloadURIs.default.current.versionsV2[]? | select(.latestVersion == $version) | .revision // empty' <<<"${package_json}"); then
        echo "[ig] Failed to parse revision for Inspektor Gadget from components metadata"
        return 1
    fi

    if [[ -z "${revision}" || "${revision}" == "null" ]]; then
        if ! revision=$(jq -r '.revision // empty' <<<"${package_json}"); then
            echo "[ig] Failed to read fallback revision for Inspektor Gadget"
            return 1
        fi
    fi

    if [[ -z "${revision}" || "${revision}" == "null" ]]; then
        echo "[ig] Unable to determine revision for Inspektor Gadget version ${version}"
        return 1
    fi

    echo "${revision}"
}

ig_detect_arch() {
    CPU_ARCH=$(getCPUArch)
    case "${CPU_ARCH}" in
        amd64)
            IG_DEB_ARCH="amd64"
            IG_RPM_ARCH="x86_64"
            ;;
        arm64)
            IG_DEB_ARCH="arm64"
            IG_RPM_ARCH="aarch64"
            ;;
        *)
            echo "[ig] Unsupported CPU architecture: ${CPU_ARCH}"
            return 1
            ;;
    esac
}

ig_download_file() {
    local url="$1"
    local dest="$2"

    if [[ -f "${dest}" ]]; then
        return 0
    fi

    echo "[ig] Downloading ${url}"
    mkdir -p "$(dirname "${dest}")"
    if ! retrycmd_curl_file 10 5 60 "${dest}" "${url}"; then
        echo "[ig] Failed to download ${url}"
        return 1
    fi
}

ig_enable_service_unit() {
    local unit_path="/usr/lib/systemd/system/${IG_SERVICE_NAME}"

    if [[ ! -f "${unit_path}" ]]; then
        echo "[ig] ${IG_SERVICE_NAME} not present; skipping enablement"
        return 0
    fi

    if ! systemctl daemon-reload; then
        echo "[ig] systemctl daemon-reload failed"
        return 1
    fi

    if ! systemctl enable "${IG_SERVICE_NAME}"; then
        echo "[ig] Failed to enable ${IG_SERVICE_NAME}"
        return 1
    fi

    return 0
}

ig_import_gadgets() {
    if [[ ! -x /usr/share/inspektor-gadget/import_gadgets.sh ]]; then
        echo "[ig] import_gadgets.sh not found"
        return 1
    fi

    echo "[ig] Running gadget import"
    if ! /usr/share/inspektor-gadget/import_gadgets.sh; then
        echo "[ig] Gadget import script failed"
        return 1
    fi
}

ig_install_deb_stack() {
    local download_dir="${IG_BUILD_ROOT}/downloads"
    mkdir -p "${download_dir}"

    local ig_tag="${IG_VERSION}-ubuntu18.04u${IG_REVISION}"
    local ig_deb="${download_dir}/ig_${ig_tag}_${IG_DEB_ARCH}.deb"
    local ig_url="https://packages.microsoft.com/ubuntu/18.04/prod/pool/main/i/ig/ig_${ig_tag}_${IG_DEB_ARCH}.deb"

    local ig_gadgets_tag="${IG_VERSION}-ubuntu20.04u${IG_REVISION}"
    local ig_gadgets_deb="${download_dir}/ig-gadgets_${ig_gadgets_tag}_${IG_DEB_ARCH}.deb"
    local ig_gadgets_url="https://packages.microsoft.com/ubuntu/20.04/prod/pool/main/i/ig-gadgets/ig-gadgets_${ig_gadgets_tag}_${IG_DEB_ARCH}.deb"

    ig_download_file "${ig_url}" "${ig_deb}" || return 1
    ig_download_file "${ig_gadgets_url}" "${ig_gadgets_deb}" || return 1

    if ! apt_get_install 30 1 600 "${ig_deb}" "${ig_gadgets_deb}"; then
        echo "[ig] Failed to install IG deb packages"
        return 1
    fi
}

ig_install_rpm_stack() {
    local download_dir="${IG_BUILD_ROOT}/downloads"
    mkdir -p "${download_dir}"

    local rpm_arch_dir="${IG_RPM_ARCH}"

    local rpm_repo="https://packages.microsoft.com/cbl-mariner/2.0/prod/cloud-native"
    local gadgets_repo="https://packages.microsoft.com/azurelinux/3.0/prod/cloud-native"
    local rpm_suffix="cm2"

    if [[ "${OS}" == "${AZURELINUX_OS_NAME}" ]]; then
        rpm_repo="https://packages.microsoft.com/azurelinux/3.0/prod/cloud-native"
        gadgets_repo="https://packages.microsoft.com/azurelinux/3.0/prod/cloud-native"
        rpm_suffix="azl3"
    fi

    local ig_version_tag="${IG_VERSION}-${IG_REVISION}.${rpm_suffix}"
    local ig_rpm="${download_dir}/ig-${ig_version_tag}.${IG_RPM_ARCH}.rpm"
    local ig_url="${rpm_repo}/${rpm_arch_dir}/Packages/i/ig-${ig_version_tag}.${IG_RPM_ARCH}.rpm"
    ig_download_file "${ig_url}" "${ig_rpm}" || return 1

    local ig_gadgets_repo_suffix="azl3"
    local ig_gadgets_version_tag="${IG_VERSION}-${IG_REVISION}.${ig_gadgets_repo_suffix}"
    local ig_gadgets_rpm="${download_dir}/ig-gadgets-${ig_gadgets_version_tag}.${IG_RPM_ARCH}.rpm"
    local ig_gadgets_url="${gadgets_repo}/${rpm_arch_dir}/Packages/i/ig-gadgets-${ig_gadgets_version_tag}.${IG_RPM_ARCH}.rpm"
    ig_download_file "${ig_gadgets_url}" "${ig_gadgets_rpm}" || return 1

    if ! dnf_install 30 1 600 "${ig_rpm}" "${ig_gadgets_rpm}"; then
        echo "[ig] Failed to install IG rpm packages"
        return 1
    fi
}

ig_cleanup_build_artifacts() {
    if [[ -n "${IG_BUILD_ROOT:-}" && -d "${IG_BUILD_ROOT}" ]]; then
        rm -rf "${IG_BUILD_ROOT}"
    fi
}

ig_log_version() {
    if [[ -n "${VHD_LOGS_FILEPATH:-}" ]]; then
        echo "  - ig version ${IG_VERSION}-${IG_REVISION}" >> "${VHD_LOGS_FILEPATH}"
    fi
}

installIG() {
    local package_json="$1"
    local version="$2"
    local download_dir="$3"

    if [[ -z "${version}" || "${version}" == "null" ]]; then
        echo "[ig] Invalid or empty Inspektor Gadget version"
        return 1
    fi

    local revision
    if ! revision=$(ig_extract_package_metadata "${package_json}" "${version}"); then
        return 1
    fi

    IG_VERSION="${version}"
    IG_REVISION="${revision}"

    IG_BUILD_ROOT="${download_dir}"
    if [[ -z "${IG_BUILD_ROOT}" || "${IG_BUILD_ROOT}" == "null" ]]; then
        IG_BUILD_ROOT="${IG_DEFAULT_BUILD_ROOT}"
    fi

    ig_detect_arch || return 1

    mkdir -p "${IG_BUILD_ROOT}"

    # For Mariner, OSGuard, Flatcar, and ACL, skip IG installation entirely during VHD build
    # install-ig.sh is only present for sourcing by install-dependencies.sh
    if [[ "${OS}" == "${MARINER_OS_NAME}" || ("${OS}" == "${AZURELINUX_OS_NAME}" && "${OS_VARIANT}" == "${AZURELINUX_OSGUARD_OS_VARIANT}") || "${OS}" == "FLATCAR" || "${OS}" == "ACL" ]]; then
        echo "[ig] Skipping IG installation for ${OS} ${OS_VARIANT:-default} - no files will be staged in VHD"
        ig_cleanup_build_artifacts
        return 0
    fi

    if [[ "${OS}" == "${AZURELINUX_OS_NAME}" ]]; then
        echo "[ig] Installing IG via RPM"
        if ! ig_install_rpm_stack; then
            ig_cleanup_build_artifacts
            return 1
        fi
    else
        echo "[ig] Installing IG via DEB"
        if ! ig_install_deb_stack; then
            ig_cleanup_build_artifacts
            return 1
        fi
    fi

    # Enable the systemd service (baseline files copied by packer_source.sh)
    ig_enable_service_unit || echo "[ig] Failed to enable ${IG_SERVICE_NAME}"
    ig_import_gadgets || echo "[ig] Gadget import failed during build"

    # Create skip sentinel file to indicate IG was installed from VHD
    mkdir -p /etc/ig.d
    touch "${IG_SKIP_FILE}"
    chmod 644 "${IG_SKIP_FILE}"

    ig_log_version
    ig_cleanup_build_artifacts
}
