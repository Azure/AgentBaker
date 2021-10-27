#!/bin/bash
set -x

echo "Build is for release notes is $1"

image_version=$1
build_id=$2
github_access_token=$3
system_access_token=$4

configure_az_devops() {
    az extension add -n azure-devops
    echo ${system_access_token} | az devops login
    az devops configure --defaults organization=https://dev.azure.com/msazure project=CloudNativeCompute
}

set_git_config() {
    git config --global user.email "amaheshwari@microsoft.com"
    git config --global user.name "anujmaheshwari1"
    git config --list
}

create_notes_branch() {
    git checkout master
    git pull
    git checkout -b releaseNotes/$image_version
}

create_pull_request() {
    git remote set-url origin https://anujmaheshwari1:$github_access_token@github.com/Azure/AgentBaker.git
    git add .
    git commit -m "Release notes for release ${image_version}"
    git push -u origin releaseNotes/$image_version
    curl \
        -X POST \
        https://api.github.com/repos/Azure/AgentBaker/pulls \
        -d '{"head" : "releaseNotes/'$image_version'", "base" : "master", "title" : "Automated PR for release notes"}' \
        -u "anujmaheshwari1:$github_access_token"
}

generate_release_notes() {
    go run vhdbuilder/release-notes/autonotes/main.go --build $build_id --date $image_version
}

configure_az_devops
set_git_config
create_notes_branch
generate_release_notes
create_pull_request