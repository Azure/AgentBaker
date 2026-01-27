#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

NODE_EXPORTER_ROOT_DIR=""
NODE_EXPORTER_BASELINE_DIR=""
NODE_EXPORTER_DEFAULT_BUILD_ROOT="/opt/node-exporter"
NODE_EXPORTER_SKIP_FILE="/etc/node-exporter.d/skip_vhd_node_exporter"
NODE_EXPORTER_SERVICE_NAME="node-exporter.service"

node_exporter_locate_root_dir() {
    [ -n "${NODE_EXPORTER_ROOT_DIR}" ] && return 0

    local candidate="${SCRIPT_DIR}/node-exporter"
    if [ -d "${candidate}" ]; then
        NODE_EXPORTER_ROOT_DIR="$(cd "${candidate}" && pwd)"
    elif [ -d "${SCRIPT_DIR}/../../parts/linux/cloud-init/artifacts/node-exporter" ]; then
        NODE_EXPORTER_ROOT_DIR="$(cd "${SCRIPT_DIR}/../../parts/linux/cloud-init/artifacts/node-exporter" && pwd)"
    elif [ -d "/home/packer/node-exporter" ]; then
        NODE_EXPORTER_ROOT_DIR="/home/packer/node-exporter"
    else
        echo "[node-exporter] Unable to locate asset directory"
        return 1
    fi

    NODE_EXPORTER_BASELINE_DIR="${NODE_EXPORTER_ROOT_DIR}/baseline"
}

node_exporter_detect_arch() {
    CPU_ARCH=$(getCPUArch)
    case "${CPU_ARCH}" in
        amd64)
            NODE_EXPORTER_DEB_ARCH="amd64"
            NODE_EXPORTER_RPM_ARCH="x86_64"
            ;;
        arm64)
            NODE_EXPORTER_DEB_ARCH="arm64"
            NODE_EXPORTER_RPM_ARCH="aarch64"
            ;;
        *)
            echo "[node-exporter] Unsupported architecture: ${CPU_ARCH}"
            return 1
            ;;
    esac
}

node_exporter_copy_baseline_assets() {
    echo "[node-exporter] Copying baseline assets"

    mkdir -p /opt/bin /etc/systemd/system /etc/node-exporter.d

    cp -f "${NODE_EXPORTER_ROOT_DIR}/node-exporter-startup.sh" /opt/bin/
    chmod +x /opt/bin/node-exporter-startup.sh
    ln -sf /usr/bin/node-exporter /opt/bin/node-exporter

    cp -f "${NODE_EXPORTER_BASELINE_DIR}/etc/systemd/system/${NODE_EXPORTER_SERVICE_NAME}" /etc/systemd/system/
    cp -f "${NODE_EXPORTER_BASELINE_DIR}/etc/systemd/system/node-exporter-restart.service" /etc/systemd/system/
    cp -f "${NODE_EXPORTER_BASELINE_DIR}/etc/systemd/system/node-exporter-restart.path" /etc/systemd/system/
    cp -f "${NODE_EXPORTER_BASELINE_DIR}/etc/node-exporter.d/web-config.yml" /etc/node-exporter.d/
    cp -f "${NODE_EXPORTER_ROOT_DIR}/skip_vhd_node_exporter" "${NODE_EXPORTER_SKIP_FILE}"

    chmod 644 /etc/systemd/system/node-exporter.service \
              /etc/systemd/system/node-exporter-restart.service \
              /etc/systemd/system/node-exporter-restart.path \
              /etc/node-exporter.d/web-config.yml \
              "${NODE_EXPORTER_SKIP_FILE}"

    systemctl daemon-reload
    # Disable services during VHD build - CSE will enable and start them at provisioning time
    # This ensures the startup script runs with correct node-specific info (e.g., NODE_IP)
    systemctl disable "${NODE_EXPORTER_SERVICE_NAME}"
    systemctl disable node-exporter-restart.path
}

node_exporter_install_deb_stack() {
    local download_dir="${NODE_EXPORTER_BUILD_ROOT}"
    mkdir -p "${download_dir}"

    local package_name="node-exporter-kubernetes"
    local ne_tag="${NODE_EXPORTER_VERSION}-ubuntu${NODE_EXPORTER_UBUNTU_VERSION}u${NODE_EXPORTER_REVISION}"
    local ne_deb="${download_dir}/${package_name}_${ne_tag}_${NODE_EXPORTER_DEB_ARCH}.deb"
    local ne_url="https://packages.microsoft.com/ubuntu/${NODE_EXPORTER_UBUNTU_VERSION}/prod/pool/main/n/${package_name}/${package_name}_${ne_tag}_${NODE_EXPORTER_DEB_ARCH}.deb"

    if [ ! -f "${ne_deb}" ]; then
        echo "[node-exporter] Downloading ${ne_url}"
        retrycmd_curl_file 10 5 60 "${ne_deb}" "${ne_url}"
    fi
    apt_get_install 30 1 600 "${ne_deb}"
}

