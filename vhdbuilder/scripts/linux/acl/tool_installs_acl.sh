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

refreshACLCAStore() {
    local payload=/home/packer/acl-ca-certificates
    local extension_dir=/opt/aks-sysext
    local vendor_source="$extension_dir/usr/share/pki/ca-trust-source"
    local active_source=/usr/share/pki/ca-trust-source
    local source_file

    if [ "$(readlink /etc/extensions/aks-sysext)" != /opt/aks-sysext ] ||
        [ ! -s "$extension_dir/usr/lib/extension-release.d/extension-release.aks-sysext" ]; then
        echo "Missing existing ACL aks-sysext configuration" >&2
        return 1
    fi
    mkdir -p "$vendor_source" || return 1
    for source_file in ca-bundle.trust.microsoft.p11-kit ca-bundle.trust.distrusted.p11-kit; do
        if [ ! -s "$payload/$source_file" ]; then
            echo "Missing refreshed ACL trust source: $payload/$source_file" >&2
            return 1
        fi
        install -m 0644 "$payload/$source_file" "$vendor_source/$source_file" || return 1
    done

    # Reuse ACL's existing writable sysext without replacing its metadata or
    # other content. NSS and later custom-CA extraction see the same vendor data.
    systemctl enable systemd-sysext.service || return 1
    if ! systemd-sysext --no-reload refresh; then
        echo "Failed to merge refreshed ACL trust sources" >&2
        return 1
    fi
    for source_file in ca-bundle.trust.microsoft.p11-kit ca-bundle.trust.distrusted.p11-kit; do
        if ! cmp -s "$payload/$source_file" "$active_source/$source_file"; then
            echo "Refreshed ACL trust source is not active: $source_file" >&2
            return 1
        fi
    done
    # The packaged script does not set -e: stop if any format fails to regenerate.
    sh -e /usr/bin/update-ca-trust || return 1
    local bundle
    for bundle in pem/tls-ca-bundle.pem pem/email-ca-bundle.pem pem/objsign-ca-bundle.pem \
        openssl/ca-bundle.trust.crt java/cacerts edk2/cacerts.bin; do
        if [ ! -f "/etc/pki/ca-trust/extracted/$bundle" ]; then
            echo "Missing refreshed ACL trust bundle: $bundle" >&2
            return 1
        fi
    done
    if [ ! -s /etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem ]; then
        echo "Refreshed ACL TLS trust bundle is empty" >&2
        return 1
    fi
    install -m 0644 "$payload/version" "$extension_dir/ca-certificates.version" || return 1
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
