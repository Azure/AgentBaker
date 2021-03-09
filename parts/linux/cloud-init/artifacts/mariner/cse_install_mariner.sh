#!/bin/bash

echo "Sourcing cse_install_distro.sh for Mariner"

removeContainerd() {
    retrycmd_if_failure 10 5 60 dnf remove -y moby-containerd
}

installDeps() {
    dnf_makecache || exit $ERR_APT_UPDATE_TIMEOUT
    dnf_update || exit $ERR_APT_DIST_UPGRADE_TIMEOUT
    for apt_package in ca-certificates cifs-utils cracklib ebtables ethtool fuse git iotop iproute ipset iptables jq pam nfs-utils socat sysstat traceroute util-linux xz zip; do
      if ! dnf_install 30 1 600 $apt_package; then
        exit $ERR_APT_INSTALL_TIMEOUT
      fi
    done
}

installGPUDrivers() {
    echo "GPU drivers not yet supported for Mariner"
    exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
}

installSGXDrivers() {
    echo "SGX drivers not yet supported for Mariner"
    exit $ERR_SGX_DRIVERS_START_FAIL
}

# CSE+VHD can dictate the containerd version, users don't care as long as it works
installStandaloneContainerd() {
    # azure-built runtimes have a "+azure" suffix in their version strings (i.e 1.4.1+azure). remove that here.
    CURRENT_VERSION=$(containerd -version | cut -d " " -f 3 | sed 's|v||' | cut -d "+" -f 1)
    # v1.4.1 is our lowest supported version of containerd
    local CONTAINERD_VERSION="1.4.3"
    if semverCompare ${CURRENT_VERSION:-"0.0.0"} ${CONTAINERD_VERSION}; then
        echo "currently installed containerd version ${CURRENT_VERSION} is greater than (or equal to) target base version ${CONTAINERD_VERSION}. skipping installStandaloneContainerd."
    else
        echo "installing containerd version ${CONTAINERD_VERSION}"
        removeContainerd
        if ! dnf_install 30 1 600 moby-containerd; then
          exit $ERR_CONTAINERD_INSTALL_TIMEOUT
        fi
    fi
}

cleanUpGPUDrivers() {
    rm -Rf $GPU_DEST
}

#EOF
