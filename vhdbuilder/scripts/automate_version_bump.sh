#!/bin/bash
set -x

source vhdbuilder/scripts/automate_helpers.sh

echo "New image version: $1"

current_image_version=""
new_image_version=$1

set +x
github_access_token=$2
set -x

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

find_current_image_version "pkg/agent/datamodel/osimageconfig.go"
set_git_config
create_branch $branch_name
update_image_version

set +x
create_pull_request $new_image_version $github_access_token $branch_name $pr_title