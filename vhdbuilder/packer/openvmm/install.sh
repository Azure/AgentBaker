#!/bin/bash
set -euo pipefail

[ "${OPENVMM_UNSIGNED_TEST_ONLY:-}" = true ] || {
    echo "Refusing unsigned OpenVMM artifacts without explicit test-only opt-in" >&2
    exit 1
}
[ "${OPENVMM_INSTALL_HOST:-}" = true ] || {
    echo "Host package installation requires explicit disposable evaluation VM approval" >&2
    exit 1
}
# shellcheck source=/dev/null
source /etc/os-release
if [ "$ID" != azurelinux ] || [ "$VERSION_ID" != 3.0 ] || [ "$(uname -m)" != x86_64 ]; then
    echo "OpenVMM prototype requires Azure Linux 3 x86_64" >&2
    exit 1
fi
bundle=$(realpath "${1:?artifact directory required}")
integration=$(cd "$(dirname "$0")" && pwd)
# Refuse to run on the controller or outside the specifically approved build VM.
curl --fail --silent --show-error --noproxy '*' -H Metadata:true \
    'http://169.254.169.254/metadata/instance/compute?api-version=2021-02-01' |
    python3 -c '
import json,os,sys
c=json.load(sys.stdin)
assert c["subscriptionId"] == os.environ["OPENVMM_BUILD_SUBSCRIPTION"] == "035db282-f1c8-4ce7-b78f-2a7265d5398c", "wrong subscription"
assert c["resourceGroupName"].lower() == os.environ["OPENVMM_BUILD_RESOURCE_GROUP"].lower(), "wrong resource group"
assert c["resourceGroupName"].lower().startswith("pkr-openvmm-"), "not a MAP temporary resource group"
assert c["name"] == os.environ["OPENVMM_BUILD_VM_NAME"], "not the approved disposable build VM"
'
python3 "$integration/validate.py" "$bundle"
prefix=/opt/kata-openvmm
[ ! -e "$prefix" ] || { echo "$prefix already exists; refusing overwrite" >&2; exit 1; }
for tool in dnf grub2-mkconfig grub2-script-check dracut depmod modinfo python3 findmnt; do
    command -v "$tool" >/dev/null
done

package_names=$(python3 - "$bundle/bundle.json" <<'PY'
import json,sys
for name in json.load(open(sys.argv[1]))["packages"]:
    print(name)
PY
)
mapfile -t packages <<< "$package_names"
# Buddy repositories are configured by the image pipeline, not this installer.
dnf install -y "${packages[@]}" device-mapper util-linux
# Also update packages already present in the marketplace image.
dnf upgrade -y "${packages[@]}"
for tool in dmsetup losetup mount; do command -v "$tool" >/dev/null; done
kernel_release=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["kernel_release"])' "$bundle/bundle.json")
test -x /etc/grub.d/50_mariner_mshv_menuentry
[ "$(findmnt -n -o FSTYPE /boot/efi)" = vfat ]
python3 "$integration/validate.py" --installed-host "$bundle/bundle.json"
test -s "/boot/vmlinuz-$kernel_release"
test -d "/lib/modules/$kernel_release"
depmod "$kernel_release"
for module in mshv_root loop dm_mod dm_zero dm_verity erofs overlay; do
    modinfo -k "$kernel_release" "$module" >/dev/null
done
if [ ! -s "/boot/initramfs-$kernel_release.img" ]; then
    dracut --kver "$kernel_release" "/boot/initramfs-$kernel_release.img"
fi
install -m0755 "$integration/grub-menuentry.sh" /etc/grub.d/51_openvmm_snp
printf '%s\n' 'GRUB_DEFAULT="OpenVMM SNP"' > /etc/default/grub.d/99-openvmm.cfg
grub2-mkconfig -o /boot/grub2/grub.cfg
grub2-script-check /boot/grub2/grub.cfg
python3 - "$kernel_release" <<'PY'
import pathlib,re,sys
text=pathlib.Path("/boot/grub2/grub.cfg").read_text()
match=re.search(r'menuentry "OpenVMM SNP" \{(.*?)^\}', text, re.S | re.M)
assert match, "OpenVMM SNP entry missing"
entry=match[1]
assert "chainloader /HvLoader.efi lxhvloader.dll" in entry
assert "MSHV_SEV_SNP=TRUE" in entry
assert "MSHV_FORCE_NESTED=TRUE" in entry
assert f"/vmlinuz-{sys.argv[1]} " in entry
assert f"/initramfs-{sys.argv[1]}.img" in entry
PY

