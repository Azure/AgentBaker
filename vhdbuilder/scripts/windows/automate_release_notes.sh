#!/bin/bash
set -euxo pipefail

source vhdbuilder/scripts/windows/automate_helpers.sh

az login --identity

echo "Build Id is $1"
 
build_id=$1

set +x
github_access_token=$2
set -x

github_user_name=$3

image_version=$(date +"%Y-%m")
branch_name=$github_user_name/win-${image_version}b-release-notes

pr_purpose="ReleaseNotes"

set_git_config $github_user_name
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