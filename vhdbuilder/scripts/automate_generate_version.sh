#!/bin/bash
set -euxo pipefail

VHD_BUILD_ID="${VHD_BUILD_ID:-""}"
IMAGE_VERSION_OVERRIDE="${IMAGE_VERSION:-""}"

get_image_version_from_publishing_info() {
    for artifact in $(az pipelines runs artifact list --run-id $VHD_BUILD_ID | jq -r '.[].name'); do # Retrieve what artifacts were published
        if [[ $artifact == *"publishing-info"* ]]; then
            # just take the image version from the first publishing-info we find (since they should all be the same)
            # TODO(cameissner): add image version validation to separate validation template
            az pipelines runs artifact download --artifact-name $artifact --path $(pwd) --run-id $VHD_BUILD_ID
            IMAGE_VERSION=$(jq -r .image_version < vhd-publishing-info.json)
            return 0
        fi
    done
}

if [ -n "$IMAGE_VERSION_OVERRIDE" ]; then
    echo "IMAGE_VERSION already has value: $IMAGE_VERSION_OVERRIDE"
    exit 0
fi

if [ -z "$VHD_BUILD_ID" ]; then
    echo "VHD_BUILD_ID must be set in order to set the image version"
    exit 1
fi

get_image_version_from_publishing_info
export IMAGE_VERSION
