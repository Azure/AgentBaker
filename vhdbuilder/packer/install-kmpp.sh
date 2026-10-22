#!/bin/bash
# Bake KMPP (Key Material Protection Platform) and its D-Bus proxy into Azure Linux 3.0 VHDs.
# Both stay dormant on the VHD. CSE turns them on only when ENABLE_KMPP=true.
#
# installKMPP():
#   1. Opts out of KMPP as the OpenSSL default provider before the package postinst runs
#   2. Installs kmpp-containers and holds it against unattended tdnf updates
#   3. Reverts the TPM unit swap the postinst makes when the build VM has a TPM
#   4. Stops, disables and masks the KMPP units and removes the state they created
#
# installKMPPDbusProxy():
#   1. Installs xdg-dbus-proxy from the Azure Linux extended repo, then drops that repo again
#   2. Installs kmpp-dbus-proxy.service, disabled

KMPP_DISABLE_DEFAULT_PROVIDER_FILE="${KMPP_DISABLE_DEFAULT_PROVIDER_FILE:-/etc/kmpp/disableDefaultProvider}"
KMPP_TDNF_LOCK_FILE="${KMPP_TDNF_LOCK_FILE:-/etc/tdnf/locks.d/kmpp.conf}"
KMPP_SYSTEMD_DIR="${KMPP_SYSTEMD_DIR:-/usr/lib/systemd/system}"
KMPP_DATA_DIR="${KMPP_DATA_DIR:-/var/opt/msft/ap/data}"
KMPP_DBUS_PROXY_SERVICE_SRC="${KMPP_DBUS_PROXY_SERVICE_SRC:-/home/packer/kmpp-dbus-proxy.service}"
KMPP_DBUS_PROXY_SERVICE_DEST="${KMPP_DBUS_PROXY_SERVICE_DEST:-/etc/systemd/system/kmpp-dbus-proxy.service}"
KMPP_UNITS=("dbus-com.microsoft.kmpp.service" "dbus-com.microsoft.kmppctrl.service")

# The 3.1.0 %preun does not check whether it runs for an erase or an upgrade, so
# an unattended upgrade would delete the units and the kmpp user. Version bumps
# must come through components.json and a new VHD instead.
holdKMPPPackage() {
    local package="$1"
    mkdir -p "$(dirname "${KMPP_TDNF_LOCK_FILE}")" || return 1
    grep -qsx "${package}" "${KMPP_TDNF_LOCK_FILE}" || printf '%s\n' "${package}" >> "${KMPP_TDNF_LOCK_FILE}"
}

installKMPP() {
    local version="$1"
    local pkg="kmpp-containers"
    local fullPackageVersion

    echo "[kmpp] Installing ${pkg} version ${version}"

    # Must exist before install and must stay: every postinst run (including
    # upgrades) otherwise rewrites /etc/pki/tls/openssl.cnf to load KMPP for all processes.
    mkdir -p "$(dirname "${KMPP_DISABLE_DEFAULT_PROVIDER_FILE}")" || return 1
    touch "${KMPP_DISABLE_DEFAULT_PROVIDER_FILE}" || return 1

    fullPackageVersion=$(getLatestRPMPackageVersion "${pkg}" "${version}") || fullPackageVersion=""
    if [ -z "${fullPackageVersion}" ]; then
        echo "[kmpp] Failed to find valid ${pkg} version for ${version}"
        return 1
    fi
    logResolvedPackageVersion "${pkg}" "${version}" "${fullPackageVersion}"
    dnf_install 30 1 600 "${pkg}-${fullPackageVersion}" || return 1
    holdKMPPPackage "${pkg}" || return 1

    # Nodes may or may not have a vTPM, so bake the package default unit and
    # keep the kmpp user out of tss regardless of the build VM.
    if [ -f "${KMPP_SYSTEMD_DIR}/dbus-com.microsoft.kmpp.service.default" ]; then
        echo "[kmpp] Reverting TPM unit swap made on the build VM"
        mv -f "${KMPP_SYSTEMD_DIR}/dbus-com.microsoft.kmpp.service" "${KMPP_SYSTEMD_DIR}/dbus-com.microsoft.kmpp.service.tpm" || return 1
        mv -f "${KMPP_SYSTEMD_DIR}/dbus-com.microsoft.kmpp.service.default" "${KMPP_SYSTEMD_DIR}/dbus-com.microsoft.kmpp.service" || return 1
    fi
    if id -nG kmpp 2>/dev/null | tr ' ' '\n' | grep -qx tss; then
        gpasswd -d kmpp tss || return 1
    fi

    # The postinst presets and starts both units. Masking also blocks D-Bus
    # activation through SystemdService= until CSE unmasks them.
    echo "[kmpp] Stopping and masking KMPP units. CSE unmasks and starts them when ENABLE_KMPP=true"
    systemctl daemon-reload || return 1
    systemctl stop "${KMPP_UNITS[@]}" || return 1
    systemctl disable "${KMPP_UNITS[@]}" || return 1
    systemctl mask "${KMPP_UNITS[@]}" || return 1

    # Starting the service created a machine secret. Each node must create its own.
    rm -rf "${KMPP_DATA_DIR}/kmpp" "${KMPP_DATA_DIR}/kmpp-private" || return 1

    echo "  - ${pkg} version ${fullPackageVersion}" >> "${VHD_LOGS_FILEPATH}"
}

installKMPPDbusProxy() {
    local version="$1"
    local pkg="xdg-dbus-proxy"
    local fullPackageVersion

    echo "[kmpp] Installing ${pkg} version ${version}"

    # xdg-dbus-proxy only ships in the extended repo. Drop the repo afterwards
    # (unless the base image already had it) so the VHD repo set is unchanged.
    local addedExtendedRepo=false
    if ! rpm -q azurelinux-repos-extended >/dev/null 2>&1; then
        dnf_install 30 1 600 azurelinux-repos-extended || return 1
        addedExtendedRepo=true
    fi
    fullPackageVersion=$(getLatestRPMPackageVersion "${pkg}" "${version}") || fullPackageVersion=""
    if [ -n "${fullPackageVersion}" ]; then
        logResolvedPackageVersion "${pkg}" "${version}" "${fullPackageVersion}"
        dnf_install 30 1 600 "${pkg}-${fullPackageVersion}" || fullPackageVersion=""
    else
        echo "[kmpp] Failed to find valid ${pkg} version for ${version}"
    fi
    if [ "${addedExtendedRepo}" = "true" ]; then
        dnf_remove 30 1 600 azurelinux-repos-extended || return 1
    fi
    [ -n "${fullPackageVersion}" ] || return 1
    holdKMPPPackage "${pkg}" || return 1

    install -D -m 0644 "${KMPP_DBUS_PROXY_SERVICE_SRC}" "${KMPP_DBUS_PROXY_SERVICE_DEST}" || return 1
    systemctl daemon-reload || return 1
    systemctl disable kmpp-dbus-proxy.service || return 1

    echo "  - ${pkg} version ${fullPackageVersion}" >> "${VHD_LOGS_FILEPATH}"
}
