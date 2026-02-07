#!/bin/bash

# NPD installation helpers used during VHD builds. This file is sourced by
# install-dependencies.sh to keep NPD logic self-contained.

# Download the NPD package for the current distro and install it into the VHD.
download_and_install_npd_package() {
    local download_dir=$1
    local package_url=$2
    local package_name=$3

    mkdir -p "${download_dir}"

    retrycmd_curl_file 10 5 60 "${download_dir}/${package_name}" "${package_url}" || exit $ERR_NPD_INSTALL_TIMEOUT

    if [ "$OS" = "$UBUNTU_OS_NAME" ]; then
        apt_get_install 30 1 600 "${download_dir}/${package_name}" || exit $ERR_NPD_INSTALL_TIMEOUT
        # Create symlink to /opt/bin for Flatcar sysext compatibility
        mkdir -p /opt/bin
        ln -snf /usr/bin/node-problem-detector /opt/bin/node-problem-detector
        return 0
    fi

    if [ "$OS" = "$AZURELINUX_OS_NAME" ]; then
        if ! dnf_install 30 1 600 "${download_dir}/${package_name}"; then
            echo "ERROR: dnf_install failed for ${package_name} with exit code $?"
            exit $ERR_NPD_INSTALL_TIMEOUT
        fi
        # Create symlink to /opt/bin for Flatcar sysext compatibility
        mkdir -p /opt/bin
        ln -snf /usr/bin/node-problem-detector /opt/bin/node-problem-detector
        echo "Successfully installed NPD package: ${package_name}"
        return 0
    fi

    echo "WARNING: NPD package install skipped for unsupported OS ${OS}"
}

# Ensure config and artifact directories exist with sane permissions.
stage_npd_directories() {
    local config_dir=$1
    local artifacts_dir=$2

    mkdir -p "${config_dir}"
    mkdir -p "${artifacts_dir}"

    # Ensure config directory remains traversable for validation scripts and extensions.
    chmod 755 "${config_dir}" || echo "WARNING: failed to chmod 755 ${config_dir}"
}