install -d "$prefix/bin" "$prefix/share/kata-containers" \
    "$prefix/share/defaults/kata-containers" "$prefix/provenance" "$prefix/integration" \
    "$prefix/containerd/v1" "$prefix/containerd/v2"
for binary in containerd-shim-kata-v2 containerd ctr mkfs.erofs; do
    install -m 0755 "$bundle/$binary" "$prefix/bin/$binary"
done
ln -s /usr/bin/openvmm "$prefix/bin/openvmm"
ln -s containerd-shim-kata-v2 "$prefix/bin/containerd-shim-kata-openvmm-v2"
install -m 0644 "$bundle"/kata-openvmm-snp*.bin "$bundle/kata-containers.img" "$prefix/share/kata-containers/"
install -m 0644 "$bundle"/configuration-openvmm-azure-snp-runtime-rs*.toml "$prefix/share/defaults/kata-containers/"
cp -r "$bundle/measurements" "$prefix/provenance/"
install -m 0644 "$bundle/bundle.json" "$bundle/SHA256SUMS" "$prefix/provenance/"
rpm -q kernel-mshv mshv mshv-bootloader-lx mshv-kd openvmm > "$prefix/provenance/installed-packages.txt"
cat /proc/sys/kernel/random/boot_id > "$prefix/provenance/pre-reboot-boot-id"
install -m 0755 "$integration/check-host.sh" "$integration/check-image.sh" "$prefix/integration/"
install -m 0644 "$integration/validate.py" "$prefix/integration/"
install -m 0644 "$integration/containerd-v2.toml" "$prefix/containerd/v2/openvmm.toml"
# The newer snapshotter enforcement knob is optional, not an old-baseline prerequisite.
dmverity_mode_supported=$("$prefix/bin/containerd" config default | python3 -c '
import sys,tomllib
p=tomllib.loads(sys.stdin.read())["plugins"]
print("yes" if "dmverity_mode" in p["io.containerd.snapshotter.v1.erofs"] else "no")
')
if [ "$dmverity_mode_supported" = no ]; then
    sed -i '/^  dmverity_mode = /d' "$prefix/containerd/v2/openvmm.toml"
    echo "Optional dmverity_mode knob unavailable; EROFS differ dm-verity remains enabled."
fi
sed -e 's/\[plugins."io.containerd.cri.v1.images"\]/[plugins."io.containerd.grpc.v1.cri".containerd]/' \
    -e 's/io.containerd.cri.v1.runtime/io.containerd.grpc.v1.cri/g' \
    -e '/^  use_local_image_pull = /d' \
    "$prefix/containerd/v2/openvmm.toml" > "$prefix/containerd/v1/openvmm.toml"
# This containerd 2.x-only setting is not part of the legacy CRI containerd table.
cat >> "$prefix/containerd/v1/openvmm.toml" <<'EOF'
[plugins."io.containerd.cri.v1.images"]
  use_local_image_pull = false
EOF

install -d /etc/systemd/system/containerd.service.d
cat > /etc/systemd/system/containerd.service.d/zz-openvmm-test.conf <<'EOF'
[Service]
Environment="PATH=/opt/kata-openvmm/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
ExecStartPre=/opt/kata-openvmm/integration/check-image.sh
ExecStart=
ExecStart=/opt/kata-openvmm/bin/containerd
EOF
printf '%s\n' \
    'UNSIGNED LOCAL TEST ARTIFACTS — NOT PRODUCTION CONFIDENTIAL SUPPORT' \
    'old kata-cc policy parity: NOT VERIFIED (OpenVMM/EROFS policy and hash-binding port pending)' > "$prefix/TEST-ONLY"
systemctl daemon-reload
echo "OpenVMM artifacts installed; Packer checks image contents, then CC-node tests must run check-host.sh."
