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

# CSE+VHD can dictate the containerd version, users don't care as long as it works
installStandaloneContainerd() {
    local desiredVersion="${1:-}"
    CURRENT_VERSION=$(containerd -version | cut -d " " -f 3)
    echo "currently installed containerd version: ${CURRENT_VERSION}. Desired version ${desiredVersion}. Skipping installStandaloneContainerd on Flatcar."
}

ensureRunc() {
    stub
}

cleanUpGPUDrivers() {
    rm -Rf $GPU_DEST /opt/gpu
}

#EOF