# Copy staged NPD artifacts into the destination cache when available.
sync_npd_artifacts_if_present() {
    local source_dir=$1
    local destination_dir=$2

    if [ -d "${source_dir}" ]; then
        echo "Copying NPD files from ${source_dir} to ${destination_dir}"
        cp -r "${source_dir}"/* "${destination_dir}/"
    fi
}

# Populate the NPD config directory with monitor definitions from cache or build source.
copy_npd_config_directories() {
    local artifacts_dir=$1
    local build_source=$2
    local config_dir=$3

    local config_dirs=(
        "custom-plugin-monitor"
        "plugin"
        "system-log-monitor"
        "system-stats-monitor"
    )

    for dir in "${config_dirs[@]}"; do
        if [ -d "${artifacts_dir}/${dir}" ]; then
            echo "Copying NPD config directory: ${dir}"
            cp -r "${artifacts_dir}/${dir}" "${config_dir}/"
            continue
        fi

        if [ -d "${build_source}/${dir}" ]; then
            echo "Copying NPD config directory from build source: ${dir}"
            cp -r "${build_source}/${dir}" "${config_dir}/"
        fi
    done
}

# Guarantee the skip_vhd_npd file is present so provisioning can short-circuit NPD setup.
# The skip file is created dynamically during VHD build, not checked into version control.
create_npd_skip_file() {
    local config_dir=$1
    local NPD_SKIP_FILE="skip_vhd_npd"
    local skip_file_path="${config_dir}/${NPD_SKIP_FILE}"

    echo "Creating ${NPD_SKIP_FILE} at ${skip_file_path}"
    touch "${skip_file_path}"
    chmod 644 "${skip_file_path}"

    if [ ! -f "${skip_file_path}" ]; then
        echo "ERROR: ${NPD_SKIP_FILE} not found after creation"
        exit 1
    fi
    echo "Successfully created and verified ${NPD_SKIP_FILE}"
}

# Normalize permissions on scripts and JSON configs to avoid execution failures.
set_npd_permissions() {
    local config_dir=$1

    if [ -d "${config_dir}/plugin" ]; then
        chmod 755 "${config_dir}/plugin"/*.sh 2>/dev/null || true
    fi

    local json_dirs=(custom-plugin-monitor system-log-monitor system-stats-monitor)
    for dir in "${json_dirs[@]}"; do
        if [ -d "${config_dir}/${dir}" ]; then
            chmod 644 "${config_dir}/${dir}"/*.json 2>/dev/null || true
        fi
    done
}

# Add compatibility symlinks so legacy npd-* binaries resolve to the Azure Linux counterparts.
ensure_npd_counter_entrypoints() {
    local os_name=${1:-}

    if [ "${os_name}" != "${AZURELINUX_OS_NAME}" ]; then
        return 0
    fi

    # Azure Linux packages expose /usr/bin/*counter instead of the legacy /usr/bin/npd-*-counter path referenced by shipped NPD configs.
    # So we're creating a symlink as needed.
    # Note that the order of the references in these arrays needs to align if we add more later for some reason.
    local targets=(
        "${NPD_LOG_COUNTER_BINARY_PATH:-/usr/bin/log-counter}"
        "${NPD_HEALTH_COUNTER_BINARY_PATH:-/usr/bin/health-counter}"
    )
    local links=(
        "${NPD_LOG_COUNTER_LINK_PATH:-/usr/bin/npd-log-counter}"
        "${NPD_HEALTH_COUNTER_LINK_PATH:-/usr/bin/npd-health-counter}"
    )
    local aliases=(
        "npd-log-counter"
        "npd-health-counter"
    )

    local i
    for i in "${!targets[@]}"; do
        local target_path="${targets[$i]}"
        local link_path="${links[$i]}"
        local alias_name="${aliases[$i]}"

        if [ ! -x "${target_path}" ]; then
            echo "${alias_name} binary not found at ${target_path}; skipping compatibility link"
            continue
        fi

        if [ -L "${link_path}" ]; then
            ln -sfn "${target_path}" "${link_path}"
            continue
        fi

        if [ -e "${link_path}" ]; then
            echo "${alias_name} already present at ${link_path}, skipping compatibility link"
            continue
        fi

        echo "Creating ${alias_name} compatibility symlink at ${link_path}"
        ln -s "${target_path}" "${link_path}"
    done
}

# Verify NPD installation completed successfully with all required artifacts.
verify_npd_installation() {
    local config_dir=$1
    local NPD_SKIP_FILE="skip_vhd_npd"
    local skip_file_path="${config_dir}/${NPD_SKIP_FILE}"

    echo "Verifying NPD installation..."

    # Verify skip_vhd_npd file exists
    if [ -f "${skip_file_path}" ]; then
        echo "Verified: ${NPD_SKIP_FILE} exists at ${skip_file_path}"
        ls -la "${skip_file_path}"
    else
        echo "ERROR: ${NPD_SKIP_FILE} NOT found at ${skip_file_path}"
        echo "Contents of ${config_dir}:"
        ls -la "${config_dir}" || echo "Directory does not exist"
        return 1
    fi

    # Verify node-problem-detector service is registered with systemd
    # Note: We check if the service file exists rather than parsing list-unit-files output
    # because systemd behavior varies across distros when querying specific unit files
    if systemctl cat node-problem-detector.service &>/dev/null; then
        echo "Verified: node-problem-detector.service is registered with systemd"
        systemctl status node-problem-detector.service --no-pager || true
    else
        echo "ERROR: node-problem-detector.service not found in systemd unit files"
        echo "This will cause node provisioning to fail. Check that systemctl daemon-reload was called."
        echo "Debug: Checking if service file exists and systemd state:"
        ls -la /etc/systemd/system/node-problem-detector.service || echo "Service file not found"
        systemctl list-unit-files | grep -i "node-problem-detector" || echo "Service not in systemd registry"
        return 1
    fi

    echo "NPD installation verification passed"
    return 0
}

# Install Node Problem Detector during VHD build. This function is only used
# during VHD generation (not at provisioning time). CSE will reload the service
# later so runtime GPU hooks can reconfigure NPD as needed.
# NOTE: packer_source.sh handles copying:
#   - startup script to /opt/bin/node-problem-detector-startup.sh
#   - service file to /etc/systemd/system/node-problem-detector.service
installNodeProblemDetector() {
    local downloadDir=$1
    local evaluatedURL=$2
    local npdName=$3

    echo "Installing Node Problem Detector."

    download_and_install_npd_package "${downloadDir}" "${evaluatedURL}" "${npdName}"

    local NPD_CONFIG_DIR="/etc/node-problem-detector.d"
    local NPD_ARTIFACTS_DIR="/opt/azure/containers/node-problem-detector"
    local NPD_BUILD_SOURCE="/home/packer/node-problem-detector"

    stage_npd_directories "${NPD_CONFIG_DIR}" "${NPD_ARTIFACTS_DIR}"

    sync_npd_artifacts_if_present "${NPD_BUILD_SOURCE}" "${NPD_ARTIFACTS_DIR}"
    copy_npd_config_directories "${NPD_ARTIFACTS_DIR}" "${NPD_BUILD_SOURCE}" "${NPD_CONFIG_DIR}"

    # Create skip file to indicate NPD was installed from VHD
    create_npd_skip_file "${NPD_CONFIG_DIR}"

    set_npd_permissions "${NPD_CONFIG_DIR}"
    ensure_npd_counter_entrypoints "$OS"

    # Reload systemd to pick up the service file copied by packer_source.sh
    systemctl daemon-reload
    systemctl disable node-problem-detector

    # Verify installation succeeded before returning
    verify_npd_installation "${NPD_CONFIG_DIR}" || exit $ERR_NPD_INSTALL_TIMEOUT

    echo "Node Problem Detector installed and verified"
}
