#!/bin/bash
set -euxo pipefail

source vhdbuilder/scripts/windows/automate_helpers.sh

az login --identity

echo "Build Id is $1"

latest_image_version_2019=$(az vm image show --urn MicrosoftWindowsServer:WindowsServer:2019-Datacenter-Core-smalldisk:latest --query 'id' -o tsv | awk -F '/' '{print $NF}')
image_version=$(echo "$latest_image_version_2019" | cut -c 12-)
  
build_id=$1

set +x
github_access_token=$2
set -x

github_user_name=$3

branch_name=releaseNotes/$image_version
pr_title="ReleaseNotes"

set_git_config $github_user_name
if [ `git branch --list $branch_name` ]; then
    git checkout $branch_name
    git pull origin
    git checkout master -- .
else
    create_branch $branch_name
fi

echo "Cherry picking"
echo "commit id is $CHERRY_PICK_COMMIT_ID"
cherry_pick_commit_id=$4
echo "commit id is $cherry_pick_commit_id"
cherry_pick $cherry_pick_commit_id

echo "Running autonotes"
#cd vhdbuilder/release-notes/autonotes
#go install .
#cd ../../..
echo "BUILD_ID" is $BUILD_ID
echo "Build id is $1"
echo "build id is $build_id"
go run vhdbuilder/release-notes/autonotes/main.go --build $build_id --include 2019-containerd,2022-containerd,2022-containerd-gen2
        
git status
set +x
echo "Creating PR for release notes"
create_pull_request $image_version $github_access_token $branch_name $pr_title