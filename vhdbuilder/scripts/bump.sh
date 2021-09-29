#!/bin/bash
set -x

echo "Starting script"
echo "First arg is $1"
git branch
git status
git log --pretty=oneline

git config --list

git checkout master
git pull
git checkout -b testBranch00
git log --pretty=oneline

filepath=pkg/agent/datamodel/osimageconfig.go
flag=0
image_version=""
new_version="2021.09.25"
while read p; do
    if [[ $p == *":"* ]]; then
        image_variable=$(echo $p | awk -F: '{print $1}')
        image_value=$(echo $p | awk -F'\"' '{print $2}')
        if [[ $flag == 0 ]]; then
            if [[ $image_value == "aks" ]]; then
                flag=1
            fi
        fi

        if [[ $flag == 1 ]] && [[ $image_variable == "ImageVersion" ]]; then
            image_version=$image_value
            flag=0
            break
        fi
    fi
done < $filepath

echo $image_version
sed -i "s/${image_version}/${new_version}/g" $filepath

git config --global user.email "amaheshwari@microsoft.com"
git config --global user.name "anujmaheshwari1"
#git config --global github.user "anujmaheshwari1"
#git config --global url."git@github.com:".insteadOf "https://github.com/"
git remote set-url origin https://anujmaheshwari1:$1@github.com/Azure/AgentBaker.git
git config --list

git status
git add .
git commit -m "will this work?"
git push -u origin testBranch00
