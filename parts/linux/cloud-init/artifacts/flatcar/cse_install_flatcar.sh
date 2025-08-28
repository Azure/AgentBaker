#!/bin/bash

stub() {
    echo "${FUNCNAME[1]} stub"
}

downloadSysextFromVersion() {
    local sysextName=$1
    local sysextURL=$2
    local downloadDir=${3:-"/opt/${sysextName}/downloads"}

    local tries
    while ! timeout 60 oras pull --output "${downloadDir}" "${sysextURL}"; do
        [[ $(( ++tries )) -eq 120 ]] && exit "${ERR_ORAS_PULL_SYSEXT_FAIL}"
        sleep 5
    done

    echo "Succeeded to download ${sysextName} from ${sysextURL}"
}

mergeSysexts() {
    local sysextArch
    sysextArch=$(getSystemdArch)

    while [[ ${1-} ]]; do
        local sysextName=$1 sysextVersion=$2
        ln -snf "/opt/${sysextName}/downloads/${sysextName}-${sysextVersion}-${sysextArch}.raw" "/etc/extensions/${sysextName}.raw"
        shift 2
    done

    systemd-sysext refresh
}

installDeps() {
    stub
}

installCriCtlPackage() {
    stub
}

installKubeletKubectlFromPkg() {
    mergeSysexts kubelet "$1" kubectl "$1"
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

cleanUpGPUDrivers() {
    rm -Rf $GPU_DEST /opt/gpu
}

#EOF
