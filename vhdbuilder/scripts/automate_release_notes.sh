#!/bin/bash
set -euxo pipefail

SKIP_LATEST="${SKIP_LATEST:-false}"

source vhdbuilder/scripts/automate_helpers.sh

echo "Image version for release notes is $1"

image_version=$1
build_ids=$2

set +x
github_access_token=$3
set -x

branch_name=releaseNotes/$1
pr_title="ReleaseNotes"

generate_release_notes() {
    for build_id in $build_ids; do
        echo $build_id
        included_skus=""
        for artifact in $(az pipelines runs artifact list --run-id $build_id | jq -r '.[].name'); do    # Retrieve what artifacts were published
            if [[ $artifact == *"vhd-release-notes"* ]]; then
                sku=$(echo $artifact | cut -d "-" -f4-) # Format of artifact is vhd-release-notes-<name of sku>
                included_skus+="$sku,"
            fi
        done
        echo "SKUs for release notes are $included_skus"
        if [ "${SKIP_LATEST,,}" == "true" ]; then
            go run vhdbuilder/release-notes/autonotes/main.go --skip-latest --build $build_id --date $image_version --include ${included_skus%?}
        else
            go run vhdbuilder/release-notes/autonotes/main.go --build $build_id --date $image_version --include ${included_skus%?}
        fi
    done
}

set_git_config
if [ `git branch --list $branch_name` ]; then
    git checkout $branch_name
    git pull origin
    git checkout master -- .
else
    create_branch $branch_name
fi

generate_release_notes
git status
set +x
create_pull_request $image_version $github_access_token $branch_name $pr_title