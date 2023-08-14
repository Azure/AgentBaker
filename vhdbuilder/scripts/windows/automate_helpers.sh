#!/bin/bash
set -euxo pipefail

set_git_config() {
    # git config needs to be set in the agent
    github_user_name=$1
    git config --global user.email "$github_user_name@microsoft.com"
    git config --global user.name "$github_user_name"
    git config --list
}

create_branch() {
    # Create PR branch
    echo "Create branch named $1"
    git fetch origin master
    git checkout master
    git pull
    git checkout -b $1
}

cherry_pick() {
    if [ -n "$1" ]; then
        echo "Cherry-picked commit id is \"$1\""
        git cherry-pick $1
    fi
}

create_pull_request() {
    # Commit current changes and create PR using curl
    github_access_token=$2
    echo "Image Version is $1"
    echo "Branch Name is $3"
    echo "PR is for $4"

    git remote set-url origin https://$github_user_name:$github_access_token@github.com/Azure/AgentBaker.git  # Set remote URL with PAT
    git add .
    git status

    if [[ $4 == "ReleaseNotes" ]]; then
        echo "to add git commit chore: windows release notes for release $1"
        git commit -m "chore: windows release notes for release $1"
    else
        echo "to add git commit chore: bumping windows image version to $1"
        git commit -m "chore: bumping windows image version to $1"
    fi

    git push -u origin $3 -f

    set +x  # To avoid logging PAT during curl
    
    # check if the pull request already existed in case of validation failure below
    # {
    #     "message": "Validation Failed",
    #     "errors": [
    #     {
    #       "resource": "PullRequest",
    #       "code": "custom",
    #       "message": "A pull request already exists for Azure:wsimageBump/230707."
    #     }
    #    ],
    #    "documentation_url": "https://docs.github.com/rest/pulls/pulls#create-a-pull-request"
    # }
    if [[ $4 == "ReleaseNotes" ]]; then
        result=$(curl -H "Authorization: token $github_access_token" \
        -H "Accept: application/vnd.github.v3+json" \
        "https://api.github.com/repos/Azure/AgentBaker/pulls?state=open&head=Azure:$3" |\
        jq '.[] | select(.title == "chore: automated PR to update windows release notes for version '$1'")')
        if [[ -n $result ]]; then
            echo "Pull request at head '$3' with title \"chore: automated PR to update windows release notes for version '$1'\" existed already"
            echo "Error: you cannot update release notes twice"
            exit 1
        else
            curl -X POST \
                -H "Authorization: token $github_access_token" \
                -H "Content-Type: application/json" \
                -d '{
                    "title": "chore: automated PR to update windows release notes for version '$1'",
                    "body": "This is an automated PR to write windows VHD release notes for image version '$1'",
                    "head": "'$3'",
                    "base": "master"
                }' \
                https://api.github.com/repos/Azure/AgentBaker/pulls
        fi
    else
        echo "1"
        result=$(curl -H "Authorization: token $github_access_token" \
        -H "Accept: application/vnd.github.v3+json" \
        "https://api.github.com/repos/Azure/AgentBaker/pulls?state=open&head=Azure:$3" |\
        jq '.[] | select(.title == "chore: automated PR to bump windows image version to '$1'")')
        echo "2"
        if [[ -n $result ]]; then
            echo "Pull request at head '$3' with title \"chore: automated PR to bump windows image version to '$1'\" existed already"
            echo "Error: you cannot run image version bumping twice"
            exit 1
        else
            curl -X POST \
                -H "Authorization: token $github_access_token" \
                -H "Content-Type: application/json" \
                -d '{
                    "title": "chore: automated PR to bump windows image version to '$1'",
                    "body": "This is an automated PR to write windows VHD release notes for image version '$1'",
                    "head": "'$3'",
                    "base": "master"
                }' \
                https://api.github.com/repos/Azure/AgentBaker/pulls

        fi
    fi

   
     
    set -x

    git checkout master # Checkout to master for subsequent stages of the pipeline
}