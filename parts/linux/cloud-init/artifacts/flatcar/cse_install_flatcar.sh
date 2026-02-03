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

installCNI() {
    #This is an old version because dalec needs to be updated for osguard/flatcar
    # https://github.com/Azure/dalec-build-defs/blob/main/specs/containernetworking-plugins/containernetworking-plugins-1.9.0.yaml
    retrycmd_get_tarball 120 5 "${CNI_DOWNLOADS_DIR}/refcni.tar.gz" "https://${PACKAGE_DOWNLOAD_BASE_URL}/cni-plugins/v1.6.2/binaries/cni-plugins-linux-amd64-v1.6.2.tgz" || exit $ERR_CNI_DOWNLOAD_TIMEOUT
    extract_tarball "${CNI_DOWNLOADS_DIR}/refcni.tar.gz" "$CNI_BIN_DIR"
}


# CSE+VHD can dictate the containerd version, users don't care as long as it works
installStandaloneContainerd() {
    local desiredVersion="${1:-}"
    CURRENT_VERSION=$(containerd -version | cut -d " " -f 3)
    echo "currently installed containerd version: ${CURRENT_VERSION}. Desired version ${desiredVersion}. Skipping installStandaloneContainerd on Flatcar."
    if [ ! -f "/etc/containerd/config.toml" ]; then
        mkdir -p /etc/containerd
        cp /usr/share/containerd/config.toml /etc/containerd/config.toml
        systemctl restart containerd || echo "Failed to restart containerd"
    fi
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

installToolFromLocalRepo() {
    stub
    return 1
}

#EOF
