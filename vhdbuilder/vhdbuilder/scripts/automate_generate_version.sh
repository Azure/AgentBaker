#!/bin/bash
set -uexo pipefail

VHD_BUILD_ID="${VHD_BUILD_ID:-""}"

get_image_version_from_publishing_info() {
    local unique_image_version=""

    for artifact in $(az pipelines runs artifact list --run-id $VHD_BUILD_ID | jq -r '.[].name'); do # Retrieve what artifacts were published
        if [[ $artifact == *"publishing-info"* ]]; then
            # remove publishing info downloaded from some previous iteration, if it exists
            rm -f vhd-publishing-info.json

            az pipelines runs artifact download --artifact-name $artifact --path $(pwd) --run-id $VHD_BUILD_ID
            version=$(jq -r .image_version < vhd-publishing-info.json)
            if [ -z "$unique_image_version" ]; then
                unique_image_version=$version
                continue
            fi

            if [ "$version" != "$unique_image_version" ]; then
                # this is to ensure that all publishing-infos coming from the same VHD build specify the same image_version,
                # mismatching image_versions will cause problems downstream in the release process
                echo "mismatched image version for VHD build with ID: $VHD_BUILD_ID"
                echo "expected publishing info artifact $artifact to have image_version $unique_image_version, but had: $version"
                echo "a new VHD build will be required"
                exit 1
            fi
        fi
    done

    if [ -z "$unique_image_version" ]; then
        echo "unable to find image version from publishing-info artifacts downloaded from VHD build with ID: $VHD_BUILD_ID"
        return 1
    fi

    # remove any dangling publishing info
    rm -f vhd-publishing-info.json

    GENERATED_IMAGE_VERSION=$unique_image_version
}

if [ -z "$VHD_BUILD_ID" ]; then
    echo "VHD_BUILD_ID must be set in order to set the image version"
    exit 1
fi

get_image_version_from_publishing_info || exit $?
export GENERATED_IMAGE_VERSION="$GENERATED_IMAGE_VERSION"
