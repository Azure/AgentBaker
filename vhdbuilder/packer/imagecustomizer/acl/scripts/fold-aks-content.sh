#!/bin/bash
set -euo pipefail

. /usr/lib/os-release
if [[ "${ID:-}" != "azurelinux" || "${VARIANT_ID:-}" != "azurecontainerlinux" ]]; then
    echo "AKS baseline folding is only supported on ACL" >&2
    exit 1
fi

[[ -d /usr/share/distro/etc && -d /opt && ! -L /opt ]]
if mountpoint -q /opt; then
    echo "/opt must not be mounted during offline customization" >&2
    exit 1
fi

# /opt must be mounted before sysext resolves /etc/extensions links into it.
mkdir -p /usr/lib/systemd/system/local-fs.target.wants /.opt-work
ln -sfn ../opt.mount /usr/lib/systemd/system/local-fs.target.wants/opt.mount
if [[ -e /usr/share/distro/opt ]]; then
    # A job retry can download the already-customized staged VHD.
    if [[ ! -d /usr/share/distro/opt || -L /usr/share/distro/opt ]] ||
        [[ -n "$(find /opt -mindepth 1 -maxdepth 1 -print -quit)" ]]; then
        echo "Existing /opt baseline has runtime changes; restage the original Packer VHD" >&2
        exit 1
    fi
else
    cp -a --no-dereference /opt /usr/share/distro/opt
    find /opt -mindepth 1 -maxdepth 1 -exec rm -rf -- {} +
fi

# Newer customizers own this overlay and fold it during their finalization.
# With the configured 1.5 container, assemble it here to resolve whiteouts
# and opaque directories rather than copying the physical upper layer.
if ! mountpoint -q /etc; then
    mkdir -p /.aks-etc-work
    mount -t overlay overlay \
        -o lowerdir=/usr/share/distro/etc,upperdir=/etc,workdir=/.aks-etc-work,redirect_dir=on,metacopy=off \
        /etc
    trap 'umount /etc' EXIT
    cp -a --no-dereference /etc /usr/share/distro/.aks-etc-fold
    rm -f /usr/share/distro/.aks-etc-fold/machine-id
    umount /etc
    trap - EXIT
    find /etc -mindepth 1 -maxdepth 1 -exec rm -rf -- {} +
    rm -rf /usr/share/distro/etc
    mv /usr/share/distro/.aks-etc-fold /usr/share/distro/etc
    rm -rf /.aks-etc-work
fi
