#!/bin/bash
# Run on ACL as root, passing a directory prepared by prepareACLTrustSources.
set -euo pipefail

if [ "${1:-}" != --namespace ]; then
    exec unshare --mount bash "$0" --namespace "${1:?Pass the prepared trust source directory}"
fi
payload=$(realpath "${2:?}")
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)
if [ "$(readlink /proc/self/ns/mnt)" = "$(readlink /proc/1/ns/mnt)" ]; then
    echo "Refusing to run outside an isolated mount namespace" >&2
    exit 1
fi
mount --make-rprivate /
# Strip existing sysext mounts only in this private namespace, so the test root
# starts from the underlying immutable OS rather than another sysext overlay.
while [ "$(findmnt -no FSTYPE /usr)" = overlay ]; do
    umount --lazy /usr
done
work=$(mktemp -d /tmp/acl-ca-trust.XXXXXX)
root=$work/root

cleanup() {
    # Never remove a directory while the original /etc is still bound into it.
    umount -R "$root" && umount "$work/lower" && rm -rf "$work"
}
trap cleanup EXIT

mkdir -p "$work/lower" "$work/upper" "$work/work" "$root"/{usr,etc,opt,run,tmp,dev,proc,sys}
mount --bind "$root" "$root"
mount -t tmpfs tmpfs "$root/run"
mount --bind /etc "$work/lower"
mount -o remount,bind,ro "$work/lower"
mount --bind /usr "$root/usr"
mount -o remount,bind,ro "$root/usr"
mount --bind /dev "$root/dev"
mount --bind /proc "$root/proc"
mount -o remount,bind,ro "$root/proc"
mount --bind /sys "$root/sys"
mount -o remount,bind,ro "$root/sys"
mount -t overlay overlay -o "lowerdir=$work/lower,upperdir=$work/upper,workdir=$work/work" "$root/etc"
ln -s usr/bin "$root/bin"
ln -s usr/sbin "$root/sbin"
ln -s usr/lib "$root/lib"
ln -s usr/lib64 "$root/lib64"
mkdir -p "$root/opt/aks-sysext/usr/lib/extension-release.d" "$root/etc/extensions"
printf 'ID=_any\nSYSEXT_SCOPE=system\n' > "$root/opt/aks-sysext/usr/lib/extension-release.d/extension-release.aks-sysext"
ln -s /opt/aks-sysext "$root/etc/extensions/aks-sysext"
# Sysext works in a child namespace. Share only this isolated root with that
# child, never with the real host. Chroot confines sysext operations here;
# --root intentionally ignores symlinks and cannot test ACL's discovery path.
mount --make-rshared "$root"
systemd-sysext() {
    chroot "$root" /usr/bin/systemd-sysext "$@"
}
systemctl() {
    command systemctl --root="$root" "$@"
}
sh() {
    chroot "$root" /usr/bin/sh -e /usr/bin/update-ca-trust
}

