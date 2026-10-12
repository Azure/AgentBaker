#!/bin/bash
set -euo pipefail

prefix=/opt/kata-openvmm
manifest=$prefix/provenance/bundle.json
python3 "$prefix/integration/validate.py" --installed-host "$manifest"
expected=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["kernel_release"])' "$manifest")
test -s "/boot/vmlinuz-$expected"
test -s "/boot/initramfs-$expected.img"
for module in mshv_root loop dm_mod dm_zero dm_verity erofs overlay; do
    modinfo -k "$expected" "$module" >/dev/null
done
export PATH="$prefix/bin:$PATH"
expected_version=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["containerd_version"])' "$manifest")
[ "$(containerd --version)" = "$expected_version" ]
containerd config default | python3 -c '
import sys,tomllib
p=tomllib.loads(sys.stdin.read())["plugins"]
d=p["io.containerd.differ.v1.erofs"]
s=p["io.containerd.snapshotter.v1.erofs"]
assert "use_local_image_pull" in p["io.containerd.cri.v1.images"]
assert {"enable_dmverity", "enable_tar_index", "mkfs_options"} <= d.keys()
assert "default_size" in s
assert isinstance(s["default_size"], str)
'
for option in --tar --mkfs-time --sort; do
    mkfs.erofs --help 2>&1 | grep -q -- "$option"
done
echo "OpenVMM image contents verified; SNP hardware and guest launch have not been tested."
