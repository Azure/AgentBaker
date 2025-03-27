#!/bin/bash
set -uexo pipefail

VHD_BUILD_ID="${VHD_BUILD_ID:-""}"

get_image_version_from_publishing_info() {
    local unique_image_version=""
    local artifacts_with_version_mismatch=""

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
                echo "$artifact has image version $version, though expected all publishing info artifacts to have image version $unique_image_version for VHD build with ID: $VHD_BUILD_ID"
                artifacts_with_version_mismatch="${artifacts_with_version_mismatch} $artifact"
            fi
        fi
    done

    if [ -n "$artifacts_with_version_mismatch" ]; then
        echo "expected all publishing info aritfacts to have image version $unique_image_version for VHD build with ID: $VHD_BUILD_ID"
        echo "the following publishing info artifacts had mismatching image versions: $artifacts_with_version_mismatch"
        echo "##vso[task.logissue type=error]image version mismatch: A NEW VHD BUILD WILL BE REQUIRED!"
        exit 1
    fi

    if [ -z "$unique_image_version" ]; then
        echo "unable to find image version from publishing-info artifacts downloaded from VHD build with ID: $VHD_BUILD_ID"
        exit 1
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
