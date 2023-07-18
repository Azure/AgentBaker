#!/bin/bash
set -euxo pipefail

source vhdbuilder/scripts/windows/automate_helpers.sh

az login --identity

pr_title="VHDVersion Bumping"

latest_image_version_2019=""
latest_image_version_2022=""
branch_name=""

set +x
github_access_token=$1
set -x

# This function finds the latest windows VHD base Image version from the command az vm image show
find_latest_image_version() {
    latest_image_version_2019=$(az vm image show --urn MicrosoftWindowsServer:WindowsServer:2019-Datacenter-Core-smalldisk:latest --query 'id' -o tsv | awk -F '/' '{print $NF}')
    latest_image_version_2022=$(az vm image show --urn MicrosoftWindowsServer:WindowsServer:2022-Datacenter-Core-smalldisk:latest --query 'id' -o tsv | awk -F '/' '{print $NF}')
    echo "Latest windows 2019 base image version is: ${latest_image_version_2019}"
    echo "Latest windows 2022 base image version is: ${latest_image_version_2022}"
    new_image_version=$(echo "$latest_image_version_2019" | cut -c 12-)
    branch_name=wsimageBump/$new_image_version
}

# This function replaces the old Windows 2019 & Windows 2022 base image version with the latest version found by az vm image show in windows-image.env
update_image_version() {
    line=$(grep "WINDOWS_2019_BASE_IMAGE_VERSION=" vhdbuilder/packer/windows-image.env)
    echo $line
    sed -i "s/$line/WINDOWS_2019_BASE_IMAGE_VERSION=$latest_image_version_2019/g" vhdbuilder/packer/windows-image.env

    line=$(grep "WINDOWS_2022_BASE_IMAGE_VERSION=" vhdbuilder/packer/windows-image.env)
    echo $line
    sed -i "s/$line/WINDOWS_2022_BASE_IMAGE_VERSION=$latest_image_version_2022/g" vhdbuilder/packer/windows-image.env

    line=$(grep "WINDOWS_2022_GEN2_BASE_IMAGE_VERSION=" vhdbuilder/packer/windows-image.env)
    echo $line
    sed -i "s/$line/WINDOWS_2022_GEN2_BASE_IMAGE_VERSION=$latest_image_version_2022/g" vhdbuilder/packer/windows-image.env

    # Jul 18, 2023: the version in windows-image.env is already up-to-date (7B)
    # to test the above scripts, the author has created a fake windows-image.env with old image version (from June 2023)
    # and run the shell in local environment. "cat windows-image.env" showed that the version was updated to *.230710 successfully.
    echo >> vhdbuilder/packer/windows-image.env
}

create_image_bump_pr() {
    create_branch $branch_name
    update_image_version

    set +x
    create_pull_request $new_image_version $github_access_token $branch_name $pr_title
    set -x
}

set_git_config
find_latest_image_version
create_image_bump_pr
