#!/bin/bash
#set -x

current_image_version=""
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

echo "Starting script"
# git branch
# git status
# git log --pretty=oneline
find_current_image_version "pkg/agent/datamodel/osimageconfig.go"
echo $current_image_version


