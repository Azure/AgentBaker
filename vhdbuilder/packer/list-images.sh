#!/usr/bin/env bash
set -euxo pipefail

IMAGE_BOM_PATH=/opt/azure/containers/image-bom.json

SKU_NAME="${SKU_NAME:=}"
IMAGE_VERSION="${IMAGE_VERSION:-$(date +%Y%m.%d.0)}"
CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-containerd}"

if [[ -z "${SKU_NAME}" ]]; then
    echo "SKU_NAME must be set when generating image list"
    exit 1
fi

function generate_image_bom_for_containerd() {
    if [ ! -f "/home/packer/lister" ]; then
        echo "could not find lister binary at /home/packer/lister needed to generate image bom for containerd"
        exit 1
    fi

    pushd /home/packer
        chmod +x lister
        ./lister --sku "$SKU_NAME" --node-image-version "$IMAGE_VERSION" --output-path "$IMAGE_BOM_PATH" || exit $?
    popd
}

function generate_image_bom_for_docker() {
    docker inspect $(docker images -aq) -f '{"id":"{{.ID}}","repoTags":{{json .RepoTags}},"repoDigests":{{json .RepoDigests}}}' | jq --slurp . | jq  'map({id:.id, repoTags:.repoTags, repoDigests:.repoDigests | map(split("@")[1])})' > $IMAGE_BOM_PATH
}

echo "Generating image-bom with IMAGE_VERSION=${IMAGE_VERSION}"

if [[ ${CONTAINER_RUNTIME} == "containerd" ]]; then
    generate_image_bom_for_containerd
elif [[ ${CONTAINER_RUNTIME} == "docker" ]]; then
    generate_image_bom_for_docker
else
    echo "Unknown container runtime: ${CONTAINER_RUNTIME}"
    exit 1
fi

chmod a+r $IMAGE_BOM_PATH
