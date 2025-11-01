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
        return 0
    fi

    if [ "$OS" = "$MARINER_OS_NAME" ] || [ "$OS" = "$AZURELINUX_OS_NAME" ]; then
        if ! dnf_install 30 1 600 "${download_dir}/${package_name}"; then
            echo "ERROR: dnf_install failed for ${package_name} with exit code $?"
            exit $ERR_NPD_INSTALL_TIMEOUT
        fi
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

# Guarantee the skip_vhd_npd sentinel is present so provisioning can short-circuit NPD setup.
copy_npd_skip_sentinel() {
    local artifacts_dir=$1
    local build_source=$2
    local config_dir=$3
    local sentinel="skip_vhd_npd"

    for search_dir in "${artifacts_dir}" "${build_source}"; do
        if [ -f "${search_dir}/${sentinel}" ]; then
            echo "Copying ${sentinel} sentinel file from ${search_dir}"
            cp "${search_dir}/${sentinel}" "${config_dir}/"
            chmod 644 "${config_dir}/${sentinel}"
            if [ ! -f "${config_dir}/${sentinel}" ]; then
                echo "ERROR: ${sentinel} file not found after copy"
                exit 1
            fi
            echo "Successfully copied and verified ${sentinel} sentinel file"
            return 0
        fi
    done

    echo "WARNING: ${sentinel} sentinel file not found in expected locations"
}

# Install the NPD systemd service and startup script while keeping copies in the cache.
install_npd_systemd_assets() {
    local artifacts_dir=$1
    local startup_src=$2
    local service_src=$3
    local startup_dest="/usr/local/bin/node-problem-detector-startup.sh"
    local service_dest="/etc/systemd/system/node-problem-detector.service"

    cp "${startup_src}" "${startup_dest}"
    chmod 755 "${startup_dest}"

    cp "${service_src}" "${service_dest}"
    cp "${startup_src}" "${artifacts_dir}/"
    cp "${service_src}" "${artifacts_dir}/"
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
    # So we're creating a symlink as needed
    local mappings=(
        "${NPD_LOG_COUNTER_BINARY_PATH:-/usr/bin/log-counter}::${NPD_LOG_COUNTER_LINK_PATH:-/usr/bin/npd-log-counter}::npd-log-counter"
        "${NPD_HEALTH_COUNTER_BINARY_PATH:-/usr/bin/health-counter}::${NPD_HEALTH_COUNTER_LINK_PATH:-/usr/bin/npd-health-counter}::npd-health-counter"
    )

    local mapping
    local target_path
    local link_path
    local alias_name
    for mapping in "${mappings[@]}"; do
        IFS="::" read -r target_path link_path alias_name <<<"${mapping}"

        # Skip empty mappings (e.g. unset env vars)
        if [ -z "${target_path}" ] || [ -z "${link_path}" ]; then
            continue
        fi

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

# Install Node Problem Detector during VHD build. This function is only used
# during VHD generation (not at provisioning time). CSE will reload the service
# later so runtime GPU hooks can reconfigure NPD as needed.
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
    copy_npd_skip_sentinel "${NPD_ARTIFACTS_DIR}" "${NPD_BUILD_SOURCE}" "${NPD_CONFIG_DIR}"

    local NPD_STARTUP_SCRIPT_SRC="/home/packer/node-problem-detector-startup.sh"
    local NPD_SERVICE_SRC="/home/packer/node-problem-detector.service"
    # NOTE: PR #7125 proposes relocating binaries from /usr/local/bin to /opt/bin to
    # align with sysext redirection. When that merges we will need to (1) update
    # NPD_STARTUP_SCRIPT_DEST, (2) ensure the systemd unit matches the new path, and
    # (3) keep the cached copy relationships intact for VHD validation.

    install_npd_systemd_assets "${NPD_ARTIFACTS_DIR}" "${NPD_STARTUP_SCRIPT_SRC}" "${NPD_SERVICE_SRC}"

    set_npd_permissions "${NPD_CONFIG_DIR}"
    ensure_npd_counter_entrypoints "$OS"

    systemctl disable node-problem-detector

    echo "Node Problem Detector installed"
}
