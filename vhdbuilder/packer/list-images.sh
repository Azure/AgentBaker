#!/usr/bin/env bash
set -euxo pipefail

IMAGE_BOM_PATH=/opt/azure/containers/image-bom.json

SKU_NAME="${SKU_NAME:=}"
IMAGE_VERSION="${IMAGE_VERSION:-$(date +%Y%m.%d.0)}"

if [ -z "${SKU_NAME}" ]; then
    echo "SKU_NAME must be set when generating image list"
    exit 1
fi

if [ ! -x "/opt/azure/containers/image-fetcher" ]; then
    echo "could not find executable image-fetcher at /opt/azure/containers/image-fetcher needed to generate image bom for containerd"
    exit 1
fi

echo "Generating image-bom with IMAGE_VERSION=${IMAGE_VERSION}"
/opt/azure/containers/image-fetcher list-images \
    --sku "$SKU_NAME" --node-image-version "$IMAGE_VERSION" --output-path "$IMAGE_BOM_PATH"

chmod a+r $IMAGE_BOM_PATH
chmod a+rx /opt/azure/ /opt/azure/containers/
