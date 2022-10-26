#!/bin/bash
{{/* BCC/BPF-related error codes */}} 
ERR_IOVISOR_KEY_DOWNLOAD_TIMEOUT=168 {{/* Timeout waiting to download IOVisor repo key */}}
ERR_IOVISOR_APT_KEY_TIMEOUT=169 {{/* Timeout waiting for IOVisor apt-key */}}
ERR_BCC_INSTALL_TIMEOUT=170 {{/* Timeout waiting for bcc install */}}
ERR_BPFTRACE_BIN_DOWNLOAD_FAIL=171 {{/* Failed to download bpftrace binary */}}
ERR_BPFTRACE_TOOLS_DOWNLOAD_FAIL=172 {{/* Failed to download bpftrace default programs */}}

BPFTRACE_DOWNLOADS_DIR="/opt/bpftrace/downloads"
UBUNTU_CODENAME=$(lsb_release -c -s)

installBpftrace() {
    local version="v0.9.4"
    local bpftrace_bin="bpftrace"
    local bpftrace_tools="bpftrace-tools.tar"
    local bpftrace_url="https://upstreamartifacts.azureedge.net/$bpftrace_bin/$version"
    local bpftrace_filepath="/usr/local/bin/$bpftrace_bin"
    local tools_filepath="/usr/local/share/$bpftrace_bin"
    if [[ -f "$bpftrace_filepath" ]]; then
        installed_version="$($bpftrace_bin -V | cut -d' ' -f2)"
        if [[ "$version" == "$installed_version" ]]; then
            return
        fi
        rm "$bpftrace_filepath"
        if [[ -d "$tools_filepath" ]]; then
            rm -r  "$tools_filepath"
        fi
    fi
    mkdir -p "$tools_filepath"
    install_dir="$BPFTRACE_DOWNLOADS_DIR/$version"
    mkdir -p "$install_dir"
    download_path="$install_dir/$bpftrace_tools"
    retrycmd_if_failure 30 5 60 curl -fSL -o "$bpftrace_filepath" "$bpftrace_url/$bpftrace_bin" || exit $ERR_BPFTRACE_BIN_DOWNLOAD_FAIL
    retrycmd_if_failure 30 5 60 curl -fSL -o "$download_path" "$bpftrace_url/$bpftrace_tools" || exit $ERR_BPFTRACE_TOOLS_DOWNLOAD_FAIL
    tar -xvf "$download_path" -C "$tools_filepath"
    chmod +x "$bpftrace_filepath"
    chmod -R +x "$tools_filepath/tools"
}

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
}
