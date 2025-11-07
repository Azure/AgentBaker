#!/bin/bash
set -x

IMAGE_BUILDER_RG_NAME="image-builder-${CAPTURED_SIG_VERSION}-${BUILD_RUN_NUMBER}"
if [ "$(az group exists -g "${IMAGE_BUILDER_RG_NAME}")" = "true" ]; then
    echo "Deleting image builder resource group: ${IMAGE_BUILDER_RG_NAME}"
    if ! az group delete -g "${IMAGE_BUILDER_RG_NAME}" --yes --no-wait; then
        echo "unable to delete image builder resource group: ${IMAGE_BUILDER_RG_NAME}, will still proceed"
    fi
fi
