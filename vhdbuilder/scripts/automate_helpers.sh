#!/bin/bash
set -x

set_git_config() {
    # git config needs to be set in the agent
    git config --global user.email "amaheshwari@microsoft.com"
    git config --global user.name "anujmaheshwari1"
    git config --list
}

configure_az_devops() {
    # Login to azure devops using PAT/System Access Token for artifacts and triggering ev2 builds
    # TODO(amaheshwari): Use only PAT for both artifacts and builds
    az extension add -n azure-devops

    set +x
    echo $1 | az devops login --organization=https://dev.azure.com/msazure
    set -x
    
    az devops configure --defaults organization=https://dev.azure.com/msazure project=CloudNativeCompute
}

create_branch() {
    # Create PR branch
    echo "Create branch named $1"
    git checkout master
    git pull
    git checkout -b $1
}

create_pull_request() {
    # Commit current changes and create PR using curl
    echo "Image Version is $1"
    echo "Branch Name is $3"
    echo "PR is for $4"

    git remote set-url origin https://anujmaheshwari1:$2@github.com/Azure/AgentBaker.git  # Set remote URL with PAT
    git add .
    git commit -m "Bumping image version to $1"
    git push -u origin $3

    set +x  # To avoid logging PAT during curl
    curl \
        -X POST \
        https://api.github.com/repos/Azure/AgentBaker/pulls \
        -d '{"head" : "'$3'", "base" : "master", "title" : "Automated PR for '$4'"}' \
        -u "anujmaheshwari1:$2"
    set -x
    
    git checkout master # Checkout to master for subsequent stages of the pipeline
}