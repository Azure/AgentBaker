#!/bin/bash
set -x

set_git_config() {
    # git config needs to be set in the agent
    git config --global user.email "wanqingfu@microsoft.com"
    git config --global user.name "wanqingfu"
    git config --list
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

    git remote set-url origin https://wanqingfu:$2@github.com/Azure/AgentBaker.git  # Set remote URL with PAT
    git add .

    if [[ $4 == "ReleaseNotes" ]]; then
        echo "to add git commit chore: release notes for release $1"
        git commit -m "chore: release notes for release $1"
    else
        echo "to add git commit chore: bumping windows image version to $1"
        git commit -m "chore: bumping windows image version to $1"
    fi

    git push -u origin $3 -f

    set +x  # To avoid logging PAT during curl
    curl -X POST \
        -H "Authorization: token $2" \
        -H "Content-Type: application/json" \
        -d '{
            "title": "chore: automated PR to update '$4' for '$1' windows VHD",
            "body": "This is an automated PR to '$4' for the windows VHD release with image version '$1'",
            "head": "'$3'",
            "base": "master"
        }' \
        https://api.github.com/repos/Azure/AgentBaker/pulls
    set -x

    git checkout master # Checkout to master for subsequent stages of the pipeline
}