#!/bin/bash

echo "Sourcing cse_install_distro.sh for Mariner"

removeContainerd() {
    retrycmd_if_failure 10 5 60 dnf remove -y moby-containerd
}

installDeps() {
    dnf_makecache || exit $ERR_APT_UPDATE_TIMEOUT
    dnf_update || exit $ERR_APT_DIST_UPGRADE_TIMEOUT
    for dnf_package in blobfuse ca-certificates check-restart cifs-utils cloud-init-azure-kvp conntrack-tools cracklib dnf-automatic ebtables ethtool fuse git inotify-tools iotop iproute ipset iptables jq logrotate lsof nmap-ncat nfs-utils pam pigz psmisc rsyslog socat sysstat traceroute util-linux xz zip; do
      if ! dnf_install 30 1 600 $dnf_package; then
        exit $ERR_APT_INSTALL_TIMEOUT
      fi
    done

    # install additional apparmor deps for 2.0
    if [[ $OS_VERSION == "2.0" ]]; then
      for dnf_package in apparmor-parser libapparmor; do
        if ! dnf_install 30 1 600 $dnf_package; then
          exit $ERR_APT_INSTALL_TIMEOUT
        fi
      done
    fi

    # Test only: manually add additional sysctl configs
    SYSCTL_CONFIG_DEST="/etc/sysctl.d/60-CIS.conf"
    touch "${SYSCTL_CONFIG_DEST}"
    cat << EOF > "${SYSCTL_CONFIG_DEST}"
# 3.1.2 Ensure packet redirect sending is disabled
net.ipv4.conf.all.send_redirects = 0
net.ipv4.conf.default.send_redirects = 0
# 3.2.1 Ensure source routed packets are not accepted 
net.ipv4.conf.all.accept_source_route = 0
net.ipv4.conf.default.accept_source_route = 0
# 3.2.2 Ensure ICMP redirects are not accepted
net.ipv4.conf.all.accept_redirects = 0
net.ipv4.conf.default.accept_redirects = 0
# 3.2.3 Ensure secure ICMP redirects are not accepted
net.ipv4.conf.all.secure_redirects = 0
net.ipv4.conf.default.secure_redirects = 0
# 3.2.4 Ensure suspicious packets are logged
net.ipv4.conf.all.log_martians = 1
net.ipv4.conf.default.log_martians = 1
# 3.3.1 Ensure IPv6 router advertisements are not accepted
net.ipv6.conf.all.accept_ra = 0
net.ipv6.conf.default.accept_ra = 0
# 3.3.2 Ensure IPv6 redirects are not accepted
net.ipv6.conf.all.accept_redirects = 0
net.ipv6.conf.default.accept_redirects = 0
# refer to https://github.com/kubernetes/kubernetes/blob/75d45bdfc9eeda15fb550e00da662c12d7d37985/pkg/kubelet/cm/container_manager_linux.go#L359-L397
vm.overcommit_memory = 1
kernel.panic = 10
kernel.panic_on_oops = 1
# to ensure node stability, we set this to the PID_MAX_LIMIT on 64-bit systems: refer to https://kubernetes.io/docs/concepts/policy/pid-limiting/
kernel.pid_max = 4194304
# https://github.com/Azure/AKS/issues/772
fs.inotify.max_user_watches = 1048576
EOF
    set -x
}

addMarinerNvidiaRepo() {
    MARINER_NVIDIA_REPO_FILEPATH="/etc/yum.repos.d/mariner-nvidia.repo"
    touch "${MARINER_NVIDIA_REPO_FILEPATH}"
    cat << EOF > "${MARINER_NVIDIA_REPO_FILEPATH}"
[mariner-official-nvidia]
name=CBL-Mariner Official Nvidia 2.0 x86_64
baseurl=https://packages.microsoft.com/cbl-mariner/2.0/prod/nvidia/x86_64
gpgkey=file:///etc/pki/rpm-gpg/MICROSOFT-RPM-GPG-KEY file:///etc/pki/rpm-gpg/MICROSOFT-METADATA-GPG-KEY
gpgcheck=1
repo_gpgcheck=1
enabled=1
skip_if_unavailable=True
sslverify=1
EOF
    set -x
}

downloadGPUDrivers() {
    #if ! dnf_install 30 1 600 cuda; then
    #  exit $ERR_APT_INSTALL_TIMEOUT
    #fi

    # Test using the preview cuda driver for now
    tdnf -y install https://packages.microsoft.com/cbl-mariner/2.0/preview/NVIDIA/x86_64/cuda-510.47.03-3_5.15.57.1.cm2.x86_64.rpm
}

installNvidiaContainerRuntime() {
    for nvidia_package in nvidia-container-runtime nvidia-container-toolkit libnvidia-container-tools libnvidia-container1; do
      if ! dnf_install 30 1 600 $nvidia_package; then
        exit $ERR_APT_INSTALL_TIMEOUT
      fi
    done
}

installNvidiaDocker() {
    if ! dnf_install 30 1 600 nvidia-docker2; then
      exit $ERR_APT_INSTALL_TIMEOUT
    fi
}

installSGXDrivers() {
    echo "SGX drivers not yet supported for Mariner"
    exit $ERR_SGX_DRIVERS_START_FAIL
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
