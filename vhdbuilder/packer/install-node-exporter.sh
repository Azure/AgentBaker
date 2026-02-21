#!/bin/bash
# Fully install node-exporter at VHD build time so CSE only needs to reenable the systemd units.
#
# installNodeExporter():
#   1. Installs via apt_get_install (Ubuntu) or dnf_install (AzureLinux)
#   2. Symlinks /usr/bin/node-exporter to /opt/bin for consistency with kubelet/kubectl
#   3. Disables systemd services (CSE reenables at provisioning time)
#   4. Creates the skip sentinel file
#   5. Logs the installed version

set -euo pipefail

installNodeExporter() {
    local version="$1"
    local pkg="node-exporter-kubernetes"

    echo "[node-exporter] Installing ${pkg} version ${version}"

    if isUbuntu; then
        apt_get_install 30 1 600 "${pkg}=${version}" || exit $ERR_APT_INSTALL_TIMEOUT
    elif isAzureLinux; then
        dnf_install 30 1 600 "${pkg}-${version}" || exit $ERR_APT_INSTALL_TIMEOUT
    else
        echo "[node-exporter] Unsupported OS for node-exporter install"
        return 1
    fi

    # Symlink to /opt/bin for consistency with other binaries
    mkdir -p /opt/bin
    ln -snf "/usr/bin/node-exporter" "/opt/bin/node-exporter"

    # Reload systemd to pick up service files copied by packer_source.sh, then disable node-exporter.
    # It will be enabled and started by CSE during provisioning via configureNodeExporter()
    echo "[node-exporter] Disabling node-exporter services. Gets systemctlEnableAndStart in CSE"
    systemctl daemon-reload
    systemctl disable node-exporter.service node-exporter-restart.service node-exporter-restart.path || exit 1

    # Create skip sentinel file to indicate node-exporter was installed from VHD
    mkdir -p /etc/node-exporter.d
    touch /etc/node-exporter.d/skip_vhd_node_exporter
    chmod 644 /etc/node-exporter.d/skip_vhd_node_exporter

    echo "  - node-exporter version ${version}" >> "${VHD_LOGS_FILEPATH}"
}
