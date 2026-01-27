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

BASE_IMAGE_ORAS=$BASE_IMG:$BASE_IMG_VERSION
if [ ! -f "$BUILD_DIR/$CONFIG/image.vhdx" ]; then
    echo "Pulling base image $BASE_IMAGE_ORAS from registry..."
    docker run \
        --rm \
        --interactive \
        --privileged=true \
        -v "$BUILD_DIR:/container/build" \
        $IMG_CUSTOMIZER_CONTAINER:$IMG_CUSTOMIZER_VERSION \
        oras pull $BASE_IMAGE_ORAS -o /container/build/$CONFIG
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
        --output-image-format vhd-fixed \
        --output-image-file /container/out/$CONFIG/"$(basename "$IMAGE_PATH")"

cp $IMAGE_PATH $OUT_DIR/$CONFIG.vhd

# Place build artifacts where later pipeline stages expect them
cp "$AGENTBAKER_DIR/vhdbuilder/packer/imagecustomizer/$CONFIG/out/release-notes.txt" "$AGENTBAKER_DIR"
cp "$AGENTBAKER_DIR/vhdbuilder/packer/imagecustomizer/$CONFIG/out/bcc-tools-installation.log" "$AGENTBAKER_DIR"
cp "$AGENTBAKER_DIR/vhdbuilder/packer/imagecustomizer/$CONFIG/out/image-bom.json" "$AGENTBAKER_DIR"
cp "$AGENTBAKER_DIR/vhdbuilder/packer/imagecustomizer/$CONFIG/out/vhd-build-performance-data.json" "$AGENTBAKER_DIR"

{
  echo "Install completed successfully on " $(date)
  echo "VSTS Build NUMBER: ${BUILD_NUMBER:-}"
  echo "VSTS Build ID: ${BUILD_ID:-}"
  echo "Commit: ${COMMIT:-}"
  echo "Hyperv generation: ${HYPERV_GENERATION:-}"
  echo "Feature flags: ${FEATURE_FLAGS:-}"
  echo "FIPS enabled: ${ENABLE_FIPS:-}"
} >> $AGENTBAKER_DIR/release-notes.txt
