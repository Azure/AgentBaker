#!/bin/bash
set -euxo pipefail

RELEASE_ASSISTANT_APP_NAME="aks-node-sig-release-assistant[bot]"
RELEASE_ASSISTANT_APP_UID="190555641"

set_git_config() {
    # git config needs to be set in the agent as the corresponding GitHub app
    # https://github.com/orgs/community/discussions/24664#discussioncomment-3244950
    git config --global user.email "${RELEASE_ASSISTANT_APP_UID}+${RELEASE_ASSISTANT_APP_NAME}@users.noreply.github.com"
    git config --global user.name "$RELEASE_ASSISTANT_APP_NAME"
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
    post_purpose=$1
    branch_name=$2
    image_version=$3
    title=$4
    body_content=$5
    labels=$6
    
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
    echo "Title is $title"
    echo "Head is $branch_name"
    echo "Body is $body_content"

    result=$(curl -H "Authorization: token $github_access_token" \
        -H "Accept: application/vnd.github.v3+json" \
        "https://api.github.com/repos/Azure/AgentBaker/pulls?state=open&head=Azure:$branch_name" |\
        jq --arg title "$title" '.[] | select(.title == $title)')

    if [[ -n $result ]]; then
        echo "Pull request at head '$branch_name' with title \"$title\" existed already"
        number=$(echo $result | jq '.number')
        echo "The existing pull request is at https://github.com/Azure/AgentBaker/pull/$number"
        echo "Error: you cannot $post_purpose for $image_version twice"
        exit 1
    else
        response=$(curl -X POST \
            -H "Authorization: token $github_access_token" \
            -H "Content-Type: application/json" \
            -d "{
                \"title\": \"$title\",
                \"body\": \"$body_content\",
                \"head\": \"$branch_name\",
                \"base\": \"master\"
            }" \
            https://api.github.com/repos/Azure/AgentBaker/pulls)

        number=$(echo $response | jq '.number')
        echo "The pull request number is $number"
        echo "The pull request link is https://github.com/Azure/AgentBaker/pull/$number"

        curl -X POST \
        -H "Authorization: token $github_access_token" \
        -H "Content-Type: application/json" \
        -d "{
            \"labels\": [$labels]
        }" \
        https://api.github.com/repos/Azure/AgentBaker/issues/$number/labels
        echo "Added the label"
    fi
    # set -x
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

    # use the installation token to authenticate for HTTP-based git access
    # https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/authenticating-as-a-github-app-installation#about-authentication-as-a-github-app-installation
    git remote set-url origin https://x-access-token:${github_access_token}@github.com/Azure/AgentBaker.git
    git add .

    if [[ $pr_purpose == "ReleaseNotes" ]]; then
        post_purpose="update windows release notes"
        title="docs: $post_purpose for ${image_version}B"
        labels="\"windows\",\"documentation\""
    else
        post_purpose="bump windows image version"
        title="feat: $post_purpose for ${image_version}B"
        labels="\"windows\", \"VHD\""
    fi
    
    echo "to add git commit feat: $post_purpose for $image_version"
    git commit -m "feat: $post_purpose for $image_version"
        
    git status
    
    git push -u origin $branch_name -f

    # modify .github/PULL_REQUEST_TEMPLATE.md after pushing the pervious changes in created branch
    if [[ $pr_purpose == "ReleaseNotes" ]]; then
        sed -i "/What type of PR is this?/a\/kind documentation" .github/PULL_REQUEST_TEMPLATE.md
        sed -i "/What this PR does/a\Add windows image release notes for new AKS Windows images with ${image_version}B. Reference: #[xxxxx]." .github/PULL_REQUEST_TEMPLATE.md
        sed -i 's/\[ \] uses/\[x\] uses/g' .github/PULL_REQUEST_TEMPLATE.md
        sed -i 's/\[ \] includes/\[x\] includes/g' .github/PULL_REQUEST_TEMPLATE.md
        body_content=$(sed 's/$/\\n/' .github/PULL_REQUEST_TEMPLATE.md | tr -d '\n')
    else
        sed -i "/What type of PR is this?/a\/kind feature" .github/PULL_REQUEST_TEMPLATE.md
        sed -i "/What this PR does/a\Update Windows base images to ${image_version}B\\n- Windows 2019: [xxxxx]()\\n- Windows 2022: [xxxxx]()\\n- Windows 23H2: [xxxxx]()" .github/PULL_REQUEST_TEMPLATE.md
        sed -i 's/\[ \] uses/\[x\] uses/g' .github/PULL_REQUEST_TEMPLATE.md
        body_content=$(sed 's/$/\\n/' .github/PULL_REQUEST_TEMPLATE.md | tr -d '\n')
    fi

    curl_post_request "$post_purpose" "$branch_name" "$image_version" "$title" "$body_content" "$labels"

    git checkout master # Checkout to master for subsequent stages of the pipeline
}