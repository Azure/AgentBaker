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

    # Validate DNS resolution and TLS connectivity before pulling.
    echo "Checking DNS resolution for ${registry}..."
    if ! nslookup "${registry}" 2>&1; then
        echo "ERROR: DNS resolution failed for ${registry}"
        return 1
    fi

    echo "Checking TLS connectivity to ${registry}..."
    local http_code
    http_code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 10 "https://${registry}/v2/" 2>&1)
    if [ "$?" -ne 0 ]; then
        echo "ERROR: TLS connection to ${registry} failed"
    fi
    echo "Registry ${registry} returned HTTP ${http_code}"

    # Validate the registry is anonymously reachable before attempting to pull.
    if ! retrycmd_can_oras_ls_acr_anonymously 3 5 "${registry}"; then
        echo "ERROR: Registry ${registry} is not anonymously reachable. Ensure anonymous pull is enabled."
    fi

    # Pull the sysext raw image using the shared ORAS retry helper.
    # retrycmd_pull_from_registry_with_oras passes --registry-config to avoid
    # the "$HOME is not defined" error that occurs in minimal CSE environments.
    if ! retrycmd_pull_from_registry_with_oras 10 5 "${sysext_dir}" "${image_ref}"; then
        echo "ERROR: Failed to pull sysext image ${image_ref}"
        return 1
    fi

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
