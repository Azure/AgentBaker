#!/bin/bash
set -euo pipefail

# Required env vars declared by the pipeline
required_env_vars=(
    "IMG_CUSTOMIZER_CONTAINER"
    "IMG_CUSTOMIZER_VERSION"
    "IMG_CUSTOMIZER_CONFIG"
    "BASE_IMG"
    "BASE_IMG_VERSION"
)

for v in "${required_env_vars[@]}"
do
    if [ -z "${!v}" ]; then
        echo "$v was not set!"
        exit 1
    fi
done

# Find the absolute path of the directory containing this script
SCRIPTS_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
CONFIG=$IMG_CUSTOMIZER_CONFIG
AGENTBAKER_DIR=`realpath $SCRIPTS_DIR/../../../../`
BUILD_DIR="${AGENTBAKER_DIR}/build"
OUT_DIR="${AGENTBAKER_DIR}/out"
mkdir -p "$OUT_DIR"
mkdir -p "$BUILD_DIR"
mkdir -p "$BUILD_DIR/$CONFIG"

# Validate CONFIG and config file
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

IMAGE_PATH="${OUT_DIR}/$CONFIG/$CONFIG.vhd"

# POC (default-safe): allow overriding the output image format. Defaults to
# vhd-fixed (unchanged AKS behaviour). Set OUTPUT_IMAGE_FORMAT=vhdx to emit a
# dynamic Gen2 VHDX suitable for a quick local Hyper-V boot test. The output
# file extension and the final copy name follow the format so downstream paths
# stay consistent.
OUTPUT_IMAGE_FORMAT="${OUTPUT_IMAGE_FORMAT:-vhd-fixed}"
case "$OUTPUT_IMAGE_FORMAT" in
    vhd|vhd-fixed) OUTPUT_IMAGE_EXT="vhd" ;;
    vhdx)          OUTPUT_IMAGE_EXT="vhdx" ;;
    qcow2)         OUTPUT_IMAGE_EXT="qcow2" ;;
    raw)           OUTPUT_IMAGE_EXT="raw" ;;
    *)
        echo "Error: unsupported OUTPUT_IMAGE_FORMAT '$OUTPUT_IMAGE_FORMAT' (expected vhd|vhd-fixed|vhdx|qcow2|raw)" >&2
        exit 1
        ;;
esac
IMAGE_PATH="${OUT_DIR}/$CONFIG/$CONFIG.${OUTPUT_IMAGE_EXT}"
echo "Output image format: $OUTPUT_IMAGE_FORMAT (-> $OUTPUT_IMAGE_EXT)"

BASE_IMAGE_ORAS=$BASE_IMG:$BASE_IMG_VERSION
if [ ! -f "$BUILD_DIR/$CONFIG/image.vhdx" ]; then
    echo "Pulling base image $BASE_IMAGE_ORAS from registry..."
    docker run \
        --rm \
        --interactive \
        --privileged=true \
        -v "$BUILD_DIR:/container/build" \
        mcr.microsoft.com/azurelinux/base/core:3.0 \
        sh -c "tdnf install -y oras && oras pull $BASE_IMAGE_ORAS -o /container/build/$CONFIG"
else
    echo "Base image already exists, skipping pull."
fi

echo "Using following Image Customizer config:"
cat $CONFIG_FILE

echo Building $CONFIG_FILE image with Image Customizer...
docker run \
    --rm \
    --interactive \
    --privileged=true \
    -v "$BUILD_DIR:/container/build" \
    -v "$OUT_DIR:/container/out" \
    -v "$(realpath "$(dirname "$CONFIG_FILE")")":/container/config \
    -v /dev:/dev \
    -v "$AGENTBAKER_DIR/:/AgentBaker:z" \
    $IMG_CUSTOMIZER_CONTAINER:$IMG_CUSTOMIZER_VERSION \
    imagecustomizer \
        --log-level "debug" \
        --config-file /container/config/"$(basename "$CONFIG_FILE")" \
        --build-dir /container/build \
        --image-file /container/build/$CONFIG/image.vhdx \
        --output-image-format "$OUTPUT_IMAGE_FORMAT" \
        --output-image-file /container/out/$CONFIG/"$(basename "$IMAGE_PATH")"

cp $IMAGE_PATH $OUT_DIR/$CONFIG.${OUTPUT_IMAGE_EXT}

# Place build artifacts where later pipeline stages expect them. Some configs
# (e.g. the azurelocal edge variant) do not emit every azlosguard artifact, so
# copy each only when present rather than hard-failing the build.
for artifact in release-notes.txt bcc-tools-installation.log image-bom.json vhd-build-performance-data.json; do
    src="$AGENTBAKER_DIR/vhdbuilder/packer/imagecustomizer/$CONFIG/out/$artifact"
    if [ -f "$src" ]; then
        cp "$src" "$AGENTBAKER_DIR"
    else
        echo "Note: build artifact '$artifact' not produced by config '$CONFIG', skipping copy."
    fi
done

{
  echo "Install completed successfully on " $(date)
  echo "VSTS Build NUMBER: ${BUILD_NUMBER:-}"
  echo "VSTS Build ID: ${BUILD_ID:-}"
  echo "Commit: ${COMMIT:-}"
  echo "Hyperv generation: ${HYPERV_GENERATION:-}"
  echo "Feature flags: ${FEATURE_FLAGS:-}"
  echo "FIPS enabled: ${ENABLE_FIPS:-}"
} >> $AGENTBAKER_DIR/release-notes.txt
