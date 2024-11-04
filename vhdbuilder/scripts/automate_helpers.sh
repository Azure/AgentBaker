#!/bin/bash
set -x

retrycmd_if_failure() {
    retries=$1; wait_sleep=$2; shift && shift
    for i in $(seq 1 $retries); do
        "${@}" && break || \
        echo "Failed to execute command \"$@\""
        if [ $i -eq $retries ]; then
            echo "ERROR: Exhausted all retries (${i}/{$retries}), forcing a failure..."
            return 1
        else
            echo "$(($retries - $i)) retries remaining"
            sleep $wait_sleep
        fi
    done
    echo Executed \"$@\" $i times;
}

set_git_config() {
    # git config needs to be set in the agent
    git config --global user.email "aks-node@microsoft.com"
    git config --global user.name "aks-node"
    git config --list
}

create_branch() {
    local branch_name=$1
    local base_branch=$2
    if [ -z "$base_branch" ]; then
        echo "create_branch: base_branch not specified, will default to master"
        base_branch="master"
    fi

    # Create PR branch
    echo "Create branch named $branch_name off of $base_branch"
    git checkout $base_branch
    git pull
    git checkout -b $branch_name
}

create_pull_request() {
    local image_version=$1
    local github_pat=$2
    local branch_name=$3
    local base_branch=$4
    local target=$5
    if [ -z "$base_branch" ]; then
        echo "create_pull_request: base_branch not specified, will default to master"
        base_branch="master"
    fi

    # Commit current changes and create PR using curl
    echo "Image Version is $image_version"
    echo "Branch Name is $branch_name"
    echo "PR is for $target"

    set +x # to avoid logging PAT
    git remote set-url origin https://${github_pat}@github.com/Azure/AgentBaker.git  # Set remote URL with PAT
    git add .
    
    if [[ "$target" == "ReleaseNotes" ]]; then
        git commit -m "chore: release notes for release $image_version"
    else
        git commit -m "chore: bumping image version to $image_version"
    fi

    git push -u origin $branch_name -f

    curl \
        -X POST \
        -H "Authorization: Bearer $github_pat"
        https://api.github.com/repos/Azure/AgentBaker/pulls \
        -d '{
            "head" : "'$branch_name'", 
            "base" : "'$base_branch'", 
            "title" : "chore: automated PR to update '$target' for '$image_version' VHD", 
            "body" : "This is an automated PR to bump '$target' for the VHD release with image version '$image_version'"
        }' \
        
    set -x
    git checkout master # Checkout to master for subsequent stages of the pipeline
}