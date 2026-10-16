#!/bin/bash
set -uxo pipefail

[ -z "${CAPTURED_SIG_VERSION:-}" ] && echo "CAPTURED_SIG_VERSION is not set" && exit 1
[ -z "${BUILD_RUN_NUMBER:-}" ] && echo "BUILD_RUN_NUMBER is not set" && exit 1
[ "${PREFETCH_DELIVERY_COMPLETE:-}" != "True" ] && echo "VHD delivery and gallery publication must both complete before cleanup" && exit 1

main() {
    cleanup_image_builder_rg
}

cleanup_image_builder_rg() {
    local group_exists
    IMAGE_BUILDER_RG_NAME="image-builder-${CAPTURED_SIG_VERSION}-${BUILD_RUN_NUMBER}"
    group_exists="$(az group exists -g "${IMAGE_BUILDER_RG_NAME}")" || return $?
    if [ "${group_exists}" = "true" ]; then
        local template_name="template-${CAPTURED_SIG_VERSION}-${BUILD_RUN_NUMBER}"
        template_name="${template_name:0:64}"
        az image builder delete -g "${IMAGE_BUILDER_RG_NAME}" -n "${template_name}" || return $?
        echo "deleting image builder resource group: ${IMAGE_BUILDER_RG_NAME}"
        if ! az group delete -g "${IMAGE_BUILDER_RG_NAME}" --yes --no-wait; then
            echo "unable to delete image builder resource group: ${IMAGE_BUILDER_RG_NAME}" >&2
            return 1
        fi
    elif [ "${group_exists}" != "false" ]; then
        echo "Unable to determine whether image builder resource group exists" >&2
        return 1
    fi
}

main "$@"
