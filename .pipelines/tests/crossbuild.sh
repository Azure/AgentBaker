#!/usr/bin/bash
#set -x
version=$1

REGISTRY=$2
# Define arrays for editions and tags
EDITIONs=("nanoserver" "servercore")

if [ "$version" = "2025" ]; then
    TAGs=("latest")
    dockerfile="buildxdocker2025"
else
    TAGs=("ltsc2019" "ltsc2022")
    dockerfile="buildxdocker"
fi


# Create an empty file named "emptyfile"
touch emptyfile

# Comment out the following as the throtting issue seems to be transient
# To make if work, will need to parse in the PAT as a paramter, there is no env variable set for the bash here
# docker login -u yuazhang -p ${DOCKER_PAT}

docker buildx create --name mybuilder --driver docker-container --use

error=false
# Loop through Windows versions
for TAG in "${TAGs[@]}"; do
    # Loop through editions
    for EDITION in "${EDITIONs[@]}"; do
        echo $TAG, $EDITION
        
        #check the os version info for the build image
        # Build the Docker image
        docker buildx build \
                --build-arg TAG=$TAG \
                --build-arg EDITION=$EDITION \
                --build-arg REGISTRY=$REGISTRY \
                --platform windows/amd64 \
                --builder mybuilder \
                --tag=windows-test \
                --file ./${dockerfile} .
        if [ $? -ne 0 ]; then
                echo "Error: build '$TAG' '$EDITION' failed."
                error=true
        fi
    done
done
if $error; then
    exit 1
else
    exit 0
fi

#set +x