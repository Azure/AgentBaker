#!/bin/bash
set -euo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/l1vh-kata-preview.sh"
validate_l1vh_kata_preview
if [ "${ENABLE_L1VH:-False}" != "True" ]; then
    echo "build-l1vh-kata-preview.sh requires ENABLE_L1VH=True" >&2
    exit 1
fi

workdir=$(mktemp -d)
trap 'rm -rf "$workdir"' EXIT
template="${workdir}/vhd-image-builder-l1vh-kata-preview.json"
render_l1vh_packer_template vhdbuilder/packer/vhd-image-builder-mariner.json > "$template"
echo "Building L1VH Kata preview from ${L1VH_SOURCE_IMAGE_VERSION_ID}"
packer build -timestamp-ui -var-file=vhdbuilder/packer/settings.json "$template"
