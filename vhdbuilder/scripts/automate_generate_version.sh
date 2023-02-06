#!/bin/bash
set -euxo pipefail

build_ids=$1
global_image_version="${IMAGE_VERSION:=}"
for build_id in $build_ids; do
    #artifacts=()
    #while IFS='' read -r line; do artifacts+=("$line"); done < <(az pipelines runs artifact list --run-id $build_id | jq -r '.[].name')
    az pipelines runs artifact list --run-id $build_id --debug
    artifacts=($(az pipelines runs artifact list --run-id $build_id | jq -r '.[].name'))
    artifacts=(${artifacts[@]//vhd*})
    printf '%s\n' "${artifacts[@]}"
    for artifact in ${artifacts[@]}; do    # Retrieve what artifacts were published
        # This loop is because of how the Image Version is set for builds. 
        # It uses the UTC time of when the build for a particular SKU ends. 
        # So in the past, it has happened that you trigger a build at say 3/4pm PST, 
        # some SKUs will have todays date some will have tomorrows based on when they are triggered because of UTC conversion
        # TODO(amaheshwari): Change VHD script to use a common var for image version that is plumbed down to all SKUs
        #if [[ $artifact == *"publishing-info"* ]]; then
            az pipelines runs artifact download --artifact-name $artifact --path $(pwd) --run-id $build_id --debug
            current_image_version=$(jq -r .image_version < vhd-publishing-info.json)
            if [[ $global_image_version != $current_image_version ]]; then
                if [[ -z $global_image_version ]]; then
                    global_image_version=$current_image_version
                else 
                    echo "mismatched image, exiting"
                    exit 1
                fi
            fi
        #fi
    done
done

echo $global_image_version
rm -rf vhd-publishing-info.json