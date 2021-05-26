#!/bin/bash

echo "Sourcing cse_install_distro.sh for Ubuntu"

removeMoby() {
    wait_for_apt_locks
    retrycmd_if_failure 10 5 60 apt-get purge -y moby-engine moby-cli
}

removeContainerd() {
    wait_for_apt_locks
    retrycmd_if_failure 10 5 60 apt-get purge -y moby-containerd
}

installDeps() {
    retrycmd_if_failure_no_stats 120 5 25 curl -fsSL https://packages.microsoft.com/config/ubuntu/${UBUNTU_RELEASE}/packages-microsoft-prod.deb > /tmp/packages-microsoft-prod.deb || exit $ERR_MS_PROD_DEB_DOWNLOAD_TIMEOUT
    retrycmd_if_failure 60 5 10 dpkg -i /tmp/packages-microsoft-prod.deb || exit $ERR_MS_PROD_DEB_PKG_ADD_FAIL
    aptmarkWALinuxAgent hold
    apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT
    apt_get_dist_upgrade || exit $ERR_APT_DIST_UPGRADE_TIMEOUT
    for apt_package in apache2-utils apt-transport-https blobfuse=1.3.7 ca-certificates ceph-common cgroup-lite cifs-utils conntrack cracklib-runtime ebtables ethtool fuse git glusterfs-client htop iftop init-system-helpers iotop iproute2 ipset iptables jq libpam-pwquality libpwquality-tools mount nfs-common pigz socat sysfsutils sysstat traceroute util-linux xz-utils zip; do
      if ! apt_get_install 30 1 600 $apt_package; then
        journalctl --no-pager -u $apt_package
        exit $ERR_APT_INSTALL_TIMEOUT
      fi
    done
}

installGPUDrivers() {
    mkdir -p $GPU_DEST/tmp
    retrycmd_if_failure_no_stats 120 5 25 curl -fsSL https://nvidia.github.io/nvidia-docker/gpgkey > $GPU_DEST/tmp/aptnvidia.gpg || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
    wait_for_apt_locks
    retrycmd_if_failure 120 5 25 apt-key add $GPU_DEST/tmp/aptnvidia.gpg || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
    wait_for_apt_locks
    retrycmd_if_failure_no_stats 120 5 25 curl -fsSL https://nvidia.github.io/nvidia-docker/ubuntu${UBUNTU_RELEASE}/nvidia-docker.list > $GPU_DEST/tmp/nvidia-docker.list || exit  $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
    wait_for_apt_locks
    retrycmd_if_failure_no_stats 120 5 25 cat $GPU_DEST/tmp/nvidia-docker.list > /etc/apt/sources.list.d/nvidia-docker.list || exit  $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
    apt_get_update
    retrycmd_if_failure 30 5 3600 apt-get install -y linux-headers-$(uname -r) gcc make dkms || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
    retrycmd_if_failure 30 5 60 curl -fLS https://us.download.nvidia.com/tesla/$GPU_DV/NVIDIA-Linux-x86_64-${GPU_DV}.run -o ${GPU_DEST}/nvidia-drivers-${GPU_DV} || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
    tmpDir=$GPU_DEST/tmp
    if ! (
      set -e -o pipefail
      cd "${tmpDir}"
      retrycmd_if_failure 30 5 3600 apt-get download nvidia-docker2="${NVIDIA_DOCKER_VERSION}+${NVIDIA_DOCKER_SUFFIX}" || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
    ); then
      exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
    fi
}

