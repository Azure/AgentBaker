#!/bin/bash

echo "Sourcing cse_install_distro.sh for Ubuntu"

removeMoby() {
    apt_get_purge 10 5 300 moby-engine moby-cli
}

removeContainerd() {
    apt_get_purge 10 5 300 moby-containerd
}

installDeps() {
    if [[ $(isARM64) == 1 ]]; then
        wait_for_apt_locks
        retrycmd_if_failure_no_stats 120 5 25 curl -fsSL https://packages.microsoft.com/config/ubuntu/${UBUNTU_RELEASE}/packages-microsoft-prod.deb > /tmp/packages-microsoft-prod.deb || exit $ERR_MS_PROD_DEB_DOWNLOAD_TIMEOUT
    else
        retrycmd_if_failure_no_stats 120 5 25 curl -fsSL https://packages.microsoft.com/config/ubuntu/${UBUNTU_RELEASE}/packages-microsoft-prod.deb > /tmp/packages-microsoft-prod.deb || exit $ERR_MS_PROD_DEB_DOWNLOAD_TIMEOUT
    fi
    retrycmd_if_failure 60 5 10 dpkg -i /tmp/packages-microsoft-prod.deb || exit $ERR_MS_PROD_DEB_PKG_ADD_FAIL

    aptmarkWALinuxAgent hold
    apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT

    pkg_list=(ca-certificates ceph-common cgroup-lite cifs-utils conntrack cracklib-runtime ebtables ethtool git glusterfs-client htop iftop init-system-helpers inotify-tools iotop iproute2 ipset iptables nftables jq libpam-pwquality libpwquality-tools mount nfs-common pigz socat sysfsutils sysstat traceroute util-linux xz-utils netcat-openbsd zip rng-tools kmod gcc make dkms initramfs-tools linux-headers-$(uname -r) linux-modules-extra-$(uname -r))

    if [ "${UBUNTU_RELEASE}" == "18.04" ]; then
        pkg_list+=(dnsutils)
    else
        pkg_list+=(bind9-dnsutils)
    fi

    local OSVERSION
    OSVERSION=$(grep DISTRIB_RELEASE /etc/*-release| cut -f 2 -d "=")
    BLOBFUSE_VERSION="1.4.5"
    BLOBFUSE2_VERSION="2.2.1"

    if [ "${OSVERSION}" == "16.04" ]; then
        BLOBFUSE_VERSION="1.3.7"
    fi
    if [ "${OSVERSION}" == "18.04" ]; then
        BLOBFUSE2_VERSION="2.2.0"
    fi

    pkg_list+=(blobfuse2=${BLOBFUSE2_VERSION})
    if [[ $(isARM64) != 1 ]]; then
        if [[ "${OSVERSION}" == "22.04" ]]; then
            pkg_list+=(fuse3)
        else
            pkg_list+=(blobfuse=${BLOBFUSE_VERSION} fuse)
        fi
    fi

    for apt_package in ${pkg_list[*]}; do
        if ! apt_get_install 30 1 600 $apt_package; then
            journalctl --no-pager -u $apt_package
            exit $ERR_APT_INSTALL_TIMEOUT
        fi
    done
}

updateAptWithMicrosoftPkg() {
    if [[ $(isARM64) == 1 ]]; then
        if [ "${UBUNTU_RELEASE}" == "22.04" ]; then
            retrycmd_if_failure_no_stats 120 5 25 curl https://packages.microsoft.com/config/ubuntu/${UBUNTU_RELEASE}/prod.list > /tmp/microsoft-prod.list || exit $ERR_MOBY_APT_LIST_TIMEOUT
        else
            retrycmd_if_failure_no_stats 120 5 25 curl https://packages.microsoft.com/config/ubuntu/${UBUNTU_RELEASE}/multiarch/prod.list > /tmp/microsoft-prod.list || exit $ERR_MOBY_APT_LIST_TIMEOUT
        fi
    else
        retrycmd_if_failure_no_stats 120 5 25 curl https://packages.microsoft.com/config/ubuntu/${UBUNTU_RELEASE}/prod.list > /tmp/microsoft-prod.list || exit $ERR_MOBY_APT_LIST_TIMEOUT
    fi

    retrycmd_if_failure 10 5 10 cp /tmp/microsoft-prod.list /etc/apt/sources.list.d/ || exit $ERR_MOBY_APT_LIST_TIMEOUT
    if [[ ${UBUNTU_RELEASE} == "18.04" ]]; then {
        echo "deb [arch=amd64,arm64,armhf] https://packages.microsoft.com/ubuntu/18.04/multiarch/prod testing main" > /etc/apt/sources.list.d/microsoft-prod-testing.list
    }
    elif [[ ${UBUNTU_RELEASE} == "20.04" || ${UBUNTU_RELEASE} == "22.04" ]]; then {
        echo "deb [arch=amd64,arm64,armhf] https://packages.microsoft.com/ubuntu/${UBUNTU_RELEASE}/prod testing main" > /etc/apt/sources.list.d/microsoft-prod-testing.list
    }
    fi
    
    retrycmd_if_failure_no_stats 120 5 25 curl https://packages.microsoft.com/keys/microsoft.asc | gpg --dearmor > /tmp/microsoft.gpg || exit $ERR_MS_GPG_KEY_DOWNLOAD_TIMEOUT
    retrycmd_if_failure 10 5 10 cp /tmp/microsoft.gpg /etc/apt/trusted.gpg.d/ || exit $ERR_MS_GPG_KEY_DOWNLOAD_TIMEOUT
    apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT
}

cleanUpGPUDrivers() {
    rm -Rf $GPU_DEST /opt/gpu
}

installStandaloneContainerd() {
    UBUNTU_RELEASE=$(lsb_release -r -s)
    UBUNTU_CODENAME=$(lsb_release -c -s)
    CONTAINERD_VERSION=$1    
    CONTAINERD_PATCH_VERSION="${2:-1}"

    logs_to_events "AKS.CSE.installContainerRuntime.ensureRunc" "ensureRunc ${RUNC_VERSION:-""}" 

    CURRENT_VERSION=$(containerd -version | cut -d " " -f 3 | sed 's|v||' | cut -d "+" -f 1)
    CURRENT_COMMIT=$(containerd -version | cut -d " " -f 4)

    if [ -z "$CURRENT_VERSION" ]; then
        CURRENT_VERSION="0.0.0"
    fi

    CONTAINERD_PACKAGE_URL="${CONTAINERD_PACKAGE_URL:=}"
    if [[ ! -z ${CONTAINERD_PACKAGE_URL} ]]; then
        echo "Installing containerd from user input: ${CONTAINERD_PACKAGE_URL}"
        logs_to_events "AKS.CSE.installContainerRuntime.removeMoby" removeMoby
        logs_to_events "AKS.CSE.installContainerRuntime.removeContainerd" removeContainerd
        logs_to_events "AKS.CSE.installContainerRuntime.downloadContainerdFromURL" downloadContainerdFromURL ${CONTAINERD_PACKAGE_URL}
        logs_to_events "AKS.CSE.installContainerRuntime.installDebPackageFromFile" "installDebPackageFromFile ${CONTAINERD_DEB_FILE}" || exit $ERR_CONTAINERD_INSTALL_TIMEOUT
        echo "Succeeded to install containerd from user input: ${CONTAINERD_PACKAGE_URL}"
        return 0
    fi

    #if there is no containerd_version input from RP, use hardcoded version
    if [[ -z ${CONTAINERD_VERSION} ]]; then
        CONTAINERD_VERSION="1.7.7"
        if [ "${UBUNTU_RELEASE}" == "18.04" ]; then
            CONTAINERD_VERSION="1.7.1"
        fi
        CONTAINERD_PATCH_VERSION="1"
        echo "Containerd Version not specified, using default version: ${CONTAINERD_VERSION}-${CONTAINERD_PATCH_VERSION}"
    else
        echo "Using specified Containerd Version: ${CONTAINERD_VERSION}-${CONTAINERD_PATCH_VERSION}"
    fi

    CURRENT_MAJOR_MINOR="$(echo $CURRENT_VERSION | tr '.' '\n' | head -n 2 | paste -sd.)"
    DESIRED_MAJOR_MINOR="$(echo $CONTAINERD_VERSION | tr '.' '\n' | head -n 2 | paste -sd.)"
    semverCompare "$CURRENT_VERSION" "$CONTAINERD_VERSION"
    HAS_GREATER_VERSION="$?"

    if [[ "$HAS_GREATER_VERSION" == "0" ]] && [[ "$CURRENT_MAJOR_MINOR" == "$DESIRED_MAJOR_MINOR" ]]; then
        echo "currently installed containerd version ${CURRENT_VERSION} matches major.minor with higher patch ${CONTAINERD_VERSION}. skipping installStandaloneContainerd."
    else
        echo "installing containerd version ${CONTAINERD_VERSION}"
        logs_to_events "AKS.CSE.installContainerRuntime.removeMoby" removeMoby
        logs_to_events "AKS.CSE.installContainerRuntime.removeContainerd" removeContainerd
        CONTAINERD_DEB_FILE="$(ls ${CONTAINERD_DOWNLOADS_DIR}/moby-containerd_${CONTAINERD_VERSION}*)"
        if [[ -f "${CONTAINERD_DEB_FILE}" ]]; then
            logs_to_events "AKS.CSE.installContainerRuntime.installDebPackageFromFile" "installDebPackageFromFile ${CONTAINERD_DEB_FILE}" || exit $ERR_CONTAINERD_INSTALL_TIMEOUT
            return 0
        fi
        logs_to_events "AKS.CSE.installContainerRuntime.downloadContainerdFromVersion" "downloadContainerdFromVersion ${CONTAINERD_VERSION} ${CONTAINERD_PATCH_VERSION}"
        CONTAINERD_DEB_FILE="$(ls ${CONTAINERD_DOWNLOADS_DIR}/moby-containerd_${CONTAINERD_VERSION}*)"
        if [[ -z "${CONTAINERD_DEB_FILE}" ]]; then
            echo "Failed to locate cached containerd deb"
            exit $ERR_CONTAINERD_INSTALL_TIMEOUT
        fi
        logs_to_events "AKS.CSE.installContainerRuntime.installDebPackageFromFile" "installDebPackageFromFile ${CONTAINERD_DEB_FILE}" || exit $ERR_CONTAINERD_INSTALL_TIMEOUT
        return 0
    fi
}

downloadContainerdFromVersion() {
    CONTAINERD_VERSION=$1
    mkdir -p $CONTAINERD_DOWNLOADS_DIR
    updateAptWithMicrosoftPkg 
    apt_get_download 20 30 moby-containerd=${CONTAINERD_VERSION}* || exit $ERR_CONTAINERD_INSTALL_TIMEOUT
    cp -al ${APT_CACHE_DIR}moby-containerd_${CONTAINERD_VERSION}* $CONTAINERD_DOWNLOADS_DIR/ || exit $ERR_CONTAINERD_INSTALL_TIMEOUT
    echo "Succeeded to download containerd version ${CONTAINERD_VERSION}"
}

downloadContainerdFromURL() {
    CONTAINERD_DOWNLOAD_URL=$1
    mkdir -p $CONTAINERD_DOWNLOADS_DIR
    CONTAINERD_DEB_TMP=${CONTAINERD_DOWNLOAD_URL##*/}
    retrycmd_curl_file 120 5 60 "$CONTAINERD_DOWNLOADS_DIR/${CONTAINERD_DEB_TMP}" ${CONTAINERD_DOWNLOAD_URL} || exit $ERR_CONTAINERD_DOWNLOAD_TIMEOUT
    CONTAINERD_DEB_FILE="$CONTAINERD_DOWNLOADS_DIR/${CONTAINERD_DEB_TMP}"
}

