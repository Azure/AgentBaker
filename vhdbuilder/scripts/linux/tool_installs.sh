#!/bin/bash
{{/* BCC/BPF-related error codes */}}
ERR_IOVISOR_KEY_DOWNLOAD_TIMEOUT=168 {{/* Timeout waiting to download IOVisor repo key */}}
ERR_IOVISOR_APT_KEY_TIMEOUT=169 {{/* Timeout waiting for IOVisor apt-key */}}
ERR_BCC_INSTALL_TIMEOUT=170 {{/* Timeout waiting for bcc install */}}
ERR_BPFTRACE_BIN_DOWNLOAD_FAIL=171 {{/* Failed to download bpftrace binary */}}
ERR_BPFTRACE_TOOLS_DOWNLOAD_FAIL=172 {{/* Failed to download bpftrace default programs */}}
ERR_BPFTRACE_TOOLS_INSTALL_TIMEOUT=173 {{/* Failed to install bpftrace default programs */}}
ERR_AZCOPY_TOOLS_DOWNLOAD_FAIL=174 {{/* Failed to download azcopy */}}

BPFTRACE_DOWNLOADS_DIR="/opt/bpftrace/downloads"
UBUNTU_CODENAME=$(lsb_release -c -s)

ensureGPUDrivers() {
    configGPUDrivers
    systemctlEnableAndStart nvidia-modprobe || exit $ERR_GPU_DRIVERS_START_FAIL
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
RemainAfterExit=no

[Install]
RequiredBy=network-online.target kubelet.service
EOF

    systemctlEnableAndStart resolv-uplink-override || exit $ERR_SYSTEMCTL_START_FAIL

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
  if [[ -f ./azcopy ]]; then
    echo "./azcopy already exists"
  else
    echo "get azcopy at \"${PWD}\"...start"
    # Download and extract
    local azcopyDownloadURL="https://azcopyvnext.azureedge.net/releases/release-10.23.0-20240129/azcopy_linux_amd64_10.23.0.tar.gz"
    local azcopySha256="69a72297736edd1afa068efc2ee0704baa819c49d6ca9d1a2950a5fff18b8431"
    if [[ $(isARM64) == 1 ]]; then
      azcopyDownloadURL="https://azcopyvnext.azureedge.net/releases/release-10.23.0-20240129/azcopy_linux_arm64_10.23.0.tar.gz"
      azcopySha256="afee9cc7577a5aa90a23dcc11cb488b7521d57570ba93b80ac489a7b35a74b9f"
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