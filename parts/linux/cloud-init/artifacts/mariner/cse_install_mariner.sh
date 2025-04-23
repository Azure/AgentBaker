#!/bin/bash

removeContainerd() {
    containerdPackageName="containerd"
    if [[ $OS_VERSION == "2.0" ]]; then
        containerdPackageName="moby-containerd"
    fi
    retrycmd_if_failure 10 5 60 dnf remove -y $containerdPackageName
}

installDeps() {
    # The nftables package turns on a service by default that tries to load config files,
    # but the stock config files in the package have no uncommented lines and make the service
    # fail to start. Masking it as it's not used, and the stop action of "flush tables" can
    # result in rules getting cleared unexpectedly. Azure Linux 3 fixes this, so we only need
    # this in 2.0.
    if [[ $OS_VERSION == "2.0" ]]; then
      systemctl --now mask nftables.service || exit $ERR_SYSTEMCTL_MASK_FAIL
    fi

    # Install the package repo for the specific OS version.
    # AzureLinux 3.0 uses the azurelinux-repos-cloud-native repo
    # Other OS, e.g., Mariner 2.0 uses the mariner-repos-cloud-native repo
    if [[ $OS_VERSION == "3.0" ]]; then
      echo "Installing azurelinux-repos-cloud-native"
      dnf_install 30 1 600 azurelinux-repos-cloud-native
    else
      echo "Installing mariner-repos-cloud-native"
      dnf_install 30 1 600 mariner-repos-cloud-native
    fi
    
    dnf_makecache || exit $ERR_APT_UPDATE_TIMEOUT
    dnf_update || exit $ERR_APT_DIST_UPGRADE_TIMEOUT
    for dnf_package in ca-certificates check-restart cifs-utils cloud-init-azure-kvp conntrack-tools cracklib dnf-automatic ebtables ethtool fuse git inotify-tools iotop iproute ipset iptables jq kernel-devel logrotate lsof nmap-ncat nfs-utils pam pigz psmisc rsyslog socat sysstat traceroute util-linux xz zip blobfuse2 nftables iscsi-initiator-utils; do
      if ! dnf_install 30 1 600 $dnf_package; then
        exit $ERR_APT_INSTALL_TIMEOUT
      fi
    done

    # install 2.0 specific packages
    # apparmor related packages and the blobfuse package are not available in AzureLinux 3.0
    if [[ $OS_VERSION == "2.0" ]]; then
      for dnf_package in apparmor-parser libapparmor blobfuse; do
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

installCriCtlPackage() {
  version="${1:-}"
  packageName="kubernetes-cri-tools-${version}"
  if [[ -z $version ]]; then
    echo "Error: No version specified for kubernetes-cri-tools package but it is required. Exiting with error."
  fi
  echo "Installing ${packageName} with dnf"
  dnf_install 30 1 600 ${packageName} || exit 1
}

downloadGPUDrivers() {
    # Mariner CUDA rpm name comes in the following format:
    #
    # 1. NVIDIA proprietary driver:
    # cuda-%{nvidia gpu driver version}_%{kernel source version}.%{kernel release version}.{mariner rpm postfix}
    #
    # 2. NVIDIA OpenRM driver:
    # cuda-open-%{nvidia gpu driver version}_%{kernel source version}.%{kernel release version}.{mariner rpm postfix}
    #
    # The proprietary driver will be used here in order to support older NVIDIA GPU SKUs like V100
    # Before installing cuda, check the active kernel version (uname -r) and use that to determine which cuda to install
    KERNEL_VERSION=$(uname -r | sed 's/-/./g')
    CUDA_PACKAGE=$(dnf repoquery --available "cuda*" | grep -E "cuda-[0-9]+.*_$KERNEL_VERSION" | sort -V | tail -n 1)

    if ! dnf_install 30 1 600 ${CUDA_PACKAGE}; then
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

installNvidiaContainerToolkit() {
    MARINER_NVIDIA_CONTAINER_TOOLKIT_VERSION=$(jq -r '.Packages[] | select(.name == "nvidia-container-toolkit") | .downloadURIs.azurelinux.current.versionsV2[0].latestVersion' $COMPONENTS_FILEPATH)

    # Check if the version is empty and set the default if needed
    if [ -z "$MARINER_NVIDIA_CONTAINER_TOOLKIT_VERSION" ]; then
      echo "nvidia-container-toolkit not found in components.json" # Expected for older VHD with new CSE
      MARINER_NVIDIA_CONTAINER_TOOLKIT_VERSION="1.16.2"
    fi

    # The following packages need to be installed in this sequence because:
    # - libnvidia-container packages are required by nvidia-container-toolkit
    # - nvidia-container-toolkit-base provides nvidia-ctk that is used to generate the nvidia container runtime config 
    #   during the posttrans phase of nvidia-container-toolkit package installation
    for nvidia_package in libnvidia-container1-${MARINER_NVIDIA_CONTAINER_TOOLKIT_VERSION} libnvidia-container-tools-${MARINER_NVIDIA_CONTAINER_TOOLKIT_VERSION} nvidia-container-toolkit-base-${MARINER_NVIDIA_CONTAINER_TOOLKIT_VERSION} nvidia-container-toolkit-${MARINER_NVIDIA_CONTAINER_TOOLKIT_VERSION}; do
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
    local desiredVersion="${1:-}"
    #e.g., desiredVersion will look like this 1.6.26-5.cm2
    # azure-built runtimes have a "+azure" suffix in their version strings (i.e 1.4.1+azure). remove that here.
    # check if containerd command is available before running it
    if command -v containerd &> /dev/null; then
        CURRENT_VERSION=$(containerd -version | cut -d " " -f 3 | sed 's|v||' | cut -d "+" -f 1)    
    fi
    # v1.4.1 is our lowest supported version of containerd    
    if semverCompare ${CURRENT_VERSION:-"0.0.0"} ${desiredVersion}; then
        echo "currently installed containerd version ${CURRENT_VERSION} is greater than (or equal to) target base version ${desiredVersion}. skipping installStandaloneContainerd."
    else
        echo "installing containerd version ${desiredVersion}"
        removeContainerd || exit $ERR_CONTAINERD_INSTALL_TIMEOUT 
        containerdPackageName="containerd-${desiredVersion}"
        if [[ $OS_VERSION == "2.0" ]]; then
            containerdPackageName="moby-containerd-${desiredVersion}"
        fi
        if [[ $OS_VERSION == "3.0" ]]; then
            containerdPackageName="containerd2-${desiredVersion}"
        fi
        
        # TODO: tie runc to r92 once that's possible on Mariner's pkg repo and if we're still using v1.linux shim
        if ! dnf_install 30 1 600 $containerdPackageName; then
            exit $ERR_CONTAINERD_INSTALL_TIMEOUT
        fi
    fi

    # Workaround to restore the CSE configuration after containerd has been installed from the package server.
    if [[ -f /etc/containerd/config.toml.rpmsave ]]; then
        mv /etc/containerd/config.toml.rpmsave /etc/containerd/config.toml
    fi

}

ensureRunc() {
  echo "Mariner Runc is included in the Mariner base image or containerd installation. Skipping downloading and installing Runc"
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
