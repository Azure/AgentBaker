#!/bin/bash
#set -x

current_image_version=""
new_image_version="2021.10.23"
find_current_image_version() {
    filepath=$1
    flag=0
    while read p; do
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
}

update_image_version() {
    sed -i '' "s/${current_image_version}/${new_image_version}/g" pkg/agent/datamodel/osimageconfig.go
    sed -i '' "s/${current_image_version}/${new_image_version}/g" pkg/agent/bakerapi_test.go
    sed -i '' "s/${current_image_version}/${new_image_version}/g" pkg/agent/datamodel/sig_config.go
    sed -i '' "s/${current_image_version}/${new_image_version}/g" pkg/agent/datamodel/sig_config_test.go
}

echo "Starting script"
# git branch
# git status
# git log --pretty=oneline
find_current_image_version "pkg/agent/datamodel/osimageconfig.go"
echo $current_image_version
update_image_version



