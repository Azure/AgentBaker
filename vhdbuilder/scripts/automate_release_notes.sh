#!/bin/bash
set -x

source vhdbuilder/scripts/automate_helpers.sh

echo "Build is for release notes is $1"

image_version=$1
build_ids=$2

set +x
github_access_token=$3
system_access_token=$4
set -x

branch_name=releaseNotes/$1
pr_title="ReleaseNotes"

generate_release_notes() {
    for build_id in $build_ids; do
        echo $build_id
        included_skus=""
        artifacts=($(az pipelines runs artifact list --run-id $build_id | jq -r '.[].name')) # Retrieve what artifacts were published
        for artifact in "${artifacts[@]}"; do
            if [[ $artifact == *"vhd-release-notes"* ]]; then
                sku=$(echo $artifact | cut -d "-" -f4-) # Format of artifact is vhd-release-notes-<name of sku>
                included_skus+="$sku,"
            fi
        done
        echo "SKUs for release notes are $included_skus"
        go run vhdbuilder/release-notes/autonotes/main.go --build $build_id --date $image_version --include ${included_skus%?}
    done
}

configure_az_devops $system_access_token
set_git_config
create_branch $branch_name
generate_release_notes

set +x
create_pull_request $image_version $github_access_token $branch_name $pr_title