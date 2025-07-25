#!/bin/bash
# Build imagecustomizer image
# Usage: build-imagecustomizer-image.sh <CONFIG>

set -euo pipefail

# Find the absolute path of the directory containing this script
SCRIPTS_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
AGENTBAKER_DIR=`realpath $SCRIPTS_DIR/../../../../`
BUILD_DIR="${AGENTBAKER_DIR}/build"
OUT_DIR="${AGENTBAKER_DIR}/out"
mkdir -p ${OUT_DIR}
mkdir -p ${BUILD_DIR}

CONFIG=$IMG_CUSTOMIZER_CONFIG

# Validate CONFIG and config file
CONFIG_FILE="$AGENTBAKER_DIR/vhdbuilder/packer/imagecustomizer/$CONFIG/$CONFIG.yml"
if [[ ! -f "$CONFIG_FILE" ]]; then
    echo "Error: Config file '$CONFIG_FILE' not found" >&2
    echo "Expected path: vhdbuilder/packer/imagecustomizer/$CONFIG/$CONFIG.yml" >&2
    exit 1
fi

if [[ ! -r "$CONFIG_FILE" ]]; then
    echo "Error: Config file '$CONFIG_FILE' is not readable" >&2
    exit 1
fi

echo "Using following Image Customizer config:"
cat $CONFIG_FILE

IMAGE_PATH="${OUT_DIR}/$CONFIG/$CONFIG.vhd"

BASE_IMAGE_ORAS=$BASE_IMG:$BASE_IMG_VERSION

if [ ! -f "$BUILD_DIR/$CONFIG/image.vhd" ]; then
    echo "Pulling base image from ORAS registry..."
    oras pull $BASE_IMAGE_ORAS -o "$BUILD_DIR/$CONFIG"
else
    echo "Base image already exists, skipping pull."
fi

# Generate repartd configuration files based on the disks section of aks-config.yaml
$SCRIPTS_DIR/generate-repartd.sh $CONFIG_FILE $AGENTBAKER_DIR/parts/linux/cloud-init/artifacts/immutableazl/repart.d

docker run \
    --rm \
    --interactive \
    --tty \
    --privileged=true \
    -e BASE_IMAGE_NAME=linuxguard \
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
        --image-file /container/build/$CONFIG/image.vhd \
        --output-image-format vhd-fixed \
        --output-image-file /container/out/$CONFIG/"$(basename "$IMAGE_PATH")" \
        --rpm-source "/container/config/azurelinux-cloud-native.repo"

cp $IMAGE_PATH $OUT_DIR/$CONFIG.vhd
