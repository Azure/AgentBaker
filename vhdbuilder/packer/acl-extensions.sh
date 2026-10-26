#!/bin/bash

writeExtensionRelease() {
    local root=$1 hierarchy=$2 name=$3
    local release_dir level
    if [[ "$hierarchy" == opt ]]; then
        release_dir="$root/usr/lib/extension-release.d"
        level=SYSEXT_LEVEL
    else
        release_dir="$root/etc/extension-release.d"
        level=CONFEXT_LEVEL
    fi
    mkdir -p "$root/$hierarchy" "$release_dir"
    # systemd ignores empty extension hierarchies; /opt needs a payload
    # even before provisioning has produced its first captured file.
    [[ "$hierarchy" != opt ]] || touch "$root/opt/.aks-opt-extension"
    (
        # os-release is supplied by the base OS, not by the build environment.
        # shellcheck disable=SC1091
        source /usr/lib/os-release
        printf 'ID=%s\nVERSION_ID=%s\n' "$ID" "$VERSION_ID"
        if [[ "$level" == SYSEXT_LEVEL && -n "${SYSEXT_LEVEL:-}" ]]; then
            printf 'SYSEXT_LEVEL=%s\n' "$SYSEXT_LEVEL"
        elif [[ "$level" == CONFEXT_LEVEL && -n "${CONFEXT_LEVEL:-}" ]]; then
            printf 'CONFEXT_LEVEL=%s\n' "$CONFEXT_LEVEL"
        fi
        printf 'EXTENSION_RELOAD_MANAGER=1\n'
    ) > "$release_dir/extension-release.$name"
}

buildExtensionImage() {
    local root=$1 destination=$2
    local size_kib
    size_kib=$(du -sk "$root" | cut -f1)
    # Account for inodes, the journal, and filesystem metadata.
    truncate -s "$((size_kib * 2 + 65536))K" "$destination"
    mkfs.ext4 -q -F -m 0 -d "$root" "$destination"
}

checkExtensionSupport() {
    local command version
    for command in systemd-sysext systemd-confext mkfs.ext4 truncate du; do
        command -v "$command" >/dev/null || {
            echo "ACL extension build requires $command" >&2
            return 1
        }
    done
    version=$(systemd-sysext --version | head -n1 | awk '{print $2}')
    if [[ ! "$version" =~ ^[0-9]+$ ]] || ((version < 256)); then
        echo "ACL extension build requires systemd >= 256 (mutable extensions)" >&2
        return 1
    fi
}

verifyExtensionCapture() {
    local hierarchy probe
    for hierarchy in opt etc; do
        probe=$(mktemp "/$hierarchy/.aks-capture.XXXXXX")
        if [[ ! -f "/var/lib/extensions.mutable/$hierarchy/${probe##*/}" ]]; then
            rm "$probe"
            echo "Writes to /$hierarchy are not reaching the capture layer" >&2
            return 1
        fi
        rm "$probe"
    done
}

