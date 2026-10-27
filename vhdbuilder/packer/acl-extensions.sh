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
    (
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
    local root=$1 destination=$2 size_kib
    size_kib=$(du -sk "$root" | cut -f1)
    truncate -s "$((size_kib * 2 + 65536))K" "$destination"
    mkfs.ext4 -q -F -m 0 -d "$root" "$destination"
}

writeExtensionListing() (
    set -euo pipefail
    local image=$1 destination=$2 listing_root
    listing_root=$(mktemp -d) || return
    trap 'if mountpoint -q "$listing_root"; then umount "$listing_root"; fi; rmdir "$listing_root"' EXIT
    mount -o loop,ro,noload "$image" "$listing_root" || return
    {
        printf '# Image: %s\n' "${image##*/}"
        printf '# inode blocks mode links owner group size/device mtime path [-> symlink target]\n'
        (cd "$listing_root" && LC_ALL=C find . -ls | LC_ALL=C sort -k11)
    } > "$destination" || return
    chmod 644 "$destination"
)

writeBaseEtcListing() (
    set -euo pipefail
    local destination=$1
    if isExtensionMounted etc; then
        echo "Cannot list base /etc while the ACL extension overlay is mounted" >&2
        return 1
    fi
    {
        printf '# Base directory: /etc (without ACL confext or runtime upper)\n'
        printf '# Includes base entries hidden or overridden by the confext; no file contents.\n'
        printf '# inode blocks mode links owner group size/device mtime path [-> symlink target]\n'
        (cd / && LC_ALL=C find ./etc -ls | LC_ALL=C sort -k11)
    } > "$destination" || return
    chmod 644 "$destination"
)

checkExtensionSupport() {
    local command version
    for command in systemd-sysext mkfs.ext4 truncate du mount umount mountpoint find sort mktemp findmnt flock; do
        command -v "$command" >/dev/null || {
            echo "ACL extension build requires $command" >&2
            return 1
        }
    done
    version=$(systemd-sysext --version | head -n1 | awk '{print $2}')
    if [[ ! "$version" =~ ^[0-9]+$ ]] || ((version < 255)); then
        echo "ACL extensions require systemd >= 255" >&2
        return 1
    fi
}

checkExtensionRelease() (
    local root=$1 hierarchy=$2 release level
    if [[ "$hierarchy" == opt ]]; then
        release="$root/usr/lib/extension-release.d/extension-release.aks-opt"
        level=SYSEXT_LEVEL
    else
        release="$root/etc/extension-release.d/extension-release.aks-etc"
        level=CONFEXT_LEVEL
    fi
    # Read only fixed metadata fields; never execute image-supplied content.
    # shellcheck disable=SC1091
    source /usr/lib/os-release
    if ! grep -Fxq "ID=$ID" "$release"; then
        echo "ACL $hierarchy extension does not match the base OS identity" >&2
        return 1
    fi
    if grep -q "^$level=" "$release"; then
        grep -Fxq "$level=${!level:-}" "$release"
    else
        grep -Fxq "VERSION_ID=$VERSION_ID" "$release"
    fi || {
        echo "ACL $hierarchy extension does not match the base OS version or level" >&2
        return 1
    }
)

isExtensionMounted() {
    # ACL can already have an /etc mount underneath our overlay.
    [[ "$(findmnt -rn -U -T "/$1" -o SOURCE)" == "aks-extensions-$1" ]]
}

unmountExtensionHierarchy() {
    local hierarchy=$1 runtime=/run/aks-extensions
    if isExtensionMounted "$hierarchy"; then
        # Services may still hold files open during finalization/shutdown.
        umount --lazy "/$hierarchy" || return
    elif [[ -e "$runtime/$hierarchy.active" ]]; then
        echo "Refusing to unmount an unexpected /$hierarchy mount" >&2
        return 1
    fi
    rm -f "$runtime/$hierarchy.active"
    if mountpoint -q "$runtime/base-$hierarchy"; then
        umount --lazy "$runtime/base-$hierarchy" || return
    fi
    if mountpoint -q "$runtime/image-$hierarchy"; then
        umount --lazy "$runtime/image-$hierarchy" || return
    fi
}

