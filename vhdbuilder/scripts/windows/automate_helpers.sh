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
    echo "Creating branch named $1"
    git fetch origin master
    git checkout master
    git pull
    git checkout -b $1
}

curl_post_request() {
    set +x # To avoid logging PAT during curl
    post_purpose=$1
    branch_name=$2
    image_version=$3
    
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

    result=$(curl -H "Authorization: token $github_access_token" \
        -H "Accept: application/vnd.github.v3+json" \
        "https://api.github.com/repos/Azure/AgentBaker/pulls?state=open&head=Azure:$branch_name" |\
        jq '.[] | select(.title == "chore: automated PR to '"$post_purpose"' for '$image_version'")')
    
    if [[ -n $result ]]; then
        echo "Pull request at head '$branch_name' with title \"chore: automated PR to '$post_purpose' for '$image_version'\" existed already"
        number=$(echo $result | jq '.number')
        echo "The existing pull request is at https://github.com/Azure/AgentBaker/pull/$number"
        echo "Error: you cannot $post_purpose for $image_version twice"
        exit 1
    else
        response=$(curl -X POST \
            -H "Authorization: token $github_access_token" \
            -H "Content-Type: application/json" \
            -d '{
                "title": "chore: automated PR to '"$post_purpose"' for '$image_version'",
                "body": "This is an automated PR to '"$post_purpose"' for '$image_version'",
                "head": "'$branch_name'",
                "base": "master"
            }' \
            https://api.github.com/repos/Azure/AgentBaker/pulls)

        number=$(echo $response | jq '.number')
        echo "The pull request number is $number"
        echo "The pull request link is https://github.com/Azure/AgentBaker/pull/$number"
    fi
    set -x
}

create_pull_request() {
    # Commit current changes and create PR using curl
    image_version=$1
    github_access_token=$2
    branch_name=$3
    pr_purpose=$4
    echo "Image Version is $image_version"
    echo "Branch Name is $branch_name"
    echo "PR is for $pr_purpose"

    git remote set-url origin https://$github_user_name:$github_access_token@github.com/Azure/AgentBaker.git  # Set remote URL with PAT
    git add .

    if [[ $pr_purpose == "ReleaseNotes" ]]; then
        post_purpose="update windows release notes"
    else
        post_purpose="bump windows image version"
    fi
    
    echo "to add git commit chore: $post_purpose for $image_version"
    git commit -m "chore: $post_purpose for $image_version"
        
    git status
    
    git push -u origin $branch_name -f

    curl_post_request "$post_purpose" "$branch_name" "$image_version"

    git checkout master # Checkout to master for subsequent stages of the pipeline
}