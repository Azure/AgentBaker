#!/bin/bash
set -x

set_git_config() {
    git config --global user.email "amaheshwari@microsoft.com"
    git config --global user.name "anujmaheshwari1"
    git config --list
}

configure_az_devops() {
    az extension add -n azure-devops
    echo $1 | az devops login --organization=https://dev.azure.com/msazure
    az devops configure --defaults organization=https://dev.azure.com/msazure project=CloudNativeCompute
}

create_branch() {
    echo "Create branch named $1"
    git checkout master
    git pull
    git checkout -b $1
}

create_pull_request() {
    echo "Image Version is $1"
    echo "Auth Token is $2"
    echo "Branch Name is $3"
    echo "PR Title is $4"
    git remote set-url origin https://anujmaheshwari1:$2@github.com/Azure/AgentBaker.git
    git add .
    git commit -m "Bumping image version to $1"
    git push -u origin imageBump/$1
    curl \
        -X POST \
        https://api.github.com/repos/Azure/AgentBaker/pulls \
        -d '{"head" : "'$3'", "base" : "master", "title" : "'$4'"}' \
        -u "anujmaheshwari1:$2"
    git checkout amaheshwari/automationPipeline # checkout to master when merged to master
}