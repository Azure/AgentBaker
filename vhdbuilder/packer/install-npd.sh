#!/bin/bash

# Ubuntu and Azure Linux NPD packages own the binary, monitors, plugins, startup
# script and unit. Keep both packages in the VHD so the live updaters (npdConfig
# and npdBinary) can roll back without fetching the old version from a repository
# that may no longer carry it.
NPD_TDNF_LOCK_FILE="/etc/tdnf/locks.d/node-problem-detector.conf"

# Hold a package against unattended updates and security patching. tdnf reads
# locks.d; the snapshot security update is a tdnf update over ms-oss, which
# would otherwise move the config package outside the LPC transaction.
holdNPDPackage() {
    local package="$1"
    if isUbuntu; then
        # dpkg selections avoid apt's slow initialization (see holdWALinuxAgent).
        printf '%s hold\n' "${package}" | dpkg --set-selections
    else
        mkdir -p "$(dirname "${NPD_TDNF_LOCK_FILE}")" || return 1
        grep -qsx "${package}" "${NPD_TDNF_LOCK_FILE}" || printf '%s\n' "${package}" >> "${NPD_TDNF_LOCK_FILE}"
    fi
}

# Prints a private apt configuration for releases that use another release's
# config package (see npd_ubuntu_config_source), or nothing. Shares the mapping
# with the LPC handler so baked and live-updated configs come from one place.
npdConfigAptConf() {
    # shellcheck disable=SC1090,SC1091
    source "${NPD_UPDATE_SCRIPT:-/home/packer/npd-update.sh}" || return 1
    NPD_UPDATE_DIR="${NPD_BUILD_DIR:-/tmp/npd-build}" npd_config_apt_conf
}

installNPDPackage() {
    local package="$1"
    local version="$2"
    local cache_dir="/opt/node-problem-detector/downloads"
    local full_version

    # components.json pins revisionless upstream versions; bake the newest
    # matching package revision, as for other package-backed components.
    mkdir -p "${cache_dir}" /opt/bin || return 1
    local apt_conf=""
    if isUbuntu && [ "${package}" = "node-problem-detector-aks-config" ]; then
        apt_conf="$(npdConfigAptConf)" || return 1
    fi
    if [ -n "${apt_conf}" ]; then
        # This release has no config build of its own; resolve and bake the
        # identical package from its config source release.
        APT_CONFIG="${apt_conf}" apt-get -o Acquire::Retries=3 update >/dev/null || return 1
        full_version="$(APT_CONFIG="${apt_conf}" getLatestDebPackageVersion "${package}" "${version}")" || full_version=""
    elif isUbuntu; then
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
    if [ -n "${apt_conf}" ]; then
        (cd "${cache_dir}" && APT_CONFIG="${apt_conf}" apt-get download "${package}=${full_version}") || return 1
        apt_get_install 30 1 600 "${cache_dir}/${package}_${full_version}_$(dpkg --print-architecture).deb" || return 1
        rm -rf "${NPD_BUILD_DIR:-/tmp/npd-build}"
    elif isUbuntu; then
        apt_get_install 30 1 600 "${package}=${full_version}" || return 1
        (cd "${cache_dir}" && apt-get download "${package}=${full_version}") || return 1
    else
        # tdnf skips downloading an installed package, so cache before installing.
        tdnf install -y --downloadonly --downloaddir "${cache_dir}" "${package}-${full_version}" || return 1
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
