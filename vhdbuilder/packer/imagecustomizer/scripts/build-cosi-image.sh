#!/bin/bash
set -euo pipefail

# Build a COSI artifact using Azure Linux Image Customizer.
# Modeled on build-imagecustomizer-image.sh but outputs COSI format
# instead of VHD for over-the-wire AB partitioning updates.

required_env_vars=(
    "IMG_CUSTOMIZER_CONTAINER"
    "IMG_CUSTOMIZER_VERSION"
    "IMG_CUSTOMIZER_CONFIG"
    "BASE_IMG"
    "BASE_IMG_VERSION"
)

for v in "${required_env_vars[@]}"
do
    if [ -z "${!v:-}" ]; then
        echo "$v was not set!"
        exit 1
    fi
done

SCRIPTS_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
CONFIG=$IMG_CUSTOMIZER_CONFIG
AGENTBAKER_DIR=$(realpath "$SCRIPTS_DIR/../../../../")
BUILD_DIR="${AGENTBAKER_DIR}/build"
OUT_DIR="${AGENTBAKER_DIR}/out"
mkdir -p "$OUT_DIR"
mkdir -p "$BUILD_DIR"
mkdir -p "$BUILD_DIR/$CONFIG"
mkdir -p "$OUT_DIR/$CONFIG"

CONFIG_FILE="$AGENTBAKER_DIR/vhdbuilder/packer/imagecustomizer/$CONFIG/$CONFIG.yml"
if [ ! -f "$CONFIG_FILE" ]; then
    echo "Error: Config file '$CONFIG_FILE' not found" >&2
    echo "Expected path: vhdbuilder/packer/imagecustomizer/$CONFIG/$CONFIG.yml" >&2
    exit 1
fi

if [ ! -r "$CONFIG_FILE" ]; then
    echo "Error: Config file '$CONFIG_FILE' is not readable" >&2
    exit 1
fi

COSI_PATH="${OUT_DIR}/${CONFIG}/${CONFIG}.cosi"

BASE_IMAGE_ORAS=$BASE_IMG:$BASE_IMG_VERSION
if [ ! -f "$BUILD_DIR/$CONFIG/image.vhdx" ]; then
    echo "Pulling base image $BASE_IMAGE_ORAS from registry..."
    docker run \
        --rm \
        --interactive \
        --privileged=true \
        -v "$BUILD_DIR:/container/build" \
        "$IMG_CUSTOMIZER_CONTAINER:$IMG_CUSTOMIZER_VERSION" \
        oras pull "$BASE_IMAGE_ORAS" -o "/container/build/$CONFIG"
else
    echo "Base image already exists, skipping pull."
fi

echo "Using following Image Customizer config:"
cat "$CONFIG_FILE"

echo "Building COSI artifact from $CONFIG_FILE with Image Customizer..."
docker run \
    --rm \
    --interactive \
    --privileged=true \
    -v "$BUILD_DIR:/container/build" \
    -v "$OUT_DIR:/container/out" \
    -v "$(realpath "$(dirname "$CONFIG_FILE")")":/container/config \
    -v /dev:/dev \
    -v "$AGENTBAKER_DIR/:/AgentBaker:z" \
    "$IMG_CUSTOMIZER_CONTAINER:$IMG_CUSTOMIZER_VERSION" \
    imagecustomizer \
        --log-level "debug" \
        --config-file /container/config/"$(basename "$CONFIG_FILE")" \
        --build-dir /container/build \
        --image-file "/container/build/$CONFIG/image.vhdx" \
        --output-image-format cosi \
        --output-image-file "/container/out/$CONFIG/$(basename "$COSI_PATH")"

SHA256=$(sha256sum "$COSI_PATH" | awk '{print $1}')
echo "$SHA256  $(basename "$COSI_PATH")" > "${OUT_DIR}/${CONFIG}/${CONFIG}.sha256"

SIZE_BYTES=$(stat --printf="%s" "$COSI_PATH")

IMAGE_VERSION="${IMAGE_VERSION:-$(date +%Y%m.%d.0)}"

# Generate publishing info JSON for downstream Nebraska registration
cat > "${OUT_DIR}/${CONFIG}/cosi-publishing-info.json" <<EOF
{
    "artifact_path": "${COSI_PATH}",
    "sha256": "${SHA256}",
    "size_bytes": ${SIZE_BYTES},
    "image_version": "${IMAGE_VERSION}",
    "os_name": "Linux",
    "sku_name": "${IMG_SKU:-${CONFIG}}",
    "offer_name": "${OS_SKU:-AzureContainerLinux}",
    "image_architecture": "${ARCHITECTURE:-X86_64}",
    "config": "${CONFIG}"
}
EOF

echo "COSI artifact built successfully:"
echo "  Path: ${COSI_PATH}"
echo "  SHA256: ${SHA256}"
echo "  Size: ${SIZE_BYTES} bytes"
echo "  Version: ${IMAGE_VERSION}"

# Copy publishing info to repo root for pipeline artifact publishing
cp "${OUT_DIR}/${CONFIG}/cosi-publishing-info.json" "$AGENTBAKER_DIR/"

# Copy build artifacts where later pipeline stages expect them
for artifact in release-notes.txt bcc-tools-installation.log image-bom.json vhd-build-performance-data.json; do
    src="$AGENTBAKER_DIR/vhdbuilder/packer/imagecustomizer/$CONFIG/out/$artifact"
    if [ -f "$src" ]; then
        cp "$src" "$AGENTBAKER_DIR/"
    fi
done

{
  echo "COSI build completed successfully on $(date)"
  echo "VSTS Build NUMBER: ${BUILD_NUMBER:-}"
  echo "VSTS Build ID: ${BUILD_ID:-}"
  echo "Commit: ${COMMIT:-}"
  echo "Image Version: ${IMAGE_VERSION}"
  echo "SHA256: ${SHA256}"
  echo "FIPS enabled: ${ENABLE_FIPS:-}"
} >> "$AGENTBAKER_DIR/release-notes.txt"
