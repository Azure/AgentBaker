#!/bin/bash

# Ubuntu NPD packages own the binary, monitors, plugins, startup script and unit.
# Keep the config deb in the VHD so the live updater can recover without fetching
# the old version from a repository that may no longer carry it.
installNPDPackage() {
    local package="$1"
    local version="$2"
    local cache_dir="/opt/node-problem-detector/downloads"

    isUbuntu || return 1
    apt_get_install 30 1 600 "${package}=${version}" || return 1
    apt-mark hold "${package}" || return 1
    echo "  - ${package} version ${version}" >> "${VHD_LOGS_FILEPATH}"

    [ "${package}" = "node-problem-detector-aks-config" ] || return 0
    mkdir -p "${cache_dir}" /opt/bin || return 1
    (cd "${cache_dir}" && apt-get download "${package}=${version}") || return 1
    ln -snf /usr/bin/node-problem-detector /opt/bin/node-problem-detector || return 1

    local executable
    for executable in /usr/bin/node-problem-detector /usr/bin/npd-log-counter /usr/bin/npd-health-checker /opt/bin/node-problem-detector-startup.sh; do
        if [ ! -x "${executable}" ]; then
            echo "[npd] Missing executable: ${executable}"
            return 1
        fi
    done
    test -s /etc/node-problem-detector.d/custom-plugin-monitor/custom-kubelet-monitor.json || return 1
    test -s /etc/systemd/system/node-problem-detector.service || return 1

    mkdir -p /etc/systemd/system/node-problem-detector.service.d || return 1
    cat > /etc/systemd/system/node-problem-detector.service.d/10-agentbaker.conf <<'EOF' || return 1
[Unit]
After=kubelet.service
Wants=kubelet.service

[Service]
Environment="PATH=/opt/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
EOF
    systemctl daemon-reload || return 1
    systemctl disable --now node-problem-detector.service || return 1
    touch /etc/node-problem-detector.d/skip_vhd_npd || return 1
    chmod 0644 /etc/node-problem-detector.d/skip_vhd_npd || return 1
}
