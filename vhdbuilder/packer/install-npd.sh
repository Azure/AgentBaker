#!/bin/bash

# NPD installation helpers used during VHD builds. This file is sourced by
# install-dependencies.sh to keep NPD logic self-contained.

# Install the NPD package for the current distro from the package manager.
install_npd_package() {
    local version=$1

    if [ "$OS" = "$UBUNTU_OS_NAME" ]; then
        apt_get_install 30 1 600 "node-problem-detector-kubernetes=${version}" || exit $ERR_NPD_INSTALL_TIMEOUT
    elif [ "$OS" = "$AZURELINUX_OS_NAME" ]; then
        dnf_install 30 1 600 "node-problem-detector-${version}" || exit $ERR_NPD_INSTALL_TIMEOUT
    else
        echo "WARNING: NPD package install skipped for unsupported OS ${OS}"
        return 1
    fi

    mkdir -p /opt/bin
    ln -snf /usr/bin/node-problem-detector /opt/bin/node-problem-detector
    echo "Successfully installed NPD version ${version}"
}

# Copy NPD monitor configs from the build source into the config directory.
# Prefers artifacts_dir, falls back to build_source for each subdirectory.
copy_npd_configs() {
    local artifacts_dir=$1
    local build_source=$2
    local config_dir=$3

    for dir in custom-plugin-monitor plugin system-log-monitor system-stats-monitor; do
        if [ -d "${artifacts_dir}/${dir}" ]; then
            cp -r "${artifacts_dir}/${dir}" "${config_dir}/"
        elif [ -d "${build_source}/${dir}" ]; then
            cp -r "${build_source}/${dir}" "${config_dir}/"
        fi
    done
}

# Normalize permissions on plugin scripts and JSON configs.
set_npd_permissions() {
    local config_dir=$1
    chmod 755 "${config_dir}/plugin"/*.sh 2>/dev/null || true
    for dir in custom-plugin-monitor system-log-monitor system-stats-monitor; do
        chmod 644 "${config_dir}/${dir}"/*.json 2>/dev/null || true
    done
}

# On Azure Linux the NPD package ships /usr/bin/log-counter and /usr/bin/health-counter,
# but NPD configs reference /usr/bin/npd-log-counter and /usr/bin/npd-health-counter.
ensure_npd_counter_entrypoints() {
    [ "$OS" = "$AZURELINUX_OS_NAME" ] || return 0
    [ -x /usr/bin/log-counter ] && ln -sfn /usr/bin/log-counter /usr/bin/npd-log-counter
    [ -x /usr/bin/health-counter ] && ln -sfn /usr/bin/health-counter /usr/bin/npd-health-counter
}

# Verify NPD binary and systemd service are in place.
verify_npd_installation() {
    local config_dir=$1

    if [ ! -f "${config_dir}/skip_vhd_npd" ]; then
        echo "ERROR: skip_vhd_npd not found in ${config_dir}"
        return 1
    fi

    if ! systemctl cat node-problem-detector.service &>/dev/null; then
        echo "ERROR: node-problem-detector.service not registered with systemd"
        return 1
    fi
}

# Install Node Problem Detector during VHD build.
# packer_source.sh copies the startup script and service file beforehand.
installNodeProblemDetector() {
    local version=$1
    local NPD_CONFIG_DIR="/etc/node-problem-detector.d"
    local NPD_ARTIFACTS_DIR="/opt/azure/containers/node-problem-detector"
    local NPD_BUILD_SOURCE="/home/packer/node-problem-detector"

    echo "Installing Node Problem Detector version ${version}."

    install_npd_package "${version}"

    mkdir -p "${NPD_CONFIG_DIR}" "${NPD_ARTIFACTS_DIR}"

    # Sync build artifacts into the artifacts cache
    if [ -d "${NPD_BUILD_SOURCE}" ]; then
        cp -r "${NPD_BUILD_SOURCE}"/* "${NPD_ARTIFACTS_DIR}/"
    fi

    copy_npd_configs "${NPD_ARTIFACTS_DIR}" "${NPD_BUILD_SOURCE}" "${NPD_CONFIG_DIR}"

    # Skip file tells CSE that NPD was pre-installed on the VHD
    touch "${NPD_CONFIG_DIR}/skip_vhd_npd"
    chmod 644 "${NPD_CONFIG_DIR}/skip_vhd_npd"

    set_npd_permissions "${NPD_CONFIG_DIR}"
    ensure_npd_counter_entrypoints

    systemctl daemon-reload
    systemctl disable node-problem-detector

    verify_npd_installation "${NPD_CONFIG_DIR}" || exit $ERR_NPD_INSTALL_TIMEOUT
    echo "Node Problem Detector installed and verified"
}