sha256sum /usr/share/pki/ca-trust-source/*.p11-kit > "$work/factory.sha256"
cp "$root/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem" "$work/old-tls.pem"
# Use production logic, redirecting writes and extraction into the test root.
eval "$(sed -n '/^refreshACLCAStore()/,/^}/p' "$repo_root/vhdbuilder/scripts/linux/acl/tool_installs_acl.sh" |
    sed "s|local payload=/home/packer/acl-ca-certificates|local payload=$payload|; s|local extension_dir=/opt/|local extension_dir=$root/opt/|; s|local active_source=/usr/|local active_source=$root/usr/|; s|readlink /etc/extensions|readlink $root/etc/extensions|; s|/etc/pki/ca-trust/extracted|$root/etc/pki/ca-trust/extracted|g")"
refreshACLCAStore
if cmp -s "$work/old-tls.pem" "$root/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem"; then
    echo "Test requires a factory image with older certificate data" >&2
    exit 1
fi
cp "$root/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem" "$work/refreshed-tls.pem"
refreshACLCAStore
cmp "$work/refreshed-tls.pem" "$root/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem"
sha256sum -c "$work/factory.sha256"

# Recreate the overlay from the same persistent upper layer, as on a fresh boot.
systemd-sysext --no-reload unmerge
umount "$root/etc"
mount -t overlay overlay -o "lowerdir=$work/lower,upperdir=$work/upper,workdir=$work/work" "$root/etc"
systemd-sysext --no-reload merge
test -s "$work/upper/pki/ca-trust/extracted/pem/tls-ca-bundle.pem"
cmp "$work/refreshed-tls.pem" "$root/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem"

err() { echo "$*" >&2; }
getCurrentPackageTestOS() { echo 'AZURELINUX|AZURECONTAINERLINUX'; }
isACL() { [ "${2:-${OS_VARIANT:-}}" = AZURECONTAINERLINUX ]; }
eval "$(sed -n '/^testACLCAStore()/,/^}/p' "$repo_root/vhdbuilder/packer/test/linux-vhd-content-test.sh" |
    sed "s|local extension_dir=/opt/|local extension_dir=$root/opt/|; s|readlink /etc/extensions/|readlink $root/etc/extensions/|; s|/etc/pki/ca-trust/extracted|$root/etc/pki/ca-trust/extracted|g; s|cd /usr/share/|cd $root/usr/share/|")"
testACLCAStore

# A later custom-CA extraction must use the refreshed vendor set, not revive
# roots from the immutable store.
sh -e /usr/bin/update-ca-trust
cmp "$work/refreshed-tls.pem" "$root/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem"
openssl req -x509 -newkey rsa:2048 -nodes -days 1 -subj /CN=acl-refresh-test \
    -keyout "$work/key.pem" -out "$work/custom.crt" >/dev/null 2>&1
OS=AZURELINUX OS_VARIANT=AZURECONTAINERLINUX ERR_UPDATE_CA_CERTS=42
isAzureLinuxOSGuard() { return 1; }
isMarinerOrAzureLinux() { return 1; }
update-ca-trust() { sh -e /usr/bin/update-ca-trust; }
eval "$(sed -n '/^configureHTTPProxyCA()/,/^}/p' "$repo_root/parts/linux/cloud-init/artifacts/cse_config.sh" |
    sed "s|/etc/pki/ca-trust/source/anchors|$root/etc/pki/ca-trust/source/anchors|g")"
HTTP_PROXY_TRUSTED_CA=$(base64 < "$work/custom.crt" | tr -d '\n')
configureHTTPProxyCA
openssl verify -CAfile "$root/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem" "$work/custom.crt"
rm "$root/etc/pki/ca-trust/source/anchors/proxyCA.crt"
sh -e /usr/bin/update-ca-trust
cmp "$work/refreshed-tls.pem" "$root/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem"

install -D -m 0755 "$repo_root/parts/linux/cloud-init/artifacts/update_certs.sh" "$root/opt/scripts/update_certs.sh"
read -r -a service_command <<< "$(sed -n 's/^ExecStart=//p' "$repo_root/parts/linux/cloud-init/artifacts/acl/update_certs.service")"
systemctl() {
    if [ "$*" = 'restart update_certs.service' ]; then
        chroot "$root" "${service_command[@]}"
    else
        command systemctl --root="$root" "$@"
    fi
}
eval "$(sed -n '/^configureCustomCaCertificate()/,/^}/p' "$repo_root/parts/linux/cloud-init/artifacts/cse_config.sh" |
    sed "s|/opt/certs|$root/opt/certs|g")"
CUSTOM_CA_TRUST_COUNT=1
CUSTOM_CA_CERT_0=$HTTP_PROXY_TRUSTED_CA
configureCustomCaCertificate
openssl verify -CAfile "$root/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem" "$work/custom.crt"
rm "$root/opt/certs/00000000000000cert0.crt"
chroot "$root" "${service_command[@]}"
cmp "$work/refreshed-tls.pem" "$root/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem"
sha256sum -c "$work/factory.sha256"
echo 'ACL certificate refresh, upper-layer persistence and custom-CA regeneration passed'