node_exporter_install_rpm_stack() {
    local download_dir="${NODE_EXPORTER_BUILD_ROOT}"
    mkdir -p "${download_dir}"

    local package_name="node-exporter-kubernetes"
    local rpm_repo="https://packages.microsoft.com/cbl-mariner/2.0/prod/cloud-native"
    local rpm_suffix="cm2"

    if [ "${OS}" = "${AZURELINUX_OS_NAME}" ]; then
        rpm_repo="https://packages.microsoft.com/azurelinux/3.0/prod/cloud-native"
        rpm_suffix="azl3"
    fi

    local ne_version_tag="${NODE_EXPORTER_VERSION}-${NODE_EXPORTER_REVISION}.${rpm_suffix}"
    local ne_rpm="${download_dir}/${package_name}-${ne_version_tag}.${NODE_EXPORTER_RPM_ARCH}.rpm"
    local ne_url="${rpm_repo}/${NODE_EXPORTER_RPM_ARCH}/Packages/n/${package_name}-${ne_version_tag}.${NODE_EXPORTER_RPM_ARCH}.rpm"

    if [ ! -f "${ne_rpm}" ]; then
        echo "[node-exporter] Downloading ${ne_url}"
        retrycmd_curl_file 10 5 60 "${ne_rpm}" "${ne_url}"
    fi
    dnf_install 30 1 600 "${ne_rpm}"
}

node_exporter_extract_package_version() {
    local package_json="$1"
    local os_type="$2"
    local os_release="$3"

    if [ "${os_type}" = "ubuntu" ]; then
        # Use 18.04 package for all Ubuntu versions - same approach as aks-vm-extension
        local version=$(jq -r '.downloadURIs.ubuntu.current.versionsV2[0].latestVersion' <<<"${package_json}")
        # Parse "1.9.1-ubuntu18.04u5" -> version:revision:ubuntu_version
        local ne_ver=$(echo "${version}" | sed -n 's/^\([0-9.]*\)-ubuntu\([0-9.]*\)u\([0-9]*\)$/\1:\3:\2/p')
        echo "${ne_ver}"
    else
        local release_key="current"
        [ "${os_type}" = "azurelinux" ] && release_key="v3.0"

        local version=$(jq -r --arg release "${release_key}" '.downloadURIs.'${os_type}'[$release].versionsV2[0].latestVersion' <<<"${package_json}")
        # Parse "1.9.1-5.cm2" or "1.9.1-5.azl3" -> version:revision
        local ne_ver=$(echo "${version}" | sed -n 's/^\([0-9.]*\)-\([0-9]*\)\.\(cm2\|azl3\)$/\1:\2/p')
        echo "${ne_ver}"
    fi
}

installNodeExporter() {
    local package_json="$1"
    local download_dir="$2"

    NODE_EXPORTER_BUILD_ROOT="${download_dir:-${NODE_EXPORTER_DEFAULT_BUILD_ROOT}}"

    node_exporter_detect_arch
    mkdir -p "${NODE_EXPORTER_BUILD_ROOT}"

    # Skip for OSGuard, Flatcar, and Kata
    if { [ "${OS}" = "${AZURELINUX_OS_NAME}" ] && [ "${OS_VARIANT}" = "${AZURELINUX_OSGUARD_OS_VARIANT}" ]; } || [ "${OS}" = "FLATCAR" ] || [ "${IS_KATA:-false}" = "true" ]; then
        echo "[node-exporter] Skipping for ${OS} ${OS_VARIANT:-default} (IS_KATA=${IS_KATA:-false})"
        rm -rf "${NODE_EXPORTER_BUILD_ROOT}"
        return 0
    fi

    node_exporter_locate_root_dir

    local version_info
    if [ "${OS}" = "${MARINER_OS_NAME}" ]; then
        version_info=$(node_exporter_extract_package_version "${package_json}" "mariner" "current")
    elif [ "${OS}" = "${AZURELINUX_OS_NAME}" ]; then
        version_info=$(node_exporter_extract_package_version "${package_json}" "azurelinux" "v3.0")
    else
        # Use 18.04 package for all Ubuntu versions - same approach as aks-vm-extension
        version_info=$(node_exporter_extract_package_version "${package_json}" "ubuntu" "current")
    fi

    IFS=':' read -r NODE_EXPORTER_VERSION NODE_EXPORTER_REVISION NODE_EXPORTER_UBUNTU_VERSION <<< "${version_info}"

    if [ "${OS}" = "${MARINER_OS_NAME}" ] || [ "${OS}" = "${AZURELINUX_OS_NAME}" ]; then
        echo "[node-exporter] Installing via RPM"
        node_exporter_install_rpm_stack
    else
        echo "[node-exporter] Installing via DEB"
        node_exporter_install_deb_stack
    fi

    node_exporter_copy_baseline_assets

    [ -n "${VHD_LOGS_FILEPATH:-}" ] && echo "  - node-exporter ${NODE_EXPORTER_VERSION}-${NODE_EXPORTER_REVISION}" >> "${VHD_LOGS_FILEPATH}"

    rm -rf "${NODE_EXPORTER_BUILD_ROOT}"
}
