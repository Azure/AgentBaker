#!/bin/bash
{{/* BCC/BPF-related error codes */}} 
ERR_IOVISOR_KEY_DOWNLOAD_TIMEOUT=168 {{/* Timeout waiting to download IOVisor repo key */}}
ERR_IOVISOR_APT_KEY_TIMEOUT=169 {{/* Timeout waiting for IOVisor apt-key */}}
ERR_BCC_INSTALL_TIMEOUT=170 {{/* Timeout waiting for bcc install */}}
ERR_BPFTRACE_BIN_DOWNLOAD_FAIL=171 {{/* Failed to download bpftrace binary */}}
ERR_BPFTRACE_TOOLS_DOWNLOAD_FAIL=172 {{/* Failed to download bpftrace default programs */}}

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
