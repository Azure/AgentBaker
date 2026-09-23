#!/bin/bash

echo "Sourcing tool_installs_acl.sh"

stub() {
    echo "${FUNCNAME[1]} stub"
}

installBcc() {
    stub
}

installBpftrace() {
    stub
}

listInstalledPackages() {
    stub
}

disableNtpAndTimesyncdInstallChrony() {
    # On ACL, chronyd is preinstalled and ntp does not exist, so we only need to
    # mask timesyncd (to prevent conflicts) and enable+start chronyd.

    systemctl stop systemd-timesyncd || exit 1
    systemctl disable systemd-timesyncd || exit 1
    systemctl mask systemd-timesyncd || exit 1

    systemctlEnableAndStart chronyd 30 || exit $ERR_SYSTEMCTL_START_FAIL
}

installFIPS() {
    echo "Installing FIPS..."

    local fips_addon_src="/boot/acl/uki-addons/fips.addon.efi"

    if [ ! -f "${fips_addon_src}" ]; then
        echo "FIPS addon not found at ${fips_addon_src}" >&2
        exit 1
    fi

    # Discover the active UKI on the ESP. systemd-boot loads addons from
    # the directory named "<UKI filename>.extra.d/", so the destination
    # must track the UKI's actual name. ACL images historically named the
    # UKI "acl.efi"; newer (UAPI-compliant) images use "vmlinuz-<ver>.efi".
    # Hardcoding "acl.efi.extra.d/" silently orphans the addon on the new
    # naming scheme and leaves the kernel booting without fips=1.
    local uki_path
    uki_path="$(find /boot/EFI/Linux -maxdepth 1 -type f \
        \( -name 'vmlinuz-*.efi' -o -name 'acl.efi' \) 2>/dev/null \
        | sort | head -n1)"

    if [ -z "${uki_path}" ]; then
        echo "No UKI found under /boot/EFI/Linux (expected acl.efi or vmlinuz-*.efi)" >&2
        exit 1
    fi

    local uki_name
    uki_name="$(basename "${uki_path}")"
    local fips_addon_dst="/boot/EFI/Linux/${uki_name}.extra.d/fips.addon.efi"

    echo "Installing FIPS addon: ${fips_addon_src} -> ${fips_addon_dst}"
    install -D -m 0644 "${fips_addon_src}" "${fips_addon_dst}"

    touch /etc/system-fips
    chmod 644 /etc/system-fips
}
