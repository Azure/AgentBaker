#!/bin/bash

echo "Sourcing cse_install_distro.sh for Mariner"

removeContainerd() {
    retrycmd_if_failure 10 5 60 dnf remove -y moby-containerd
}

installDeps() {
    dnf_makecache || exit $ERR_APT_UPDATE_TIMEOUT
    dnf_update || exit $ERR_APT_DIST_UPGRADE_TIMEOUT
    for dnf_package in blobfuse2 ca-certificates check-restart cifs-utils cloud-init-azure-kvp conntrack-tools cracklib dnf-automatic ebtables ethtool fuse git inotify-tools iotop iproute ipset iptables jq kernel-devel logrotate lsof nmap-ncat nfs-utils pam pigz psmisc rsyslog socat sysstat traceroute util-linux xz zip; do
      if ! dnf_install 30 1 600 $dnf_package; then
        exit $ERR_APT_INSTALL_TIMEOUT
      fi
    done

    # install additional apparmor deps for 2.0;
    #if [[ $OS_VERSION == "3.0" ]]; then
    #  for dnf_package in apparmor-parser libapparmor blobfuse2 nftables iscsi-initiator-utils; do
    #    if ! dnf_install 30 1 600 $dnf_package; then
    #      exit $ERR_APT_INSTALL_TIMEOUT
    #    fi
    #  done
    #fi
}

installKataDeps() {
    if [[ $OS_VERSION != "1.0" ]]; then
      #if ! dnf_install 30 1 600 kata-packages-host; then
      #  exit $ERR_APT_INSTALL_TIMEOUT
      #fi
      echo "[temp] install kata-packages-host"
      wget "https://mitchzhu.blob.core.windows.net/mariner3/cloud-hypervisor-37.0-1.azl3.x86_64.rpm"
      wget "https://mitchzhu.blob.core.windows.net/mariner3/cloud-hypervisor-cvm-32.0.314-2000.geb595874.azl3.x86_64.rpm"
      # wget "https://mitchzhu.blob.core.windows.net/mariner3/hvloader-1.0.1-1.azl3.x86_64.rpm"
      wget "https://mitchzhu.blob.core.windows.net/mariner3/kata-containers-3.2.0.azl0-2.azl3.x86_64.rpm"
      wget "https://mitchzhu.blob.core.windows.net/mariner3/kata-containers-cc-3.2.0.azl0-3.azl3.x86_64.rpm"
      wget "https://mitchzhu.blob.core.windows.net/mariner3/kernel-mshv-5.15.126.mshv9-4.azl3.x86_64.rpm"
      wget "https://mitchzhu.blob.core.windows.net/mariner3/kernel-uvm-6.1.0.mshv16-2.azl3.x86_64.rpm"
      wget "https://mitchzhu.blob.core.windows.net/mariner3/moby-containerd-cc-1.7.7-2.azl3.x86_64.rpm"
      wget "https://mitchzhu.blob.core.windows.net/mariner3/mshv-25941.1000.230825-1352.1.x86_64.rpm"
      wget "https://mitchzhu.blob.core.windows.net/mariner3/mshv-bootloader-lx-25941.1000.230825-1352.1.x86_64.rpm"

      dnf install -y mshv-bootloader-lx-25941.1000.230825-1352.1.x86_64.rpm
      dnf install -y mshv-25941.1000.230825-1352.1.x86_64.rpm
      dnf install -y hvloader
      dnf install -y kernel-mshv-5.15.126.mshv9-4.azl3.x86_64.rpm
      dnf install -y cloud-hypervisor-37.0-1.azl3.x86_64.rpm
      dnf install -y cloud-hypervisor-cvm-32.0.314-2000.geb595874.azl3.x86_64.rpm
      dnf install -y kernel-uvm-6.1.0.mshv16-2.azl3.x86_64.rpm
      dnf install -y kata-containers-3.2.0.azl0-2.azl3.x86_64.rpm
      dnf install -y kata-containers-cc-3.2.0.azl0-3.azl3.x86_64.rpm
      dnf install -y moby-containerd-cc-1.7.7-2.azl3.x86_64.rpm
    fi
}

downloadGPUDrivers() {
    # Mariner CUDA rpm name comes in the following format:
    #
    # cuda-%{nvidia gpu driver version}_%{kernel source version}.%{kernel release version}.{mariner rpm postfix}
    #
    # Before installing cuda, check the active kernel version (uname -r) and use that to determine which cuda to install
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
# This will create /dev/char symlinks to all device nodes
ACTION=="add", DEVPATH=="/bus/pci/drivers/nvidia", RUN+="/usr/bin/nvidia-ctk system create-dev-char-symlinks --create-all"
EOF

    /usr/bin/nvidia-ctk system create-dev-char-symlinks --create-all
}

installNvidiaFabricManager() {
    # Check the NVIDIA driver version installed and install nvidia-fabric-manager
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

# CSE+VHD can dictate the containerd version, users don't care as long as it works
installStandaloneContainerd() {
    CONTAINERD_VERSION=$1
    #overwrite the passed containerd_version since mariner uses only 1 version now which is different than ubuntu's
    CONTAINERD_VERSION="1.3.4"
    # azure-built runtimes have a "+azure" suffix in their version strings (i.e 1.4.1+azure). remove that here.
    CURRENT_VERSION=$(containerd -version | cut -d " " -f 3 | sed 's|v||' | cut -d "+" -f 1)
    # v1.4.1 is our lowest supported version of containerd
    
    if semverCompare ${CURRENT_VERSION:-"0.0.0"} ${CONTAINERD_VERSION}; then
        echo "currently installed containerd version ${CURRENT_VERSION} is greater than (or equal to) target base version ${CONTAINERD_VERSION}. skipping installStandaloneContainerd."
    else
        echo "installing containerd version ${CONTAINERD_VERSION}"
        removeContainerd
        # TODO: tie runc to r92 once that's possible on Mariner's pkg repo and if we're still using v1.linux shim
        if ! dnf_install 30 1 600 moby-containerd; then
          exit $ERR_CONTAINERD_INSTALL_TIMEOUT
        fi
    fi

    # Workaround to restore the CSE configuration after containerd has been installed from the package server.
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
