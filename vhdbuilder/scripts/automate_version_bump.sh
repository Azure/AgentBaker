#!/bin/bash
set -x

echo "Starting script"
echo "New image version: $1"
current_image_version=""
new_image_version=$1
pat=$2

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
    echo "Current image version is: ${current_image_version}"
}

update_image_version() {
    sed -i "s/${current_image_version}/${new_image_version}/g" pkg/agent/datamodel/osimageconfig.go
    sed -i "s/${current_image_version}/${new_image_version}/g" pkg/agent/bakerapi_test.go
    sed -i "s/${current_image_version}/${new_image_version}/g" pkg/agent/datamodel/sig_config.go
    sed -i "s/${current_image_version}/${new_image_version}/g" pkg/agent/datamodel/sig_config_test.go
}

set_git_config() {
    git config --global user.email "amaheshwari@microsoft.com"
    git config --global user.name "anujmaheshwari1"
    git config --list
}

create_bump_branch() {
    git checkout master
    git pull
    git checkout -b imageBump/$new_image_version
}

create_pull_request() {
    git remote set-url origin https://anujmaheshwari1:$pat@github.com/Azure/AgentBaker.git
    git add .
    git commit -m "Bumping image version to ${new_image_version}"
    git push -u origin imageBump/$new_image_version
    curl \
        -X POST \
        https://api.github.com/repos/Azure/AgentBaker/pulls \
        -d '{"head" : "imageBump/'$new_image_version'", "base" : "master", "title" : "Automated PR for version bump"}' \
        -u "anujmaheshwari1:$pat"
}

find_current_image_version "pkg/agent/datamodel/osimageconfig.go"
set_git_config
create_bump_branch
update_image_version
create_pull_request