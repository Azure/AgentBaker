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

    install -D -m 0644 \
     /boot/acl/uki-addons/fips.addon.efi \
     /boot/EFI/Linux/acl.efi.extra.d/fips.addon.efi

    touch /etc/system-fips
    chmod 644 /etc/system-fips
}