mountExtensionHierarchy() {
    local hierarchy=$1 mode=$2 runtime=/run/aks-extensions
    local mutable=/var/lib/aks-extensions.mutable
    local image="/var/lib/aks-extensions/aks-$hierarchy.raw" lower
    isExtensionMounted "$hierarchy" && return 0
    mkdir -p "$runtime/base-$hierarchy" "$runtime/image-$hierarchy" \
        "$mutable/$hierarchy" "$mutable/$hierarchy.work"
    lower="$runtime/base-$hierarchy"
    if [[ "$mode" == activate ]]; then
        [[ -f "$image" ]] || {
            echo "Missing ACL extension image $image" >&2
            return 1
        }
        mount -o loop,ro,noload "$image" "$runtime/image-$hierarchy" || return
        if ! checkExtensionRelease "$runtime/image-$hierarchy" "$hierarchy"; then
            unmountExtensionHierarchy "$hierarchy"
            return 1
        fi
        lower="$runtime/image-$hierarchy/$hierarchy:$lower"
    fi
    if ! mount --bind "/$hierarchy" "$runtime/base-$hierarchy" ||
        ! mount --make-private "$runtime/base-$hierarchy" ||
        ! mount -t overlay "aks-extensions-$hierarchy" \
            -o "lowerdir=$lower,upperdir=$mutable/$hierarchy,workdir=$mutable/$hierarchy.work" "/$hierarchy"; then
        unmountExtensionHierarchy "$hierarchy"
        return 1
    fi
    touch "$runtime/$hierarchy.active"
}

activateExtensions() {
    mountExtensionHierarchy etc activate || return
    if ! mountExtensionHierarchy opt activate; then
        unmountExtensionHierarchy etc
        return 1
    fi
}

stopExtensions() {
    unmountExtensionHierarchy opt || return
    unmountExtensionHierarchy etc
}

installExtensionService() {
    install -m 755 "${BASH_SOURCE[0]}" /var/lib/aks-extensions/acl-extensions.sh
    cat > /etc/systemd/system/aks-extensions.service <<'UNIT'
[Unit]
Description=Activate writable ACL extension layers
DefaultDependencies=no
RequiresMountsFor=/var/lib/aks-extensions /var/lib/aks-extensions.mutable
After=local-fs.target
Before=sysinit.target systemd-sysext.service systemd-sysusers.service systemd-tmpfiles-setup.service cloud-init-local.service
Conflicts=shutdown.target
Before=shutdown.target

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=/var/lib/aks-extensions/acl-extensions.sh activate
ExecStartPost=/usr/bin/systemctl daemon-reload
ExecStop=/var/lib/aks-extensions/acl-extensions.sh stop

[Install]
WantedBy=sysinit.target
UNIT
    mkdir -p /etc/systemd/system/systemd-sysext.service.d
    cat > /etc/systemd/system/systemd-sysext.service.d/20-aks-extensions.conf <<'UNIT'
[Unit]
Requires=aks-extensions.service
After=aks-extensions.service

[Service]
Environment=SYSTEMD_SYSEXT_HIERARCHIES=/usr
UNIT
    systemctl enable aks-extensions.service systemd-sysext.service
    systemctl daemon-reload
}

verifyExtensionCapture() {
    local hierarchy probe
    for hierarchy in opt etc; do
        probe=$(mktemp "/$hierarchy/.aks-capture.XXXXXX")
        if [[ ! -f "/var/lib/aks-extensions.mutable/$hierarchy/${probe##*/}" ]]; then
            rm "$probe"
            echo "Writes to /$hierarchy are not reaching the capture layer" >&2
            return 1
        fi
        rm "$probe"
    done
}

startExtensionCapture() {
    local state=/var/lib/aks-extension-build hierarchy upper
    checkExtensionSupport
    [[ ! -e "$state" && ! -e /var/lib/aks-extensions ]]
    for hierarchy in opt etc; do
        upper="/var/lib/aks-extensions.mutable/$hierarchy"
        if [[ -L "$upper" || ( -e "$upper" && ! -d "$upper" ) ]] ||
            [[ -d "$upper" && -n "$(ls -A "$upper")" ]]; then
            echo "Refusing to capture into existing mutable layer $upper" >&2
            return 1
        fi
    done
    mkdir -p "$state" /var/lib/aks-extensions
    chmod 700 "$state"
    installExtensionService
    mountExtensionHierarchy opt capture
    mountExtensionHierarchy etc capture
    verifyExtensionCapture
    touch "$state/capturing"
}

deprovisionExtensionBuild() {
    /usr/bin/waagent -force -deprovision+user
    export HISTSIZE=0
    sync
}

