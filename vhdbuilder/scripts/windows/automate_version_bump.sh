#!/bin/bash
set -euxo pipefail

source vhdbuilder/scripts/windows/automate_helpers.sh

az login --identity

pr_purpose="VHDVersion Bumping"

new_image_version=$(date +"%Y-%m")
branch_name=imageBump/win-${new_image_version}b

set +x
github_access_token=$1
set -x

cherry_pick_commit_id=$2

# This function replaces the old Windows 2019 & Windows 2022 (gen1/gen2) base image version with the latest version found by az vm image show in windows_settings.json
update_image_version() {
  distros=`jq -r ".WindowsBaseVersions | keys  | join(\" \") " < vhdbuilder/packer/windows/windows_settings.json`
  for win_sku in $distros ; do
	  command=`jq -r ".WindowsBaseVersions.\"${win_sku}\".version_update_command" < vhdbuilder/packer/windows/windows_settings.json`
	  base_image_sku=` jq -r ".WindowsBaseVersions.\"${win_sku}\".base_image_sku" < vhdbuilder/packer/windows/windows_settings.json`
    newVersion=`az vm image show --urn MicrosoftWindowsServer:WindowsServer:${base_image_sku}:latest --query name -o tsv`

    jq ".WindowsBaseVersions.\"${win_sku}\".base_image_version = \"${newVersion}\"" < vhdbuilder/packer/windows/windows_settings.json > vhdbuilder/packer/windows/windows_settings_temp.json
    mv vhdbuilder/packer/windows/windows_settings_temp.json vhdbuilder/packer/windows/windows_settings.json
    echo "Found version $newVersion for $base_image_sku"
  done
}

cherry_pick() {
    if [ -n "$1" ]; then
        echo "Cherry-picked commit id is \"$1\""
        git cherry-pick "$1"
    fi
}

create_image_bump_pr() {
    if [ `git branch --list "$branch_name"` ]; then
        git checkout master
        git pull -p
        git checkout "$branch_name"
    else
        create_branch "$branch_name"
    fi
    if [ -n "$cherry_pick_commit_id" ]; then
        cherry_pick "$cherry_pick_commit_id"
    fi
    update_image_version

    set +x
    create_pull_request "$new_image_version" "$github_access_token" "$branch_name" "$pr_purpose"
    set -x
}

set_git_config
create_image_bump_pr
