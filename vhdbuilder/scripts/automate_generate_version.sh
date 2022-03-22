#!/bin/bash
set -euxo pipefail

build_ids=$1
global_image_version=""
for build_id in $build_ids; do
    for artifact in $(az pipelines runs artifact list --run-id $build_id | jq -r '.[].name'); do    # Retrieve what artifacts were published
        if [[ $artifact == *"publishing-info"* ]]; then
            az pipelines runs artifact download --artifact-name $artifact --path $(pwd) --run-id $build_id
            current_image_version=$(jq -r .image_version < vhd-publishing-info.json)
            if [[ $global_image_version != $current_image_version ]]; then
                if [[ -z $global_image_version ]]; then
                    global_image_version=$current_image_version
                else 
                    exit 1
                fi
            fi
        fi
    done
done

echo $global_image_version
rm -rf vhd-publishing-info.json