finishExtensionCapture() {
    local state=/var/lib/aks-extension-build hierarchy
    [[ -f "$state/capturing" ]]
    verifyExtensionCapture
    deprovisionExtensionBuild
    stopExtensions
    for hierarchy in opt etc; do
        mkdir "$state/$hierarchy-image"
        mv "/var/lib/aks-extensions.mutable/$hierarchy" "$state/$hierarchy-image/$hierarchy"
        # Rotate workdirs too: lazily detached build processes can retain the
        # old overlay, so the new runtime overlay needs different directories.
        mv "/var/lib/aks-extensions.mutable/$hierarchy.work" "$state/$hierarchy-work"
        mkdir "/var/lib/aks-extensions.mutable/$hierarchy" "/var/lib/aks-extensions.mutable/$hierarchy.work"
    done
    rm -f "$state/etc-image/etc/machine-id" /etc/machine-id
    touch /etc/machine-id
    chmod 644 /etc/machine-id
    for hierarchy in opt etc; do
        writeExtensionRelease "$state/$hierarchy-image" "$hierarchy" "aks-$hierarchy"
        buildExtensionImage "$state/$hierarchy-image" "/var/lib/aks-extensions/aks-$hierarchy.raw"
        writeExtensionListing "/var/lib/aks-extensions/aks-$hierarchy.raw" "/var/log/aks-$hierarchy.files.txt"
        chmod 644 "/var/lib/aks-extensions/aks-$hierarchy.raw"
    done
    writeBaseEtcListing /var/log/aks-etc-base.files.txt
    activateExtensions
    rm -rf "$state/opt-image" "$state/etc-image" "$state/opt-work" "$state/etc-work"
    rm "$state/capturing"
    rmdir "$state"
    sync
}

prepareExtensionExport() {
    local state=/run/aks-extensions/export user="${SUDO_USER:-}" entry uid
    if [[ ! "$user" =~ ^[a-zA-Z_][a-zA-Z0-9_-]*$ || "$user" == root ]]; then
        echo "Extension export requires sudo from the non-root Packer account" >&2
        return 1
    fi
    entry=$(getent passwd "$user") || return
    uid=$(printf '%s\n' "$entry" | cut -d: -f3)
    [[ "$uid" =~ ^[0-9]+$ && "$uid" != 0 && ! -e "$state" && ! -L "$state" ]] || return 1
    # Keep only the passwd record in RAM, never passwords or authorized keys.
    install -d -m 700 "$state"
    printf '%s\n' "$entry" > "$state/account"
}

enableExtensionExport() {
    local state=/run/aks-extensions/export
    local rule=/etc/sudoers.d/99-aks-extension-export entry user uid
    entry=$(cat "$state/account")
    user=${entry%%:*}
    uid=$(printf '%s\n' "$entry" | cut -d: -f3)
    if getent passwd "$user" >/dev/null || getent passwd "$uid" >/dev/null; then
        echo "Refusing to export extensions containing the Packer account" >&2
        return 1
    fi
    [[ ! -e "$rule" && ! -L "$rule" ]] || return 1
    cp -p /etc/passwd "$state/passwd"
    printf '%s\n' "$entry" >> /etc/passwd
    mkdir -p /etc/sudoers.d
    (
        umask 077
        # The account intentionally has no shadow entry or login credentials.
        printf 'Defaults:%s !pam_acct_mgmt, !pam_session, !pam_setcred\n' "$user"
        printf '%s ALL=(root) NOPASSWD: /var/lib/aks-extensions/acl-extensions.sh complete\n' "$user"
    ) > "$rule"
    chmod 440 "$rule"
    visudo -cf "$rule" >/dev/null
}

completeExtensionExport() {
    local state=/run/aks-extensions/export
    [[ -f "$state/passwd" && -f "$state/account" ]] || return 1
    cp -p "$state/passwd" /etc/passwd
    rm /etc/sudoers.d/99-aks-extension-export
    rm "$state/passwd" "$state/account"
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
    mkdir -p /run/aks-extensions
    exec 9>/run/aks-extensions/lock
    flock -x 9
    case "${1:-}" in
        start) startExtensionCapture ;;
        finish) finishExtensionCapture ;;
        finish-export) prepareExtensionExport; finishExtensionCapture; enableExtensionExport ;;
        complete) completeExtensionExport ;;
        activate) activateExtensions ;;
        stop) stopExtensions ;;
        *) echo "Usage: $0 {start|finish|finish-export|complete|activate|stop}" >&2; return 1 ;;
    esac
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    main "$@"
fi
