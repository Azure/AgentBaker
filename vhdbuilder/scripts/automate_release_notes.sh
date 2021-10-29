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
    echo $build_ids
    for build_id in $build_ids; do
        echo $build_id
        included_skus=""
        artifacts=($(az pipelines runs artifact list --run-id $build_id | jq -r '.[].name'))
        for artifact in "${artifacts[@]}"; do
            sku=$(echo $artifact | cut -d "-" -f4-)
            included_skus+=",$sku"
        done
        echo "skus for release notes are $included_skus"
        go run vhdbuilder/release-notes/autonotes/main.go --build $build_id --date $image_version --include $included_skus
    done
    #go run vhdbuilder/release-notes/autonotes/main.go --build $build_id --date $image_version
}

configure_az_devops $system_access_token
set_git_config
create_branch $branch_name
generate_release_notes

set +x
create_pull_request $image_version $github_access_token $branch_name $pr_title