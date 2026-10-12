#!/bin/bash
set -euo pipefail

prefix=/opt/kata-openvmm
manifest=$prefix/provenance/bundle.json
"$prefix/integration/check-image.sh"
expected=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["kernel_release"])' "$manifest")
[ "$(uname -r)" = "$expected" ] || {
    echo "OpenVMM requires kernel $expected with matching modules; running $(uname -r)" >&2
    exit 1
}
for module in mshv_root loop dm_mod dm_zero dm_verity erofs overlay; do modprobe "$module"; done
test -c /dev/mshv
python3 "$prefix/integration/validate.py" --boot-evidence \
    "$manifest" "$prefix/provenance/pre-reboot-boot-id"
test -c /dev/loop-control
dmsetup targets | grep -q '^linear '
dmsetup targets | grep -q '^zero '
dmsetup targets | grep -q '^verity '
losetup --find >/dev/null
echo "OpenVMM test host prerequisites verified (not an SNP launch or attestation verification)."
echo "WARNING: old kata-cc policy parity is unverified; OpenVMM/EROFS policy and hash-binding port is pending." >&2