installSGXDrivers() {
    echo "Installing SGX driver"
    local VERSION
    VERSION=$(grep DISTRIB_RELEASE /etc/*-release| cut -f 2 -d "=")
    case $VERSION in
    "18.04")
        SGX_DRIVER_URL="https://download.01.org/intel-sgx/dcap-1.2/linux/dcap_installers/ubuntuServer18.04/sgx_linux_x64_driver_1.12_c110012.bin"
        ;;
    "16.04")
        SGX_DRIVER_URL="https://download.01.org/intel-sgx/dcap-1.2/linux/dcap_installers/ubuntuServer16.04/sgx_linux_x64_driver_1.12_c110012.bin"
        ;;
    "*")
        echo "Version $VERSION is not supported"
        exit 1
        ;;
    esac

    local PACKAGES="make gcc dkms"
    wait_for_apt_locks
    retrycmd_if_failure 30 5 3600 apt-get -y install $PACKAGES  || exit $ERR_SGX_DRIVERS_INSTALL_TIMEOUT

    local SGX_DRIVER
    SGX_DRIVER=$(basename $SGX_DRIVER_URL)
    local OE_DIR=/opt/azure/containers/oe
    mkdir -p ${OE_DIR}

    retrycmd_if_failure 120 5 25 curl -fsSL ${SGX_DRIVER_URL} -o ${OE_DIR}/${SGX_DRIVER} || exit $ERR_SGX_DRIVERS_INSTALL_TIMEOUT
    chmod a+x ${OE_DIR}/${SGX_DRIVER}
    ${OE_DIR}/${SGX_DRIVER} || exit $ERR_SGX_DRIVERS_START_FAIL
}

updateAptWithMicrosoftPkg() {
    retrycmd_if_failure_no_stats 120 5 25 curl https://packages.microsoft.com/config/ubuntu/${UBUNTU_RELEASE}/prod.list > /tmp/microsoft-prod.list || exit $ERR_MOBY_APT_LIST_TIMEOUT
    retrycmd_if_failure 10 5 10 cp /tmp/microsoft-prod.list /etc/apt/sources.list.d/ || exit $ERR_MOBY_APT_LIST_TIMEOUT
    retrycmd_if_failure_no_stats 120 5 25 curl https://packages.microsoft.com/keys/microsoft.asc | gpg --dearmor > /tmp/microsoft.gpg || exit $ERR_MS_GPG_KEY_DOWNLOAD_TIMEOUT
    retrycmd_if_failure 10 5 10 cp /tmp/microsoft.gpg /etc/apt/trusted.gpg.d/ || exit $ERR_MS_GPG_KEY_DOWNLOAD_TIMEOUT
    apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT
}

# CSE+VHD can dictate the containerd version, users don't care as long as it works
installStandaloneContainerd() {
    CONTAINERD_VERSION=$1
    # azure-built runtimes have a "+azure" suffix in their version strings (i.e 1.4.1+azure). remove that here.
    CURRENT_VERSION=$(containerd -version | cut -d " " -f 3 | sed 's|v||' | cut -d "+" -f 1)
    # v1.4.1 is our lowest supported version of containerd
    
    #if there is no containerd_version input from RP, use hardcoded version
    if [[ -z ${CONTAINERD_VERSION} ]]; then
        CONTAINERD_VERSION="1.4.4"
        echo "Containerd Version not specified, using default version: ${CONTAINERD_VERSION}"
    else
        echo "Using specified Containerd Version: ${CONTAINERD_VERSION}"
    fi

    if semverCompare ${CURRENT_VERSION:-"0.0.0"} ${CONTAINERD_VERSION}; then
        echo "currently installed containerd version ${CURRENT_VERSION} is greater than (or equal to) target base version ${CONTAINERD_VERSION}. skipping installStandaloneContainerd."
    else
        echo "installing containerd version ${CONTAINERD_VERSION}"
        removeMoby
        removeContainerd
        # if containerd version has been overriden then there should exist a local .deb file for it on aks VHDs (best-effort)
        # if no files found then try fetching from packages.microsoft repo
        if [[ "${IS_VHD:-"false"}" = true ]]; then
            CONTAINERD_DEB_TMP="moby-containerd_${CONTAINERD_VERSION/-/\~}+azure-1_amd64.deb"
            CONTAINERD_DEB_FILE="$CONTAINERD_DOWNLOADS_DIR/${CONTAINERD_DEB_TMP}"
            if [[ -f "${CONTAINERD_DEB_FILE}" ]]; then
                installStandaloneContainerdFromFile ${CONTAINERD_DEB_FILE} || exit $ERR_CONTAINERD_INSTALL_TIMEOUT
                return 0
            fi
        fi
        updateAptWithMicrosoftPkg
        apt_get_install 20 30 120 moby-containerd=${CONTAINERD_VERSION}* --allow-downgrades || exit $ERR_CONTAINERD_INSTALL_TIMEOUT
    fi
    ensureRunc
}

installStandaloneContainerdFromFile() {
    CONTAINERD_DEB_FILE=$1
    wait_for_apt_locks
    retrycmd_if_failure 10 5 600 apt-get -y -f install ${CONTAINERD_DEB_FILE} --allow-downgrades
    if [[ $? -ne 0 ]]; then
        return 1
    fi
    rm -Rf $CONTAINERD_DOWNLOADS_DIR &
}

downloadContainerd() {
    CONTAINERD_VERSION=$1
    # currently upstream maintains the package on a storage endpoint rather than an actual apt repo
    CONTAINERD_DOWNLOAD_URL="https://mobyartifacts.azureedge.net/moby/moby-containerd/${CONTAINERD_VERSION}+azure/bionic/linux_amd64/moby-containerd_${CONTAINERD_VERSION/-/\~}+azure-1_amd64.deb"
    mkdir -p $CONTAINERD_DOWNLOADS_DIR
    CONTAINERD_DEB_TMP=${CONTAINERD_DOWNLOAD_URL##*/}
    retrycmd_curl_file 120 5 60 "$CONTAINERD_DOWNLOADS_DIR/${CONTAINERD_DEB_TMP}" ${CONTAINERD_DOWNLOAD_URL} || exit $ERR_CONTAINERD_DOWNLOAD_TIMEOUT
    CONTAINERD_DEB_FILE="$CONTAINERD_DOWNLOADS_DIR/${CONTAINERD_DEB_TMP}"
}

