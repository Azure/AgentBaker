#!/bin/bash
set -euo pipefail
source vhdbuilder/scripts/automate_helpers.sh

set +x
GITHUB_TOKEN="${GITHUB_TOKEN:-""}"
set -x

IMAGE_VERSION="${IMAGE_VERSION:-""}"
VHD_BUILD_ID="${VHD_BUILD_ID:-""}"
SKIP_LATEST="${SKIP_LATEST:-false}"
PR_TARGET_BRANCH="${PR_TARGET_BRANCH:-master}"

generate_release_notes() {
    included_skus=""
    for artifact in $(az pipelines runs artifact list --run-id $VHD_BUILD_ID | jq -r '.[].name'); do # Retrieve what artifacts were published
        if [[ $artifact == *"vhd-release-notes"* ]]; then
            sku=$(echo $artifact | cut -d "-" -f4-) # Format of artifact is vhd-release-notes-<name of sku>
            included_skus+="$sku,"
        fi
    done
    echo "SKUs for release notes are $included_skus"
    if [ "${SKIP_LATEST,,}" == "true" ]; then
        go run vhdbuilder/release-notes/autonotes/main.go --skip-latest --build $VHD_BUILD_ID --date $IMAGE_VERSION --include ${included_skus%?}
    else
        go run vhdbuilder/release-notes/autonotes/main.go --build $VHD_BUILD_ID --date $IMAGE_VERSION --include ${included_skus%?}
    fi
    if [ $? -ne 0 ]; then
        echo "running 'git clean -df' before retrying..."
        git clean -df
        return 1
    fi
}

if [ -z "$IMAGE_VERSION" ]; then
    echo "IMAGE_VERSION must be set to generate release notes"
    exit 1
fi

if [ -z "$VHD_BUILD_ID" ]; then
    echo "VHD_BUILD_ID must be set to generate release notes"
fi

set +x
if [ -z "$GITHUB_TOKEN" ]; then
    echo "GITHUB_TOKEN must be set to generate release notes"
fi
set -x

echo "VHD build ID for release notes is: $VHD_BUILD_ID"
echo "Image version for release notes is: $IMAGE_VERSION"

BRANCH_NAME=releaseNotes/$IMAGE_VERSION
PR_TITLE="ReleaseNotes"

set_git_config
if [ `git branch --list $BRANCH_NAME` ]; then
    git checkout $BRANCH_NAME
    git pull origin
    git checkout master -- .
else
    create_branch $BRANCH_NAME $PR_TARGET_BRANCH
fi

retrycmd_if_failure 5 10 generate_release_notes || exit $?
git status
set +x
create_pull_request $IMAGE_VERSION $GITHUB_TOKEN $BRANCH_NAME $PR_TARGET_BRANCH $PR_TITLE