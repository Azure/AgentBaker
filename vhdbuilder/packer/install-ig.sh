#!/bin/bash
# This script handles Inspektor Gadget package installation during VHD build.
# Baseline files (helper scripts, systemd service) are copied by packer_source.sh.

set -euo pipefail

IG_SERVICE_NAME="ig-import-gadgets.service"
IG_SKIP_FILE="/etc/ig.d/skip_vhd_ig"

# ig-gadgets is built independently from ig (separate Dalec specs), so versions
# are managed separately here rather than derived from the ig version.
# NOTE: ig-gadgets deb is only published to the 20.04 repo on PMC, even though
# the project only builds 22.04 and 24.04 VHDs. The 20.04 deb is compatible.
IG_GADGETS_DEB_VERSION="0.49.1-ubuntu20.04u1"
IG_GADGETS_RPM_VERSION="0.49.1-1.azl3"

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
    # ig deb was already downloaded via downloadPkgFromVersion to IG_BUILD_ROOT
    local ig_deb="${IG_BUILD_ROOT}/ig_${IG_VERSION}_${IG_DEB_ARCH}.deb"
    if [[ ! -f "${ig_deb}" ]]; then
        echo "[ig] ig deb not found at ${ig_deb}"
        return 1
    fi

    # ig-gadgets: always from ubuntu 20.04 repo, version managed independently
    local ig_gadgets_deb="${IG_BUILD_ROOT}/ig-gadgets_${IG_GADGETS_DEB_VERSION}_${IG_DEB_ARCH}.deb"
    local ig_gadgets_url="https://packages.microsoft.com/ubuntu/20.04/prod/pool/main/i/ig-gadgets/ig-gadgets_${IG_GADGETS_DEB_VERSION}_${IG_DEB_ARCH}.deb"

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
    local rpm_repo="https://packages.microsoft.com/azurelinux/3.0/prod/cloud-native"

    # IG_VERSION is the full version tag from components.json (e.g. "0.45.0-1.azl3")
    local ig_rpm="${download_dir}/ig-${IG_VERSION}.${IG_RPM_ARCH}.rpm"
    local ig_url="${rpm_repo}/${rpm_arch_dir}/Packages/i/ig-${IG_VERSION}.${IG_RPM_ARCH}.rpm"
    ig_download_file "${ig_url}" "${ig_rpm}" || return 1

    # ig-gadgets: version managed independently from ig
    local ig_gadgets_rpm="${download_dir}/ig-gadgets-${IG_GADGETS_RPM_VERSION}.${IG_RPM_ARCH}.rpm"
    local ig_gadgets_url="${rpm_repo}/${rpm_arch_dir}/Packages/i/ig-gadgets-${IG_GADGETS_RPM_VERSION}.${IG_RPM_ARCH}.rpm"
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
        echo "  - ig version ${IG_VERSION}" >> "${VHD_LOGS_FILEPATH}"
    fi
}

installIG() {
    local version="$1"
    local download_dir="$2"

    if [[ -z "${version}" || "${version}" == "null" ]]; then
        echo "[ig] Invalid or empty Inspektor Gadget version"
        return 1
    fi

    IG_VERSION="${version}"

    IG_BUILD_ROOT="${download_dir}"
    if [[ -z "${IG_BUILD_ROOT}" || "${IG_BUILD_ROOT}" == "null" ]]; then
        echo "[ig] download_dir is required"
        return 1
    fi

    ig_detect_arch || return 1

    mkdir -p "${IG_BUILD_ROOT}"

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
