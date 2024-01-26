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

az pipelines runs artifact download --run-id $build_id --artifact-name publishing-info-2019-containerd --path .
image_version=$(jq '.image_version' vhd-publishing-info.json)
image_version=$(echo $image_version | rev | cut -c 2-7 | rev)
rm vhd-publishing-info.json
branch_name=winreleaseNotes/$image_version

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