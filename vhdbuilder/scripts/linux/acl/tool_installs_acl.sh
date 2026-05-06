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
    local fips_addon_dst="/boot/EFI/Linux/acl.efi.extra.d/fips.addon.efi"

    if [ ! -f "${fips_addon_src}" ]; then
        echo "FIPS addon not found at ${fips_addon_src}" >&2
        exit 1
    fi

    install -D -m 0644 "${fips_addon_src}" "${fips_addon_dst}"

    touch /etc/system-fips
    chmod 644 /etc/system-fips
}
