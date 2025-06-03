#!/usr/bin/env bash
set -euxo pipefail

IMAGE_BOM_PATH=/opt/azure/containers/image-bom.json

SKU_NAME="${SKU_NAME:=}"
IMAGE_VERSION="${IMAGE_VERSION:-$(date +%Y%m.%d.0)}"
CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-containerd}"

if [ -z "${SKU_NAME}" ]; then
    echo "SKU_NAME must be set when generating image list"
    exit 1
fi

if [ "${CONTAINER_RUNTIME}" != "containerd" ]; then
    echo "unknown container runtime: ${CONTAINER_RUNTIME}, expected containerd"
    exit 1
fi

if [ ! -f "/home/packer/lister" ]; then
    echo "could not find lister binary at /home/packer/lister needed to generate image bom for containerd"
    exit 1
fi

echo "Generating image-bom with IMAGE_VERSION=${IMAGE_VERSION}"
pushd /home/packer
    chmod +x lister
    ./lister --sku "$SKU_NAME" --node-image-version "$IMAGE_VERSION" --output-path "$IMAGE_BOM_PATH" || exit $?
popd

chmod a+r $IMAGE_BOM_PATH
