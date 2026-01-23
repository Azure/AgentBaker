#!/bin/bash

stub() {
    echo "${FUNCNAME[1]} stub"
}

installDeps() {
    # We must configure containerd before running Docker commands.
    installStandaloneContainerd ""

    local ARCH; ARCH="$(uname -m)"
    local BLOB_CSI_IMAGE="mcr.microsoft.com/oss/v2/kubernetes-csi/blob-csi:v1.27.1"

    docker run --rm \
        -v /var/bin:/host/var/bin  \
        --env DISTRIBUTION=flatcar \
        --env ARCH="${ARCH}" \
        --env INSTALL_BLOBFUSE2=true  \
        --env INSTALL_BLOBFUSE_PROXY=false  \
        --entrypoint /blobfuse-proxy/install-proxy-rhcos.sh \
        "${BLOB_CSI_IMAGE}"
    docker image rm "${BLOB_CSI_IMAGE}"
}

installCriCtlPackage() {
    stub
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
