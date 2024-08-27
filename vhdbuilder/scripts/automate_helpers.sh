#!/bin/bash
set -x

retrycmd_if_failure() {
    retries=$1; wait_sleep=$2; shift && shift
    for i in $(seq 1 $retries); do
        "${@}" && break || \
        echo "Failed to execute command \"$@\""
        if [ $i -eq $retries ]; then
            echo "ERROR: Exhausted all retries (${i}/${retries}), forcing a failure..."
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
    git config --global user.email "amaheshwari@microsoft.com"
    git config --global user.name "anujmaheshwari1"
    git config --list
}

create_branch() {
    # Create PR branch
    echo "Create branch named $1"
    git checkout cameissner/bundle-artifacts
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
    
    if [[ $4 == "ReleaseNotes" ]]; then
        git commit -m "chore: release notes for release $1"
    else
        git commit -m "chore: bumping image version to $1"
    fi

    git push -u origin $3 -f

    set +x  # To avoid logging PAT during curl
    curl \
        -X POST \
        https://api.github.com/repos/Azure/AgentBaker/pulls \
        -d '{
            "head" : "'$3'", 
            "base" : "master", 
            "title" : "chore: automated PR to update '$4' for '$1' VHD", 
            "body" : "This is an automated PR to bump '$4' for the VHD release with image version '$1'"
        }' \
        -u "anujmaheshwari1:$2"
    set -x
    
    git checkout master # Checkout to master for subsequent stages of the pipeline
}