#!/bin/bash
set -euxo pipefail

source vhdbuilder/scripts/automate_helpers.sh

echo "New image version: $1"

current_image_version=""
new_image_version=$1

set +x
github_access_token=$2
set -x

build_ids=$3

branch_name=imageBump/$new_image_version
pr_title="VHDVersion"

# This function takes the build ID and reads the queue time.
# It then sanitizes the queue time in the format that the Canonical snapshot endpoint expects, which is 20230727T000000Z 
# Following that, it will write the timestamp to a JSON file to be consumed later in the event of a hotfix
# This file is only written to an official branch, because the build timestamp will correspond with a particular VHD
find_and_write_build_timestamp() {
    first_build=$(echo "$build_ids" | cut -d' ' -f1)
    build_time=$(az pipelines runs show --id $first_build | jq -r ".queueTime")
    canonical_sanitized_timestamp=$(date -u -d "$build_time" "+%Y%m%dT%H%M%SZ")
    # shellcheck disable=SC2089
    json_string="{\"build_timestamp\": \"$canonical_sanitized_timestamp\"}"
    echo "$json_string" > vhdbuilder/${new_image_version}_build_timestamp.json
}

# This function finds the current SIG Image version from the input JSON file
find_current_image_version() {
    filepath=$1
    current_image_version=$(jq -r .version $filepath)
    echo "Current image version is: ${current_image_version}"
}

# This function replaces the old image version with the new input image version for all relevant files
update_image_version() {
    sed -i "s/${current_image_version}/${new_image_version}/g" pkg/agent/datamodel/linux_sig_version.json
}

create_image_bump_pr() {
    create_branch $branch_name
    update_image_version

    set +x
    create_pull_request $new_image_version $github_access_token $branch_name $pr_title
    set -x
}

# This function cuts the official branch based off the commit ID that the builds were triggered from and tags it
cut_official_branch() {
    # Image version format: YYYYMM.DD.revision
    # Official branch format: official/vYYYYMMDD
    # Official tag format: v0.YYYYMMDD.revision
    parsed_image_version="$(echo -n "${new_image_version}" | head -c-1 | tr -d .)"
    official_branch_name="official/v${parsed_image_version}"
    official_tag="v0.${parsed_image_version}.0"
    final_commit_hash=""
    for build_id in $build_ids; do
        current_build_commit_hash=$(az pipelines runs show --id $build_id | jq -r '.sourceVersion')
        if [[ -z "$final_commit_hash" ]]; then
            final_commit_hash=$current_build_commit_hash
        else
            if [[ $final_commit_hash != $current_build_commit_hash ]]; then
                echo "Builds are not triggered off the same commit, exit"
                exit 1
            fi
        fi
    done
    echo "All builds are based off the same commit"

    # Checkout branch and commit the image bump file diff to official branch too
    if [ `git branch -r | grep $official_branch_name` ]; then
        git checkout $official_branch_name
        git pull origin
    else
        git checkout -b $official_branch_name $final_commit_hash
    fi
    update_image_version
    git add .
    git commit -m "chore: update image version in official branch"

    # Avoid including release notes in the official tag
    rm -rf vhdbuilder/release-notes
    git add .
    git commit -m "chore: remove release notes in official branch"
    
    # Compute and store the VHD build timestamp for hotfixes
    find_and_write_build_timestamp
    git add .
    git commit -m "chore: compute and store VHD build timestamp in official branch"

    git push -u origin $official_branch_name

    git tag $official_tag
    git push origin tag $official_tag -f
    git checkout master
}

set_git_config
find_current_image_version "pkg/agent/datamodel/linux_sig_version.json"
create_image_bump_pr
cut_official_branch
