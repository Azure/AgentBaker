#!/bin/bash
set -x

source vhdbuilder/scripts/automate_helpers.sh

echo "Build is for release notes is $1"

image_version=$1
build_id=$2
github_access_token=$3
system_access_token=$4

branch_name=releaseNotes/$1
pr_title="ReleaseNotes"

generate_release_notes() {
    go run vhdbuilder/release-notes/autonotes/main.go --build $build_id --date $image_version
}

configure_az_devops $system_access_token
set_git_config
create_branch $branch_name
generate_release_notes
create_pull_request $image_version $github_access_token $branch_name $pr_title