#!/bin/bash
set -euxo pipefail

source vhdbuilder/scripts/windows/automate_helpers.sh

az login --identity

pr_purpose="VHDVersion Bumping"

branch_name=""

set +x
github_access_token=$1
set -x

cherry_pick_commit_id=$2

# This function finds the latest windows VHD base Image version from the command az vm image show
find_latest_image_version() {
    latest_image_version_2019=$(az vm image show --urn MicrosoftWindowsServer:WindowsServer:2019-Datacenter-Core-smalldisk:latest --query 'id' -o tsv | awk -F '/' '{print $NF}')
    latest_image_version_2022=$(az vm image show --urn MicrosoftWindowsServer:WindowsServer:2022-Datacenter-Core-smalldisk:latest --query 'id' -o tsv | awk -F '/' '{print $NF}')
    latest_image_version_2022_g2=$(az vm image show --urn MicrosoftWindowsServer:WindowsServer:2022-Datacenter-Core-smalldisk-g2:latest --query 'id' -o tsv | awk -F '/' '{print $NF}')
    latest_image_version_23H2=$(az vm image show --urn MicrosoftWindowsServer:WindowsServer:23h2-datacenter-core:latest --query 'id' -o tsv | awk -F '/' '{print $NF}')
    latest_image_version_23H2_g2=$(az vm image show --urn MicrosoftWindowsServer:WindowsServer:23h2-datacenter-core-g2:latest --query 'id' -o tsv | awk -F '/' '{print $NF}')
    echo "Latest windows 2019 base image version is: ${latest_image_version_2019}"
    echo "Latest windows 2022 base image version is: ${latest_image_version_2022}"
    echo "Latest windows 2022 Gen 2 base image version is: ${latest_image_version_2022_g2}"
    echo "Latest windows 23H2 base image version is ${latest_image_version_23H2}"
    echo "Latest windows 23H2 Gen 2 base image version is: ${latest_image_version_23H2_g2}"
    new_image_version=$(date +"%Y-%m")
    branch_name=imageBump/win-${new_image_version}b
}

# This function replaces the old Windows 2019 & Windows 2022 (gen1/gen2) base image version with the latest version found by az vm image show in windows-image.env
update_image_version() {
    line=$(grep "WINDOWS_2019_BASE_IMAGE_VERSION=" vhdbuilder/packer/windows/windows-image.env)
    sed -i "s/$line/WINDOWS_2019_BASE_IMAGE_VERSION=$latest_image_version_2019/g" vhdbuilder/packer/windows/windows-image.env

    line=$(grep "WINDOWS_2022_BASE_IMAGE_VERSION=" vhdbuilder/packer/windows/windows-image.env)
    sed -i "s/$line/WINDOWS_2022_BASE_IMAGE_VERSION=$latest_image_version_2022/g" vhdbuilder/packer/windows/windows-image.env

    line=$(grep "WINDOWS_2022_GEN2_BASE_IMAGE_VERSION=" vhdbuilder/packer/windows/windows-image.env)
    sed -i "s/$line/WINDOWS_2022_GEN2_BASE_IMAGE_VERSION=$latest_image_version_2022_g2/g" vhdbuilder/packer/windows/windows-image.env

    line=$(grep "WINDOWS_23H2_BASE_IMAGE_VERSION=" vhdbuilder/packer/windows/windows-image.env)
    sed -i "s/$line/WINDOWS_23H2_BASE_IMAGE_VERSION=$latest_image_version_23H2/g" vhdbuilder/packer/windows/windows-image.env

    line=$(grep "WINDOWS_23H2_GEN2_BASE_IMAGE_VERSION=" vhdbuilder/packer/windows/windows-image.env)
    sed -i "s/$line/WINDOWS_23H2_GEN2_BASE_IMAGE_VERSION=$latest_image_version_23H2_g2/g" vhdbuilder/packer/windows/windows-image.env
}

cherry_pick() {
    if [ -n "$1" ]; then
        echo "Cherry-picked commit id is \"$1\""
        git cherry-pick $1
    fi
}

create_image_bump_pr() {
    if [ `git branch --list $branch_name` ]; then
        git checkout master
        git pull -p
        git checkout $branch_name
    else
        create_branch $branch_name
    fi
    if [[ -n "$cherry_pick_commit_id" ]]; then
        cherry_pick "$cherry_pick_commit_id"
    fi
    update_image_version

    set +x
    create_pull_request $new_image_version $github_access_token $branch_name $pr_purpose
    set -x
}

set_git_config
find_latest_image_version
create_image_bump_pr
