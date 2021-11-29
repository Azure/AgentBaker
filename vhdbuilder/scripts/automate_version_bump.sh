#!/bin/bash
set -x

source vhdbuilder/scripts/automate_helpers.sh

echo "New image version: $1"

current_image_version=""
new_image_version=$1

set +x
github_access_token=$2
set -x

build_ids=$3

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

# This function cuts the official branch based off the commit ID that the builds were triggered from and tags it
cut_official_branch() {
    official_branch_name="official/v${new_image_version//.}"
    official_tag="v0.${new_image_version//.}.0"
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
    git checkout -b $official_branch_name $final_commit_hash
    update_image_version
    git add .
    git commit -m"Update image version in official branch"
    git push -u origin $official_branch_name

    git tag $official_tag
    git push origin tag $official_tag
    git checkout master

}

set_git_config
find_current_image_version "pkg/agent/datamodel/osimageconfig.go"
create_image_bump_pr
cut_official_branch
