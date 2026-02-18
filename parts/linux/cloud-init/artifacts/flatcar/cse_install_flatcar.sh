#!/bin/bash

stub() {
    echo "${FUNCNAME[1]} stub"
}

installDeps() {
    stub
}

installCriCtlPackage() {
    stub
}

ensureRunc() {
    stub
}

removeNvidiaRepos() {
    stub
}

cleanUpGPUDrivers() {
    rm -Rf $GPU_DEST /opt/gpu
}

# Pulls a sysext image from a registry and installs it via systemd-sysext.
# Args: $1 = image path within the registry (e.g., "acl/gpu/cuda-open")
installSysext() {
    local registry="acldevsysext.azurecr.io"
    local image_path="${1}"
    local sysext_dir="/etc/extensions"

    if [ -z "${image_path}" ]; then
        echo "ERROR: image_path argument is required"
        return 1
    fi

    # Read VERSION_ID from /etc/os-release
    source /etc/os-release
    if [ -z "${VERSION_ID}" ]; then
        echo "ERROR: VERSION_ID not found in /etc/os-release"
        return 1
    fi

    local image_ref="${registry}/${image_path}:${VERSION_ID}"
    echo "Pulling sysext image: ${image_ref}"

    # Pull into a temp directory because oras preserves the artifact's internal
    # directory structure (e.g. __build__/images/images/amd64-usr/latest/cuda-open.raw).
    # systemd-sysext only inspects top-level entries in /etc/extensions/, so we
    # must move the .raw file(s) up after pulling.
    local pull_dir="${sysext_dir}/oras-pull-tmp"
    mkdir -p "${pull_dir}"
    if ! retrycmd_pull_from_registry_with_oras 10 5 "${pull_dir}" "${image_ref}"; then
        echo "ERROR: Failed to pull sysext image ${image_ref}"
        rm -rf "${pull_dir}"
        return 1
    fi

    # Move .raw files from the nested oras output to /etc/extensions/
    local found_raw=false
    while IFS= read -r raw_file; do
        local base_name
        base_name=$(basename "${raw_file}")
        echo "Moving ${raw_file} to ${sysext_dir}/${base_name}"
        mv "${raw_file}" "${sysext_dir}/${base_name}"
        found_raw=true
    done < <(find "${pull_dir}" -name '*.raw' -type f)

    rm -rf "${pull_dir}"

    if [ "${found_raw}" = false ]; then
        echo "ERROR: No .raw sysext image found after pulling ${image_ref}"
        return 1
    fi

    # Log current sysext contents for debugging
    find /etc/extensions/ -name '*.raw' -ls

    # Reload systemd-sysext to pick up the new extension
    systemd-sysext refresh
    echo "Successfully installed ${image_path} sysext for ACL version ${VERSION_ID}"
}

downloadGPUDriverSysext() {
    installSysext "acl/gpu/cuda-open"
}

installNvidiaContainerToolkitSysext() {
    installSysext "acl/gpu/nvidia-container-toolkit"
}

installNvidiaFabricManagerSysext() {
    installSysext "acl/gpu/nvidia-fabric-manager"
}

enableNvidiaPersistenceMode() {
    PERSISTENCED_SERVICE_FILE_PATH="/etc/systemd/system/nvidia-persistenced.service"
    touch ${PERSISTENCED_SERVICE_FILE_PATH}
    cat << EOF > ${PERSISTENCED_SERVICE_FILE_PATH}
[Unit]
Description=NVIDIA Persistence Daemon
Wants=syslog.target

[Service]
Type=forking
ExecStart=/usr/bin/nvidia-persistenced --verbose
ExecStopPost=/bin/rm -rf /var/run/nvidia-persistenced
Restart=always
TimeoutSec=300

[Install]
WantedBy=multi-user.target
EOF

    systemctl enable nvidia-persistenced.service || exit 1
    systemctl restart nvidia-persistenced.service || exit 1
}

installToolFromLocalRepo() {
    stub
    return 1
}

#EOF
