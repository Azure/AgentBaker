#!/bin/bash
set -euxo pipefail

build_ids=$1
#global_image_version=""
for build_id in $build_ids; do
    for artifact in $(az pipelines runs artifact list --run-id $build_id | jq -r '.[].name'); do    # Retrieve what artifacts were published
        if [[ $artifact == *"publishing-info"* ]]; then
            az pipelines runs artifact download --artifact-name $artifact --path $(pwd) --run-id $build_id
            current_image_version=$(cat vhd-publishing-info.json | jq -r .image_version)
            echo "current_image_version is $current_image_version"
            if [[ -z $global_image_version ]]; then
                global_image_version=$current_image_version
            elif [[ $global_image_version != $current_image_version ]]; then
                echo "Image versions mismatch for $build_id and $artifact"
                exit 1
            else
                echo "Image versions match, check next"
            fi
        fi
    done
done

#export global_image_version
rm -rf vhd-publishing-info.json