installMoby() {
    ensureRunc ${RUNC_VERSION:-""} 
    CURRENT_VERSION=$(dockerd --version | grep "Docker version" | cut -d "," -f 1 | cut -d " " -f 3 | cut -d "+" -f 1)
    local MOBY_VERSION="19.03.14"
    local MOBY_CONTAINERD_VERSION="1.4.13"
    if semverCompare ${CURRENT_VERSION:-"0.0.0"} ${MOBY_VERSION}; then
        echo "currently installed moby-docker version ${CURRENT_VERSION} is greater than (or equal to) target base version ${MOBY_VERSION}. skipping installMoby."
    else
        removeMoby
        updateAptWithMicrosoftPkg
        MOBY_CLI=${MOBY_VERSION}
        if [[ "${MOBY_CLI}" == "3.0.4" ]]; then
            MOBY_CLI="3.0.3"
        fi
        apt_get_install 20 30 120 moby-engine=${MOBY_VERSION}* moby-cli=${MOBY_CLI}* moby-containerd=${MOBY_CONTAINERD_VERSION}* --allow-downgrades || exit $ERR_MOBY_INSTALL_TIMEOUT
    fi
}

ensureRunc() {
    RUNC_PACKAGE_URL="${RUNC_PACKAGE_URL:=}"
    if [[ ! -z ${RUNC_PACKAGE_URL} ]]; then
        echo "Installing runc from user input: ${RUNC_PACKAGE_URL}"
        mkdir -p $RUNC_DOWNLOADS_DIR
        RUNC_DEB_TMP=${RUNC_PACKAGE_URL##*/}
        RUNC_DEB_FILE="$RUNC_DOWNLOADS_DIR/${RUNC_DEB_TMP}"
        retrycmd_curl_file 120 5 60 ${RUNC_DEB_FILE} ${RUNC_PACKAGE_URL} || exit $ERR_RUNC_DOWNLOAD_TIMEOUT
        installDebPackageFromFile ${RUNC_DEB_FILE} || exit $ERR_RUNC_INSTALL_TIMEOUT
        echo "Succeeded to install runc from user input: ${RUNC_PACKAGE_URL}"
        return 0
    fi

    TARGET_VERSION=${1:-""}
    if [[ -z ${TARGET_VERSION} ]]; then
        TARGET_VERSION="1.1.12-ubuntu${UBUNTU_RELEASE}"
        if [ "${UBUNTU_RELEASE}" == "18.04" ]; then
            TARGET_VERSION="1.1.12-ubuntu${UBUNTU_RELEASE}"
        fi
    fi

    if [[ $(isARM64) == 1 ]]; then
        if [[ ${TARGET_VERSION} == "1.0.0-rc92" || ${TARGET_VERSION} == "1.0.0-rc95" ]]; then
            return
        fi
    fi

    CPU_ARCH=$(getCPUArch)  #amd64 or arm64
    CURRENT_VERSION=$(runc --version | head -n1 | sed 's/runc version //')
    CLEANED_TARGET_VERSION=${TARGET_VERSION}

    CURRENT_VERSION=${CURRENT_VERSION%-*} 
    CLEANED_TARGET_VERSION=${CLEANED_TARGET_VERSION%-*} 

    if [ "${CURRENT_VERSION}" == "${CLEANED_TARGET_VERSION}" ]; then
        echo "target moby-runc version ${CLEANED_TARGET_VERSION} is already installed. skipping installRunc."
        return
    fi
    if [ -f $VHD_LOGS_FILEPATH ]; then
        RUNC_DEB_PATTERN="moby-runc_*.deb"
        RUNC_DEB_FILE=$(find ${RUNC_DOWNLOADS_DIR} -type f -iname "${RUNC_DEB_PATTERN}" | sort -V | tail -n1)
        if [[ -f "${RUNC_DEB_FILE}" ]]; then
            installDebPackageFromFile ${RUNC_DEB_FILE} || exit $ERR_RUNC_INSTALL_TIMEOUT
            return 0
        fi
    fi
    apt_get_install 20 30 120 moby-runc=${TARGET_VERSION}* --allow-downgrades || exit $ERR_RUNC_INSTALL_TIMEOUT
}

#EOF
