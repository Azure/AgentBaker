#!/bin/bash
#{{/* BCC/BPF-related error codes */}}
ERR_IOVISOR_KEY_DOWNLOAD_TIMEOUT=168 #{{/* Timeout waiting to download IOVisor repo key */}}
ERR_IOVISOR_APT_KEY_TIMEOUT=169 #{{/* Timeout waiting for IOVisor apt-key */}}
ERR_BCC_INSTALL_TIMEOUT=170 #{{/* Timeout waiting for bcc install */}}
ERR_BPFTRACE_BIN_DOWNLOAD_FAIL=171 #{{/* Failed to download bpftrace binary */}}
ERR_BPFTRACE_TOOLS_DOWNLOAD_FAIL=172 #{{/* Failed to download bpftrace default programs */}}
ERR_BPFTRACE_TOOLS_INSTALL_TIMEOUT=173 #{{/* Failed to install bpftrace default programs */}}
ERR_AZCOPY_TOOLS_DOWNLOAD_FAIL=174 #{{/* Failed to download azcopy */}}

BPFTRACE_DOWNLOADS_DIR="/opt/bpftrace/downloads"
UBUNTU_CODENAME=$(lsb_release -c -s 2>/dev/null || echo "")

installJq() {
    # jq is not available until downloaded in install-dependencies.sh with the installDeps function
    # but it is needed earlier to call the capture_benchmarks function in pre-install-dependencies.sh
    if command -v jq &> /dev/null; then
        return 0
    fi
    if isMarinerOrAzureLinux "$OS"; then
        tdnf install -y jq && echo "jq was installed: $(jq --version)"
    else
        apt_get_install 5 1 60 jq && echo "jq was installed: $(jq --version)"
    fi
}

# sets MULTI_ARCH_VERSIONS to multiArchVersionsV2 if it exists, otherwise multiArchVersions
updateMultiArchVersions() {
    MULTI_ARCH_VERSIONS=()
    readarray -t MULTI_ARCH_VERSIONS < <(jq -r "if .multiArchVersionsV2 == null then .multiArchVersions // [] else .multiArchVersionsV2 // [] | map(.latestVersion, .previousLatestVersion) end | .[] | values" <<< "$1")
    return
}

# sets PACKAGE_DOWNLOAD_URL to the URL of the package based on the os and osVersion
updatePackageDownloadURL() {
    PACKAGE_DOWNLOAD_URL=$(jq -r ".downloadURL | values" <<< "$(getPackageJSON "$@")")
    return
}

configureSSHService() {
    local os_param="${1:-$OS}"
    local os_version_param="${2:-$OS_VERSION}"

    # If not Ubuntu, no changes needed
    if [ "$os_param" != "$UBUNTU_OS_NAME" ]; then
        return 0
    fi

    # Only for Ubuntu 22.10+ or newer socket activation is used, for earlier versions no changes needed
    if semverCompare "22.10" "$os_version_param" ; then
        return 0
    fi

    # For Ubuntu 22.10+ with socket-based SSH activation
    # disable socket if is active
    if systemctl is-active --quiet ssh.socket; then
        # Socket is active, disable it and switch to service-based activation
        systemctl disable --now ssh.socket || echo "Warning: Could not disable ssh.socket"
    fi

    # Remove the socket-based configuration files if present
    if [ -f /etc/systemd/system/ssh.service.d/00-socket.conf ]; then
        rm /etc/systemd/system/ssh.service.d/00-socket.conf || echo "Warning: Could not remove 00-socket.conf"
    fi

    if [ -f /etc/systemd/system/ssh.socket.d/addresses.conf ]; then
        rm /etc/systemd/system/ssh.socket.d/addresses.conf || echo "Warning: Could not remove addresses.conf"
    fi

    # For all Ubuntu versions, just make sure ssh service is enabled and running
    if ! systemctl is-enabled --quiet ssh.service; then
        echo "Enabling SSH service..."
        systemctlEnableAndStart ssh 30 || return $ERR_SYSTEMCTL_START_FAIL
    fi
    # Verify SSH service is now running
    if ! systemctl is-active --quiet ssh; then
        echo "Error: Failed to start SSH service after configuration changes"
        return $ERR_SYSTEMCTL_START_FAIL
    fi

    echo "SSH service successfully reconfigured and started"
    return 0
}

