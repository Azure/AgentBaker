#!/bin/bash
set -uexo pipefail

VHD_BUILD_ID="${VHD_BUILD_ID:-""}"
GENERATED_IMAGE_VERSION=""

get_image_version_from_publishing_info() {
    # Note that we previously would specify potentially more than one build, rather than just VHD_BUILD_ID
    # when doing so, we made sure that all publishing-info's coming from the different builds had the same image version.
    # this was needed when we previously released VHDs as a single batch from multiple builds.
    # if we ever need to do that again we should add back that validation.
    for artifact in $(az pipelines runs artifact list --run-id $VHD_BUILD_ID | jq -r '.[].name'); do
        mkdir -p artifacts
        az pipelines runs artifact download --artifact-name $artifact --path artifacts --run-id $VHD_BUILD_ID
        if [ -f "artifacts/vhd-publishing-info.json" ]; then
            GENERATED_IMAGE_VERSION=$(jq -r .image_version < artifacts/vhd-publishing-info.json)
            break
        fi
        rm -rf artifacts
    done
    
    # make absolutely sure we aren't leaving anything around we don't want
    rm -f ./*vhd-publishing-info.json
    rm -rf artifacts

    if [ -z "$GENERATED_IMAGE_VERSION" ]; then
        echo "unable to find image version from publishing-info artifacts downloaded from VHD build with ID: $VHD_BUILD_ID"
        return 1
    fi
}

if [ -z "$VHD_BUILD_ID" ]; then
    echo "VHD_BUILD_ID must be set in order to set the image version"
    exit 1
fi

get_image_version_from_publishing_info || exit $?
export GENERATED_IMAGE_VERSION="$GENERATED_IMAGE_VERSION"
