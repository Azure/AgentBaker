#!/bin/bash
set -x

echo "Build is for release notes is $1"
image_version=$1
build_id=$2
access_token=$3

echo $image_version
echo $build_id
echo $access_token

configure_az_devops() {
    az extension add -n azure-devops
    az devops configure --defaults organization=https://dev.azure.com/msazure project=CloudNativeCompute
}

create_notes_branch() {
    git checkout master
    git pull
    git checkout -b releaseNotes/$new_image_version
}

create_pull_request() {
    git remote set-url origin https://anujmaheshwari1:$access_token@github.com/Azure/AgentBaker.git
    git add .
    git commit -m "Release notes for release ${new_image_version}"
    git push -u origin releaseNotes/$new_image_version
    curl \
        -X POST \
        https://api.github.com/repos/Azure/AgentBaker/pulls \
        -d '{"head" : "releaseNotes/'$new_image_version'", "base" : "master", "title" : "Automated PR for release notes"}' \
        -u "anujmaheshwari1:$access_token"
}

generate_release_notes() {
    go run vhdbuilder/release-notes/autonotes/main.go --build $build_id --date $image_version
}

configure_az_devops
create_notes_branch
generate_release_notes
create_pull_request