# TODO (alburgess) have oras version managed by dependant or Renovate
installOras() {
    ORAS_DOWNLOAD_DIR="/opt/oras/downloads"
    ORAS_EXTRACTED_DIR=${1} # Use components.json var for /opt/bin for linux-vhd-content-test.sh binary file checks.
    ORAS_DOWNLOAD_URL=${2}
    ORAS_VERSION=${3}

    mkdir -p $ORAS_DOWNLOAD_DIR

    echo "Installing Oras version $ORAS_VERSION..."
    # Use bash builtin ## to remove all chars ("*") up to the final "/"
    ORAS_TMP=${ORAS_DOWNLOAD_URL##*/}
    retrycmd_get_tarball 120 5 60 "$ORAS_DOWNLOAD_DIR/${ORAS_TMP}" ${ORAS_DOWNLOAD_URL} 300 || exit $ERR_ORAS_DOWNLOAD_ERROR

    if [ ! -f "$ORAS_DOWNLOAD_DIR/${ORAS_TMP}" ]; then
        echo "File $ORAS_DOWNLOAD_DIR/${ORAS_TMP} does not exist."
        exit $ERR_ORAS_DOWNLOAD_ERROR
    fi

    echo "File $ORAS_DOWNLOAD_DIR/${ORAS_TMP} exists."
    # no-same-owner because the files in the tarball are owned by 1001:admin
    extract_tarball "$ORAS_DOWNLOAD_DIR/${ORAS_TMP}" "$ORAS_EXTRACTED_DIR/"
    rm -r "$ORAS_DOWNLOAD_DIR"
    echo "Oras version $ORAS_VERSION installed successfully."
}

downloadCrictl() {
    # if $1 is empty, take ${CRICTL_DOWNLOAD_DIR} as default value. Otherwise take $1 as the value
    downloadDir=${1:-${CRICTL_DOWNLOAD_DIR}}
    mkdir -p $downloadDir
    url=${2}
    logs_to_events "AKS.CSE.logDownloadURL" "echo $url"
    url=$(update_base_url $url)
    crictlTgzTmp=${url##*/}
    retrycmd_curl_file 10 5 60 "$downloadDir/${crictlTgzTmp}" ${url} 300 || exit $ERR_CRICTL_DOWNLOAD_TIMEOUT
}

overrideNetworkConfig() {
    CONFIG_FILEPATH="/etc/cloud/cloud.cfg.d/80_azure_net_config.cfg"
    mkdir -p "${CONFIG_FILEPATH%/*}"
    touch ${CONFIG_FILEPATH}
    cat <<EOF >>${CONFIG_FILEPATH}
datasource:
    Azure:
        apply_network_config: false
EOF
}

ensureGPUDrivers() {
    configGPUDrivers
    systemctlEnableAndStart nvidia-modprobe 30 || exit $ERR_GPU_DRIVERS_START_FAIL
}

disableSystemdResolvedCache() {
    SERVICE_FILEPATH="/etc/systemd/system/resolv-uplink-override.service"
    touch ${SERVICE_FILEPATH}
    cat << EOF >> ${SERVICE_FILEPATH}
[Unit]
Description=Symlink /etc/resolv.conf to /run/systemd/resolve/resolv.conf
After=systemd-networkd.service

[Service]
Type=oneshot
ExecStart=/usr/bin/ln -sf /run/systemd/resolve/resolv.conf /etc/resolv.conf
RemainAfterExit=yes

[Install]
RequiredBy=network-online.target kubelet.service
EOF

    systemctlEnableAndStart resolv-uplink-override 30 || exit $ERR_SYSTEMCTL_START_FAIL

}

disableSystemdIptables() {
    systemctlDisableAndStop iptables || exit $ERR_DISBALE_IPTABLES

    # Mask the iptables service to prevent it from ever re-enabling and breaking pod networking.
    systemctl mask iptables || exit $ERR_DISBALE_IPTABLES
}

enableCgroupV2forAzureLinux() {
    sed -i 's/systemd.legacy_systemd_cgroup_controller=yes systemd.unified_cgroup_hierarchy=0//g' /boot/systemd.cfg
}

# download and setup azcopy to use to download private packages with MSI auth
getAzCopyCurrentPath() {
  if [ -f ./azcopy ]; then
    echo "./azcopy already exists"
  else
    echo "get azcopy at \"${PWD}\"...start"
    # Download and extract
    local azcopyDownloadURL="https://azcopyvnext.azureedge.net/releases/release-10.22.1-20231220/azcopy_linux_amd64_10.22.1.tar.gz"
    local azcopySha256="7549424d56ab2d8b4033c84c2a9bb167dc2dcbb23998acd7fffb37bc1a71a267"
    if [ "$(isARM64)" -eq 1 ]; then
      azcopyDownloadURL="https://azcopyvnext.azureedge.net/releases/release-10.22.1-20231220/azcopy_linux_arm64_10.22.1.tar.gz"
      azcopySha256="4db9a4b48abc7775f1a5d6d928afc42361dcc57bbfcde23ac82e4c419a0dc8fc"
    fi

    local downloadedPkg="downloadazcopy"
    pkgprefix="azcopy_linux_"

    retrycmd_if_failure 30 5 60 curl -fSL -k -o "$downloadedPkg" "$azcopyDownloadURL" || exit $ERR_AZCOPY_TOOLS_DOWNLOAD_FAIL &&
    echo "$azcopySha256 $downloadedPkg" | sha256sum --check >/dev/null &&
    tar -xvf ./$downloadedPkg &&
    cp ./$pkgprefix*/azcopy ./azcopy &&
    chmod +x ./azcopy

    rm -f $downloadedPkg
    rm -rf ./$pkgprefix*/

    echo "get azcopy...done"
  fi
}
