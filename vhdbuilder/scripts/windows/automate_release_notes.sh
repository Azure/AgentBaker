#!/bin/bash
set -euxo pipefail

source vhdbuilder/scripts/windows/automate_helpers.sh

set +x
github_access_token=$1
set -x

build_id=$2
echo "Build Id is $build_id"


image_version=$(date +"%Y-%m")
branch_name=releaseNotes/win-${image_version}b-release-notes

pr_purpose="ReleaseNotes"

set_git_config
if [ `git branch --list $branch_name` ]; then
    git checkout $branch_name
    git pull origin
    git checkout master -- .
else
    create_branch $branch_name
fi

git status
set +x
create_pull_request $image_version $github_access_token $branch_name $pr_purpose