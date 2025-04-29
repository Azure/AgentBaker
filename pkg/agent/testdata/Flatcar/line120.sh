#!/bin/bash

echo "Sourcing cse_install_distro.sh for Flatcar"

cleanUpGPUDrivers() {
    rm -Rf $GPU_DEST /opt/gpu
}

installStandaloneContainerd() {
    local desiredVersion="${1:-}"
    CURRENT_VERSION=$(containerd -version | cut -d " " -f 3)
    echo "currently installed containerd version: ${CURRENT_VERSION}. Desired version ${desiredVersion}. Skipping installStandaloneContainerd on Flatcar."
}
#EOF
