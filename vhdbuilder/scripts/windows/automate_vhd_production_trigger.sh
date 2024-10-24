#!/bin/bash
set -euxo pipefail

source vhdbuilder/scripts/windows/automate_helpers.sh

az login --identity

DRY_RUN=$1
BUILD_DATE=$2
truncated_build_date=$(echo $BUILD_DATE | tr -d '.')
release_branch_name=windows/v$truncated_build_date
github_user_name=$3
github_access_token=$4

create_release_branch() {
     # BUILD_DATE defaults to YYYY.MM.DD (i.e. $(date +%Y.%m.%d)) if not explicitly set in pipeline variable $CUSTOM_BUILD_DATE
    if [ `git branch --list $release_branch_name` ]; then
        echo "Release branch $release_branch_name already existed, checking out ..."
        git checkout $branch_name
    else
        echo "Creating branch named $release_branch_name"
        git fetch origin master
        git checkout master
        git pull
        git checkout -b $release_branch_name
        git branch --set-upstream-to=origin/master $release_branch_name
        git remote set-url origin https://$github_user_name:$github_access_token@github.com/Azure/AgentBaker.git  # Set remote URL with PAT
        git push -f --set-upstream origin $release_branch_name
    fi
}

trigger_win_vhd_prod_pipeline() {
    pipeline_id=188674 # 182855 - AKS Windows VHD Build - PR check-in gate; 188674 - AKS Windows VHD Build -ContainerD&Docker
    # [TO-DO for production] --parameters dryrun=$DRY_RUN 
    run_id=$(az pipelines run --id $pipeline_id --branch $release_branch_name | jq -r '.id')    
    echo "Pipeline is running at: https://msazure.visualstudio.com/CloudNativeCompute/_build/results?buildId=$run_id&view=results"
}

trigger_build_latest_e2e_image() {
    run_id=$(az pipelines run --id 210712 --branch $release_branch_name --variables SIG_GALLERY_NAME=AKSWindows SIG_IMAGE_NAME_PREFIX=windows-e2e-test SIG_IMAGE_VERSION=$BUILD_DATE | jq -r '.id')
    echo "Pipeline is running at: https://msazure.visualstudio.com/CloudNativeCompute/_build/results?buildId=$run_id&view=results"
}

set_git_config $github_user_name
create_release_branch
trigger_win_vhd_prod_pipeline
trigger_build_latest_e2e_image