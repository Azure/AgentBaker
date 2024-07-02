#!/bin/bash
set -euxo pipefail
source vhdbuilder/scripts/automate_helpers.sh

set +x
GITHUB_PAT="${GITHUB_PAT:-""}"
set -x

NEW_IMAGE_VERSION="${IMAGE_VERSION:-""}"
VHD_BUILD_ID="${VHD_BUILD_ID:-""}"

# This function takes the build ID and reads the queue time.
# It then sanitizes the queue time in the format that the Canonical snapshot endpoint expects, which is 20230727T000000Z 
# Following that, it will write the timestamp to a JSON file to be consumed later in the event of a hotfix
# This file is only written to an official branch, because the build timestamp will correspond with a particular VHD
find_and_write_build_timestamp() {
    build_time=$(az pipelines runs show --id $VHD_BUILD_ID | jq -r ".queueTime")
    canonical_sanitized_timestamp=$(date -u -d "$build_time" "+%Y%m%dT%H%M%SZ")
    # shellcheck disable=SC2089
    json_string="{\"build_timestamp\": \"$canonical_sanitized_timestamp\"}"
    echo "$json_string" > vhdbuilder/${NEW_IMAGE_VERSION}_build_timestamp.json
}

parse_and_write_base_image_version() {
  # Use the VHD_BUILD_ID to go through publishing-info.json
  # If 20.04 or higher, parse the base_image_version and append it to a json file
  # Skip for others
  base_image_version_json_file="vhdbuilder/${NEW_IMAGE_VERSION}_base_image_version.json"
  if [ ! -f "$base_image_version_json_file" ]; then
    echo "{}" > "$base_image_version_json_file"
  fi

  artifact_name=$(az pipelines runs artifact list --run-id ${VHD_BUILD_ID} | jq -r '.[].name' | grep "publishing-info" | awk '/2004|2204/')
  artifacts=()
  while IFS= read -r line; do
    artifacts+=("$line")
  done <<< "$artifact_name"
  for artifact in "${artifacts[@]}"; do
    az pipelines runs artifact download --artifact-name $artifact --path $(pwd) --run-id ${VHD_BUILD_ID}
    BASE_IMAGE_VERSION=$(jq -r .base_image_version < vhd-publishing-info.json)
    SKU_NAME=$(jq -r .sku_name < vhd-publishing-info.json)
    jq --arg base_image_version "${BASE_IMAGE_VERSION}" --arg sku_name "${SKU_NAME}" '.[$sku_name] = $base_image_version' "$base_image_version_json_file" > tmp.json && mv tmp.json "$base_image_version_json_file"
    rm -f vhd-publishing-info.json
  done
}

# This function finds the current SIG Image version from the input JSON file
find_current_image_version() {
    filepath=$1
    CURRENT_IMAGE_VERSION=$(jq -r .version $filepath)
    echo "Current image version is: ${CURRENT_IMAGE_VERSION}"
}

# This function replaces the old image version with the new input image version for all relevant files
update_image_version() {
    sed -i "s/${CURRENT_IMAGE_VERSION}/${NEW_IMAGE_VERSION}/g" pkg/agent/datamodel/linux_sig_version.json
}

create_image_bump_pr() {
    create_branch $BRANCH_NAME
    update_image_version

    set +x
    create_pull_request $NEW_IMAGE_VERSION $GITHUB_PAT $BRANCH_NAME $PR_TITLE
    set -x
}

# This function cuts the official branch based off the commit ID that the builds were triggered from and tags it
cut_official_branch() {
    # Image version format: YYYYMM.DD.revision
    # Official branch format: official/vYYYYMMDD
    # Official tag format: v0.YYYYMMDD.revision
    parsed_image_version="$(echo -n "${NEW_IMAGE_VERSION}" | head -c-1 | tr -d .)"
    official_branch_name="official/v${parsed_image_version}"
    official_tag="v0.${parsed_image_version}.0"

    build_commit_hash=$(az pipelines runs show --id $VHD_BUILD_ID | jq -r '.sourceVersion')
    echo "official commit hash is: $build_commit_hash"

    # Checkout branch and commit the image bump file diff to official branch too
    if [ `git branch -r | grep $official_branch_name` ]; then
        git checkout $official_branch_name
        git pull origin
    else
        git checkout -b $official_branch_name $build_commit_hash
    fi
    update_image_version
    git add .
    git commit -m "chore: update image version in official branch"

    # Avoid including release notes in the official tag
    rm -rf vhdbuilder/release-notes
    git add .
    git commit -m "chore: remove release notes in official branch"
    
    # Compute and store the VHD build timestamp for hotfixes
    find_and_write_build_timestamp
    # Parse and store the publisher base image versions for hotfixes
    parse_and_write_base_image_version
    git add .
    git commit -m "chore: store VHD build timestamp and publisher base image version in official branch"

    git push -u origin $official_branch_name

    git tag $official_tag
    git push origin tag $official_tag -f
    git checkout master
}

set +x
if [ -z "$GITHUB_PAT" ]; then
    echo "GITHUB_PAT must be set in order to bump the image version and create the official branch"
    exit 1
fi
set -x

if [ -z "$NEW_IMAGE_VERSION" ]; then
    echo "NEW_IMAGE_VERSION must be set in order to bump the image version and create the official branch"
    exit 1
fi

if [ -z "$VHD_BUILD_ID" ]; then
    echo "VHD_BUILD_ID must be set in order to bump the image version and create the official branch"
    exit 1
fi

echo "New image version: $NEW_IMAGE_VERSION"
BRANCH_NAME=imageBump/$NEW_IMAGE_VERSION
PR_TITLE="VHDVersion"

set_git_config
find_current_image_version "pkg/agent/datamodel/linux_sig_version.json"
create_image_bump_pr
cut_official_branch