startExtensionCapture() {
    local state=/var/lib/aks-extension-build
    local hierarchy type binary
    checkExtensionSupport
    [[ ! -e "$state" ]]
    for hierarchy in opt etc; do
        local upper="/var/lib/extensions.mutable/$hierarchy"
        if [[ -L "$upper" ]] || [[ -d "$upper" && -n "$(ls -A "$upper")" ]]; then
            echo "Refusing to capture into existing mutable layer $upper" >&2
            return 1
        fi
    done
    [[ ! -e /var/lib/extensions/aks-opt && ! -e /var/lib/extensions/aks-opt.raw ]]
    [[ ! -e /var/lib/confexts/aks-etc && ! -e /var/lib/confexts/aks-etc.raw ]]
    mkdir -p "$state" /var/lib/extensions.mutable/{opt,etc}
    chmod 700 "$state"

    # These bootstrap files must remain in the base /etc so activation does
    # not depend on configuration supplied by the confext it is activating.
    for type in sysext confext; do
        binary=$(command -v "systemd-$type")
        mkdir -p "/etc/systemd/system/systemd-$type.service.d"
        local options=--mutable=auto
        [[ "$type" != confext ]] || options="$options --noexec=false"
        printf '[Service]\nExecStart=\nExecStart=%s %s refresh\nExecReload=\nExecReload=%s %s refresh\n' \
            "$binary" "$options" "$binary" "$options" \
            > "/etc/systemd/system/systemd-$type.service.d/20-aks-mutable.conf"
    done
    printf '[Unit]\nRequires=systemd-confext.service\nAfter=systemd-confext.service\n' \
        >> /etc/systemd/system/systemd-sysext.service.d/20-aks-mutable.conf
    systemctl enable systemd-sysext.service systemd-confext.service
    systemctl daemon-reload
    writeExtensionRelease /var/lib/extensions/aks-opt opt aks-opt
    writeExtensionRelease /var/lib/confexts/aks-etc etc aks-etc
    systemd-sysext --mutable=auto --no-reload refresh
    systemd-confext --mutable=auto --noexec=false --no-reload refresh
    verifyExtensionCapture
    touch "$state/capturing"
}

finishExtensionCapture() {
    local state=/var/lib/aks-extension-build
    [[ -f "$state/capturing" ]]
    verifyExtensionCapture
    # Deprovision before freezing /etc; otherwise the confext bakes the
    # packer account or machine-specific agent state back into the image.
    /usr/bin/waagent -force -deprovision+user
    export HISTSIZE=0
    sync
    systemd-confext --no-reload unmerge
    systemd-sysext --no-reload unmerge

    mkdir "$state/opt-image" "$state/etc-image"
    mv /var/lib/extensions.mutable/opt "$state/opt-image/opt"
    mv /var/lib/extensions.mutable/etc "$state/etc-image/etc"
    # PID 1 initializes machine-id before confext activation. Leave its
    # empty deprovisioned file in the base, not in the immutable confext.
    rm -f "$state/etc-image/etc/machine-id" /etc/machine-id
    touch /etc/machine-id
    chmod 644 /etc/machine-id
    mkdir /var/lib/extensions.mutable/{opt,etc}
    writeExtensionRelease "$state/opt-image" opt aks-opt
    writeExtensionRelease "$state/etc-image" etc aks-etc
    # ext4 preserves symlinks, ownership, xattrs, opaque directories, and
    # overlay whiteout devices. Do not translate whiteouts into .wh.* files.
    buildExtensionImage "$state/opt-image" /var/lib/extensions/aks-opt.raw
    buildExtensionImage "$state/etc-image" /var/lib/confexts/aks-etc.raw
    rm -rf /var/lib/extensions/aks-opt /var/lib/confexts/aks-etc
    systemd-confext --mutable=auto --noexec=false --no-reload merge
    systemd-sysext --mutable=auto --no-reload merge
    # Only discard captured data after both images have been merged.
    rm -rf "$state/opt-image" "$state/etc-image"
    rm "$state/capturing"
    rmdir "$state"
    sync
}

main() {
    set -euo pipefail
    if [[ $EUID != 0 ]]; then
        echo "ACL extension capture must run as root" >&2
        return 1
    fi
    # shellcheck disable=SC1091
    source /usr/lib/os-release
    local os_id="${ID,,}" os_variant="${VARIANT_ID:-}"
    if [[ "$os_id" != azurecontainerlinux ]] &&
        [[ "$os_id" != azurelinux || "${os_variant,,}" != azurecontainerlinux ]]; then
        echo "ACL extension capture is only supported on Azure Container Linux" >&2
        return 1
    fi
    case "${1:-}" in
        start) startExtensionCapture ;;
        finish) finishExtensionCapture ;;
        *) echo "Usage: $0 {start|finish}" >&2; return 1 ;;
    esac
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    main "$@"
fi
