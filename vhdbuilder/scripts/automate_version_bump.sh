#!/bin/bash
set -x

source vhdbuilder/scripts/automate_helpers.sh

echo "New image version: $1"

current_image_version=""
new_image_version=$1

# set +x
# github_access_token=$2
# set -x

build_id=$3
official_tag=$4

branch_name=imageBump/$new_image_version
pr_title="VersionBump"

# This function finds the current SIG Image version from pkg/agent/datamodelosimageconfig.go
find_current_image_version() {
    filepath=$1
    flag=0
    while read -r p; do
        if [[ $p == *":"* ]]; then
            image_variable=$(echo $p | awk -F: '{print $1}')
            image_value=$(echo $p | awk -F'\"' '{print $2}')
            if [[ $flag == 0 ]]; then
                if [[ $image_value == "aks" ]]; then
                    flag=1
                fi
            fi

            if [[ $flag == 1 ]] && [[ $image_variable == "ImageVersion" ]]; then
                current_image_version=$image_value
                flag=0
                break
            fi
        fi
    done < $filepath
    echo "Current image version is: ${current_image_version}"
}

# This function replaces the old image version with the new input image version for all relevant files
update_image_version() {
    sed -i "s/${current_image_version}/${new_image_version}/g" pkg/agent/datamodel/osimageconfig.go
    sed -i "s/${current_image_version}/${new_image_version}/g" pkg/agent/bakerapi_test.go
    sed -i "s/${current_image_version}/${new_image_version}/g" pkg/agent/datamodel/sig_config.go
    sed -i "s/${current_image_version}/${new_image_version}/g" pkg/agent/datamodel/sig_config_test.go
}

create_image_bump_pr() {
    create_branch $branch_name
    update_image_version

    set +x
    create_pull_request $new_image_version $github_access_token $branch_name $pr_title
    set -x
}

cut_official_branch() {
    commit_hash=$(az pipelines runs show --id $build_id | jq -r '.sourceVersion')
    git checkout -b official/v$new_image_version $commit_hash
    update_image_version
    git add .
    git commit -m"Update image version in official branch"
    git tag $official_tag
    git push
}

set_git_config
find_current_image_version "pkg/agent/datamodel/osimageconfig.go"
create_image_bump_pr