installMoby() {
    CURRENT_VERSION=$(dockerd --version | grep "Docker version" | cut -d "," -f 1 | cut -d " " -f 3 | cut -d "+" -f 1)
    local MOBY_VERSION="19.03.14"
    if semverCompare ${CURRENT_VERSION:-"0.0.0"} ${MOBY_VERSION}; then
        echo "currently installed moby-docker version ${CURRENT_VERSION} is greater than (or equal to) target base version ${MOBY_VERSION}. skipping installMoby."
    else
        removeMoby
        updateAptWithMicrosoftPkg
        MOBY_CLI=${MOBY_VERSION}
        if [[ "${MOBY_CLI}" == "3.0.4" ]]; then
            MOBY_CLI="3.0.3"
        fi
        apt_get_install 20 30 120 moby-engine=${MOBY_VERSION}* moby-cli=${MOBY_CLI}* --allow-downgrades || exit $ERR_MOBY_INSTALL_TIMEOUT
    fi
    ensureRunc
}

ensureRunc() {
    CURRENT_VERSION=$(runc --version | head -n1 | sed 's/runc version //' | sed 's/-/~/')
    local TARGET_VERSION="1.0.0~rc95"
    # runc rc93 has a regression that causes pods to be stuck in containercreation
    # https://github.com/opencontainers/runc/issues/2865
    # not using semverCompare b/c we need to downgrade
    if [ "${CURRENT_VERSION}" != "${TARGET_VERSION}" ]; then
        apt_get_install 20 30 120 moby-runc=${TARGET_VERSION}* --allow-downgrades || exit $ERR_RUNC_INSTALL_TIMEOUT
    fi
}

cleanUpGPUDrivers() {
    rm -Rf $GPU_DEST
    rm -f /etc/apt/sources.list.d/nvidia-docker.list
}

#EOF
