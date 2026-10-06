#!/bin/bash
# POC: publishTarget=sfs publisher for the Azure Local edge node image.
#
# The AKS default publisher (publish-imagecustomizer-image.sh) uploads the VHD to
# a staging blob and creates a SIG (Azure Compute Gallery) image version. Azure
# Local instead distributes node images through the signed 3-layer SFS catalog,
# cloned per host by MOC's DownloadSdk.
#
# This POC stages the fixed-size VHD and emits the metadata an SFS publish needs.
# The actual SFS upload is owned by the aksarc-vhd/AzureLocal + sfs-publishing
# tooling in the Aks-Arc-Assembly monorepo; wire it in here when converging.
set -euo pipefail

CONFIG="${IMG_CUSTOMIZER_CONFIG:?IMG_CUSTOMIZER_CONFIG must be set}"
SCRIPTS_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
AGENTBAKER_DIR="$(realpath "${SCRIPTS_DIR}/../../../../")"
OUT_DIR="${AGENTBAKER_DIR}/out"
# Resolve the built image regardless of output format (vhd/vhdx/qcow2/raw).
VHD_PATH=""
for ext in vhd vhdx qcow2 raw; do
    if [ -f "${OUT_DIR}/${CONFIG}.${ext}" ]; then
        VHD_PATH="${OUT_DIR}/${CONFIG}.${ext}"
        break
    fi
done
STAGE_DIR="${OUT_DIR}/sfs-stage"
CREATE_TIME="$(date +%s)"
IMG_VERSION="${CAPTURED_SIG_VERSION:-1.${CREATE_TIME}.0}"

if [ -z "${VHD_PATH}" ] || [ ! -f "${VHD_PATH}" ]; then
    echo "ERROR: no built image found at ${OUT_DIR}/${CONFIG}.{vhd,vhdx,qcow2,raw}" >&2
    exit 1
fi
IMG_EXT="${VHD_PATH##*.}"

mkdir -p "${STAGE_DIR}"
STAGED_VHD="${STAGE_DIR}/${CONFIG}-${IMG_VERSION}.${IMG_EXT}"
cp "${VHD_PATH}" "${STAGED_VHD}"

SHA256="$(sha256sum "${STAGED_VHD}" | awk '{print $1}')"
SIZE_BYTES="$(stat -c %s "${STAGED_VHD}")"

MANIFEST="${STAGE_DIR}/${CONFIG}-${IMG_VERSION}.sfs.json"
cat > "${MANIFEST}" <<EOF
{
  "config": "${CONFIG}",
  "imageVersion": "${IMG_VERSION}",
  "publishTarget": "sfs",
  "bootstrapMode": "kubeadm-nocloud",
  "agentSet": "edge",
  "vhd": "$(basename "${STAGED_VHD}")",
  "sizeBytes": ${SIZE_BYTES},
  "sha256": "${SHA256}",
  "osImageUriHint": "sfs://node-image-ref/${CONFIG}/${IMG_VERSION}",
  "createdUtc": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}
EOF

echo "Staged edge VHD for SFS publish:"
echo "  vhd:      ${STAGED_VHD}"
echo "  size:     ${SIZE_BYTES} bytes"
echo "  sha256:   ${SHA256}"
echo "  manifest: ${MANIFEST}"
echo
echo "TODO (out of POC scope): hand ${STAGED_VHD} + ${MANIFEST} to the"
echo "Aks-Arc-Assembly sfs-publishing pipeline to sign + publish into the SFS"
echo "3-layer catalog, then reference it from IaaSGalleryImage.osImageUri."

# Surface the staged artifacts to the pipeline if running under ADO.
echo "##vso[task.setvariable variable=EDGE_STAGED_VHD]${STAGED_VHD}" || true
echo "##vso[task.setvariable variable=EDGE_SFS_MANIFEST]${MANIFEST}" || true
