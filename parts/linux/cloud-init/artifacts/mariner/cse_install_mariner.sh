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

    # install additional apparmor deps for 2.0;
    if [[ $OS_VERSION == "2.0" ]]; then
      for dnf_package in apparmor-parser libapparmor blobfuse2 nftables; do
        if ! dnf_install 30 1 600 $dnf_package; then
          exit $ERR_APT_INSTALL_TIMEOUT
        fi
      done
    fi
}

installKataDeps() {
    if [[ $OS_VERSION == "2.0" ]]; then
      for dnf_package in cargo opa parted qemu-img moby-runc python3-devel python3-pip kernel-mshv cloud-hypervisor kata-containers moby-containerd-cc; do
        if ! dnf_install 30 1 600 $dnf_package; then
          exit $ERR_APT_INSTALL_TIMEOUT
        fi
      done

      echo "Copy UVM build pipeline artifacts"
      wget "https://mitchzhu.blob.core.windows.net/public/cloud-hypervisor-igvm" -O cloud-hypervisor-igvm

      mkdir -p /opt/confidential-containers/share/kata-containers/
      mv igvm.bin /opt/confidential-containers/share/kata-containers/igvm.bin
      mv igvm-measurement /opt/confidential-containers/share/kata-containers/igvm-measurement
      mv kata-containers-initrd.img /opt/confidential-containers/share/kata-containers/kata-containers-initrd.img
      mv kata-containers.img /opt/confidential-containers/share/kata-containers/kata-containers.img
      mkdir -p /opt/confidential-containers/bin/
      mv cloud-hypervisor-igvm /opt/confidential-containers/bin/cloud-hypervisor-igvm
      chmod 777 /opt/confidential-containers/bin/cloud-hypervisor-igvm

      echo "wget kata-cc packages"
      wget "https://mitchzhu.blob.core.windows.net/public/kata-containers-cc-0.4.1-4.cm2.x86_64.rpm" -O kata-containers-cc-0.4.1-4.cm2.x86_64.rpm
      wget "https://mitchzhu.blob.core.windows.net/public/kata-containers-cc-tools-0.4.1-4.cm2.x86_64.rpm" -O kata-containers-cc-tools-0.4.1-4.cm2.x86_64.rpm
      wget "https://kataccstorage.blob.core.windows.net/confidential-containers/igvm-generator-0.0.1-3.cm2.x86_64.rpm" -O igvm-generator-0.0.1-3.cm2.x86_64.rpm
      wget "https://mitchzhu.blob.core.windows.net/public/kernel-uvm-5.15.110.mshv2-1.cm2.x86_64.rpm" -O kernel-uvm-5.15.110.mshv2-1.cm2.x86_64.rpm
      wget "https://mitchzhu.blob.core.windows.net/public/kernel-uvm-devel-5.15.110.mshv2-1.cm2.x86_64.rpm" -O kernel-uvm-devel-5.15.110.mshv2-1.cm2.x86_64.rpm

      rpm -ihv kernel-uvm-5.15.110.mshv2-1.cm2.x86_64.rpm
      rpm -ihv kernel-uvm-devel-5.15.110.mshv2-1.cm2.x86_64.rpm
      rpm -ihv kata-containers-cc-0.4.1-4.cm2.x86_64.rpm
      rpm -ihv kata-containers-cc-tools-0.4.1-4.cm2.x86_64.rpm
      rpm -ihv igvm-generator-0.0.1-3.cm2.x86_64.rpm

      rm kata-containers-cc-0.4.1-4.cm2.x86_64.rpm
      rm kata-containers-cc-tools-0.4.1-4.cm2.x86_64.rpm
      rm igvm-generator-0.0.1-3.cm2.x86_64.rpm
      rm kernel-uvm-5.15.110.mshv2-1.cm2.x86_64.rpm
      rm kernel-uvm-devel-5.15.110.mshv2-1.cm2.x86_64.rpm

      echo "wget mshv packages"
      wget "https://mitchzhu.blob.core.windows.net/public/mshv-bootloader-25357.1.230428-1528.1.cm2.x86_64.rpm" -O mshv-bootloader-25357.1.230428-1528.1.cm2.x86_64.rpm
      wget "https://mitchzhu.blob.core.windows.net/public/mshv-linuxloader-0.5.0-2.3.cm2.x86_64.rpm" -O mshv-linuxloader-0.5.0-2.3.cm2.x86_64.rpm
      wget "https://mitchzhu.blob.core.windows.net/public/mshv-25357.1.230428-1528.2.cm2.x86_64.rpm" -O mshv-25357.1.230428-1528.2.cm2.x86_64.rpm

      rpm -ihv mshv-bootloader-25357.1.230428-1528.1.cm2.x86_64.rpm
      rpm -ihv mshv-linuxloader-0.5.0-2.3.cm2.x86_64.rpm
      rpm -ihv mshv-25357.1.230428-1528.2.cm2.x86_64.rpm

      rm mshv-bootloader-25357.1.230428-1528.1.cm2.x86_64.rpm
      rm mshv-linuxloader-0.5.0-2.3.cm2.x86_64.rpm
      rm mshv-25357.1.230428-1528.2.cm2.x86_64.rpm

      echo "create snapshotter dir"
      mkdir -p /var/lib/containerd/io.containerd.snapshotter.v1.tardev/staging

      echo "Appending kata-cc config to enable IGVM"
      sed -i '/image =/a igvm = "/opt/confidential-containers/share/kata-containers/igvm.bin"' /opt/confidential-containers/share/defaults/kata-containers/configuration-clh.toml
      sed -i 's/cloud-hypervisor/cloud-hypervisor-igvm/g' /opt/confidential-containers/share/defaults/kata-containers/configuration-clh.toml
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

installNvidiaContainerRuntime() {
    MARINER_NVIDIA_CONTAINER_RUNTIME_VERSION="3.11.0"
    MARINER_NVIDIA_CONTAINER_TOOLKIT_VERSION="1.11.0"
    
    for nvidia_package in nvidia-container-runtime-${MARINER_NVIDIA_CONTAINER_RUNTIME_VERSION} nvidia-container-toolkit-${MARINER_NVIDIA_CONTAINER_TOOLKIT_VERSION} nvidia-container-toolkit-base-${MARINER_NVIDIA_CONTAINER_TOOLKIT_VERSION} libnvidia-container-tools-${MARINER_NVIDIA_CONTAINER_TOOLKIT_VERSION} libnvidia-container1-${MARINER_NVIDIA_CONTAINER_TOOLKIT_VERSION}; do
      if ! dnf_install 30 1 600 $nvidia_package; then
        exit $ERR_APT_INSTALL_TIMEOUT
      fi
    done
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
