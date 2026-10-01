#!/bin/sh

aks_rj_is_supported() {
    OS_ID=$1
    OS_VERSION=$2
    IMAGE_SKU=$3
    FEATURE_FLAGS=$4
    ENABLE_FIPS_LOWER=$(printf '%s' "$5" | tr '[:upper:]' '[:lower:]')
    FEATURE_FLAGS_LOWER=$(printf '%s %s' "$FEATURE_FLAGS" "$ENABLE_FIPS_LOWER" | tr '[:upper:]' '[:lower:]')
    IMAGE_SKU_LOWER=$(printf '%s' "$IMAGE_SKU" | tr '[:upper:]' '[:lower:]')
    INSTALL_REASON=""

    case "$ENABLE_FIPS_LOWER" in
        true|1|yes)
            INSTALL_REASON="FIPS image variant is excluded"
            return 1
            ;;
    esac

    case "$FEATURE_FLAGS_LOWER" in
        *cvm*|*kata*|*fips*|*osguard*|*encrypted*|*immutable*)
            INSTALL_REASON="specialized image variant is excluded"
            return 1
            ;;
    esac

    case "$OS_ID:$OS_VERSION" in
        ubuntu:24.04)
            INSTALL_REASON="Ubuntu 24.04"
            return 0
            ;;
        ubuntu:26.04)
            case "$IMAGE_SKU_LOWER" in
                *minimal*)
                    INSTALL_REASON="Ubuntu 26.04 minimal"
                    return 0
                    ;;
                *)
                    INSTALL_REASON="Ubuntu 26.04 non-minimal image is excluded"
                    return 1
                    ;;
            esac
            ;;
        azurelinux:3.0|azurelinux:3.0.*)
            INSTALL_REASON="Azure Linux 3"
            return 0
            ;;
        *)
            INSTALL_REASON="image is outside the supported OS scope"
            return 1
            ;;
    esac
}

aks_rj_install() {
    INSTALL_DIR=$1
    OS_ID=$2
    OS_VERSION=$3
    IMAGE_SKU=$4
    FEATURE_FLAGS=$5
    ENABLE_FIPS=$6

    if ! aks_rj_is_supported "$OS_ID" "$OS_VERSION" "$IMAGE_SKU" "$FEATURE_FLAGS" "$ENABLE_FIPS"; then
        echo "AKS root journal initramfs: skipping image: $INSTALL_REASON"
        return 0
    fi
    echo "AKS root journal initramfs: installing for $INSTALL_REASON"

    for tool in growpart lsblk sfdisk partx blockdev flock resize2fs tune2fs e2fsck debugfs blkid mke2fs dd findmnt; do
        command -v "$tool" >/dev/null 2>&1 || {
            echo "AKS root journal initramfs: required tool '$tool' is missing" >&2
            return 1
        }
    done
    [ "$(findmnt -n -o FSTYPE /)" = "ext4" ] || {
        echo "AKS root journal initramfs: supported image does not have an ext4 root filesystem" >&2
        return 1
    }

    install -D -m 0755 "$INSTALL_DIR/resize-root-journal" /usr/local/sbin/aks-root-journal-resize
    case "$OS_ID" in
        ubuntu)
            install -D -m 0755 "$INSTALL_DIR/ubuntu-initramfs-hook" /etc/initramfs-tools/hooks/aks-root-journal
            install -D -m 0755 "$INSTALL_DIR/ubuntu-hook" /etc/initramfs-tools/scripts/local-premount/zz-aks-root-journal
            update-initramfs -u -k all
            ;;
        azurelinux)
            install -d -m 0755 /usr/lib/dracut/modules.d/99aksrootjournal
            install -m 0755 "$INSTALL_DIR/resize-root-journal" /usr/lib/dracut/modules.d/99aksrootjournal/resize-root-journal
            install -m 0755 "$INSTALL_DIR/dracut-module-setup.sh" /usr/lib/dracut/modules.d/99aksrootjournal/module-setup.sh
            install -m 0755 "$INSTALL_DIR/dracut-hook.sh" /usr/lib/dracut/modules.d/99aksrootjournal/dracut-hook.sh
            dracut --regenerate-all --force
            ;;
    esac
}

if [ "${0##*/}" = "install.sh" ]; then
    set -eu
    [ "$#" -eq 0 ] || {
        echo "usage: install.sh" >&2
        exit 2
    }
    . /etc/os-release
    aks_rj_install \
        "$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)" \
        "${ID:-}" \
        "${VERSION_ID:-}" \
        "${IMG_SKU:-}" \
        "${FEATURE_FLAGS:-}" \
        "${ENABLE_FIPS:-false}"
fi
