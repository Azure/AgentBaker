#!/bin/bash
set -uexo pipefail

VHD_BUILD_ID="${VHD_BUILD_ID:-""}"

get_image_version_from_publishing_info() {
    # Note that we previously would specify potentially more than one build, rather than just VHD_BUILD_ID
    # when doing so, we made sure that all publishing-info's coming from the different builds had the same image version.
    # this was needed when we previously released VHDs as a single batch from multiple builds.
    # if we ever need to do that again we should add back that validation.
    for artifact in $(az pipelines runs artifact list --run-id $VHD_BUILD_ID | jq -r '.[].name'); do # Retrieve what artifacts were published
        if [[ $artifact == *"publishing-info"* ]]; then
            # just take the image version from the first publishing-info we find (since they should all be the same)
            # TODO(cameissner): add image version validation to separate validation template
            az pipelines runs artifact download --artifact-name $artifact --path $(pwd) --run-id $VHD_BUILD_ID
            GENERATED_IMAGE_VERSION=$(jq -r .image_version < vhd-publishing-info.json)
            rm -f vhd-publishing-info.json
            return 0
        fi
    done
    echo "unable to find image version from publishing-info artifacts downloaded from VHD build with ID: $VHD_BUILD_ID"
    return 1
}

if [ -z "$VHD_BUILD_ID" ]; then
    echo "VHD_BUILD_ID must be set in order to set the image version"
    exit 1
fi

get_image_version_from_publishing_info || exit $?
export GENERATED_IMAGE_VERSION="$GENERATED_IMAGE_VERSION"
