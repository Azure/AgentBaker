#!/bin/bash

# Ubuntu and Azure Linux NPD packages own the binary, monitors, plugins, startup
# script and unit. Keep the config package in the VHD so the live updater can
# recover without fetching the old version from a repository that may no longer
# carry it.
NPD_TDNF_LOCK_FILE="/etc/tdnf/locks.d/node-problem-detector.conf"

# Hold a package against unattended updates and security patching. tdnf reads
# locks.d; the snapshot security update is a tdnf update over ms-oss, which
# would otherwise move the config package outside the LPC transaction.
holdNPDPackage() {
    local package="$1"
    if isUbuntu; then
        apt-mark hold "${package}"
    else
        mkdir -p "$(dirname "${NPD_TDNF_LOCK_FILE}")" || return 1
        grep -qsx "${package}" "${NPD_TDNF_LOCK_FILE}" || printf '%s\n' "${package}" >> "${NPD_TDNF_LOCK_FILE}"
    fi
}

installNPDPackage() {
    local package="$1"
    local version="$2"
    local cache_dir="/opt/node-problem-detector/downloads"
    local full_version

    # components.json pins revisionless upstream versions; bake the newest
    # matching package revision, as for other package-backed components.
    mkdir -p "${cache_dir}" /opt/bin || return 1
    if isUbuntu; then
        full_version="$(getLatestDebPackageVersion "${package}" "${version}")" || full_version=""
    elif isAzureLinux; then
        full_version="$(getLatestRPMPackageVersion "${package}" "${version}")" || full_version=""
    else
        return 1
    fi
    if [ -z "${full_version}" ]; then
        echo "[npd] Failed to find ${package} version ${version}"
        return 1
    fi
    logResolvedPackageVersion "${package}" "${version}" "${full_version}"
    if isUbuntu; then
        apt_get_install 30 1 600 "${package}=${full_version}" || return 1
        if [ "${package}" = "node-problem-detector-aks-config" ]; then
            (cd "${cache_dir}" && apt-get download "${package}=${full_version}") || return 1
        fi
    else
        # tdnf skips downloading an installed package, so cache before installing.
        if [ "${package}" = "node-problem-detector-aks-config" ]; then
            tdnf install -y --downloadonly --downloaddir "${cache_dir}" "${package}-${full_version}" || return 1
        fi
        dnf_install 30 1 600 "${package}-${full_version}" || return 1
    fi
    holdNPDPackage "${package}" || return 1

    [ "${package}" = "node-problem-detector-aks-config" ] || return 0
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
