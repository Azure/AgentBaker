#!/bin/bash

echo "Sourcing cse_install_distro.sh for Mariner"

removeContainerd() {
    retrycmd_if_failure 10 5 60 dnf remove -y moby-containerd
}

installDeps() {
    dnf_makecache || exit $ERR_APT_UPDATE_TIMEOUT
    dnf_update || exit $ERR_APT_DIST_UPGRADE_TIMEOUT
    for dnf_package in blobfuse ca-certificates check-restart cifs-utils cloud-init-azure-kvp conntrack-tools cracklib dnf-automatic ebtables ethtool fuse git inotify-tools iotop iproute ipset iptables jq kernel-devel logrotate lsof nmap-ncat nfs-utils pam pigz psmisc rsyslog socat sysstat traceroute util-linux xz zip; do
      if ! dnf_install 30 1 600 $dnf_package; then
        exit $ERR_APT_INSTALL_TIMEOUT
      fi
    done

    if [[ $OS_VERSION == "2.0" ]]; then
      for dnf_package in apparmor-parser libapparmor blobfuse2 nftables iscsi-initiator-utils; do
        if ! dnf_install 30 1 600 $dnf_package; then
          exit $ERR_APT_INSTALL_TIMEOUT
        fi
      done
    fi
}

installKataDeps() {
    if [[ $OS_VERSION != "1.0" ]]; then
      if ! dnf_install 30 1 600 kata-packages-host; then
        exit $ERR_APT_INSTALL_TIMEOUT
      fi
    fi
}

downloadGPUDrivers() {
    #
    #
    KERNEL_VERSION=$(uname -r | sed 's/-/./g')
    CUDA_VERSION="*_${KERNEL_VERSION}*"

    if ! dnf_install 30 1 600 cuda-${CUDA_VERSION}; then
      exit $ERR_APT_INSTALL_TIMEOUT
    fi
}

createNvidiaSymlinkToAllDeviceNodes() {
    NVIDIA_DEV_CHAR="/lib/udev/rules.d/71-nvidia-dev-char.rules"
    touch "${NVIDIA_DEV_CHAR}"
    cat << EOF > "${NVIDIA_DEV_CHAR}"
ACTION=="add", DEVPATH=="/bus/pci/drivers/nvidia", RUN+="/usr/bin/nvidia-ctk system create-dev-char-symlinks --create-all"
EOF

    /usr/bin/nvidia-ctk system create-dev-char-symlinks --create-all
}

installNvidiaFabricManager() {
    NVIDIA_DRIVER_VERSION=$(cut -d - -f 2 <<< "$(rpm -qa cuda)")
    for nvidia_package in nvidia-fabric-manager-${NVIDIA_DRIVER_VERSION} nvidia-fabric-manager-devel-${NVIDIA_DRIVER_VERSION}; do
      if ! dnf_install 30 1 600 $nvidia_package; then
        exit $ERR_APT_INSTALL_TIMEOUT
      fi
    done
}

installNvidiaContainerRuntime() {
    MARINER_NVIDIA_CONTAINER_RUNTIME_VERSION="3.13.0"
    MARINER_NVIDIA_CONTAINER_TOOLKIT_VERSION="1.13.5"
    
    for nvidia_package in nvidia-container-runtime-${MARINER_NVIDIA_CONTAINER_RUNTIME_VERSION} nvidia-container-toolkit-${MARINER_NVIDIA_CONTAINER_TOOLKIT_VERSION} nvidia-container-toolkit-base-${MARINER_NVIDIA_CONTAINER_TOOLKIT_VERSION} libnvidia-container-tools-${MARINER_NVIDIA_CONTAINER_TOOLKIT_VERSION} libnvidia-container1-${MARINER_NVIDIA_CONTAINER_TOOLKIT_VERSION}; do
      if ! dnf_install 30 1 600 $nvidia_package; then
        exit $ERR_APT_INSTALL_TIMEOUT
      fi
    done
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

[Install]
WantedBy=multi-user.target
EOF

    systemctl enable nvidia-persistenced.service || exit 1
    systemctl restart nvidia-persistenced.service || exit 1
}

installStandaloneContainerd() {
    CONTAINERD_VERSION=$1
    #overwrite the passed containerd_version since mariner uses only 1 version now which is different than ubuntu's
    CONTAINERD_VERSION="1.3.4"
    CURRENT_VERSION=$(containerd -version | cut -d " " -f 3 | sed 's|v||' | cut -d "+" -f 1)
    
    if semverCompare ${CURRENT_VERSION:-"0.0.0"} ${CONTAINERD_VERSION}; then
        echo "currently installed containerd version ${CURRENT_VERSION} is greater than (or equal to) target base version ${CONTAINERD_VERSION}. skipping installStandaloneContainerd."
    else
        echo "installing containerd version ${CONTAINERD_VERSION}"
        removeContainerd
        if ! dnf_install 30 1 600 moby-containerd; then
          exit $ERR_CONTAINERD_INSTALL_TIMEOUT
        fi
    fi

    if [[ -f /etc/containerd/config.toml.rpmsave ]]; then
        mv /etc/containerd/config.toml.rpmsave /etc/containerd/config.toml
    fi

}

cleanUpGPUDrivers() {
    rm -Rf $GPU_DEST /opt/gpu
}

downloadContainerdFromVersion() {
    echo "downloadContainerdFromVersion not implemented for mariner"
}

downloadContainerdFromURL() {
    echo "downloadContainerdFromURL not implemented for mariner"
}

#EOF
