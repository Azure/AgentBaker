#!/bin/bash
set -uxo pipefail

[ -z "${CAPTURED_SIG_VERSION:-}" ] && echo "CAPTURED_SIG_VERSION is not set" && exit 1
[ -z "${BUILD_RUN_NUMBER:-}" ] && echo "BUILD_RUN_NUMBER is not set" && exit 1

cleanup_image_builder_rg() {
    IMAGE_BUILDER_RG_NAME="image-builder-${CAPTURED_SIG_VERSION}-${BUILD_RUN_NUMBER}"
    if [ "$(az group exists -g "${IMAGE_BUILDER_RG_NAME}")" = "true" ]; then
        echo "deleting image builder resource group: ${IMAGE_BUILDER_RG_NAME}"
        if ! az group delete -g "${IMAGE_BUILDER_RG_NAME}" --yes --no-wait; then
            echo "unable to delete image builder resource group: ${IMAGE_BUILDER_RG_NAME}, will still proceed"
        fi
    fi
}

main "$@"
