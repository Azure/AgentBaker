#!/usr/bin/env bash
set -euxo pipefail

CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-docker}"

TEMP_IMAGE_BOM_PATH=/opt/azure/containers/temp-image-bom.json
IMAGE_BOM_PATH=/opt/azure/containers/image-bom.json

function generate_image_bom_for_containerd() {
    IFS_backup=$IFS; IFS=$'\n'
    ctr_list=$(ctr -n k8s.io image list | sed 1d | awk '{print $1, $3}')
    digests=$(echo "$ctr_list" | awk '{print $2}' | xargs -n1 | sort -u)

    for digest in $digests; do
        digest_entries=$(echo "$ctr_list" | grep -e "$digest")
        tags=$(echo "$digest_entries" | awk '{print $1}' | grep -v "sha256")
        id=$(echo "$digest_entries" | awk '{print $1}' | grep -e "sha256")

        jq --arg tags "$tags" --arg digest "$digest" --arg id "$id" -n '{id:$id, repoTags:$tags | split("\n"), repoDigests:[$digest]}' >> $TEMP_IMAGE_BOM_PATH
    done

    IFS=$IFS_backup
    jq --slurpfile images $TEMP_IMAGE_BOM_PATH -n '$images | group_by(.id) | map({id:.[0].id, repoTags:[.[].repoTags] | add | unique, repoDigests:[.[].repoDigests] | add | unique})' > $IMAGE_BOM_PATH
    rm -f $TEMP_IMAGE_BOM_PATH
}

function generate_image_bom_for_docker() {
    docker inspect $(docker images -aq) -f '{"id":"{{.ID}}","repoTags":{{json .RepoTags}},"repoDigests":{{json .RepoDigests}}}' | jq --slurp . | jq  'map({id:.id, repoTags:.repoTags, repoDigests:.repoDigests | map(split("@")[1])})' > $IMAGE_BOM_PATH
}

if [[ ${CONTAINER_RUNTIME} == "containerd" ]]; then
    generate_image_bom_for_containerd
elif [[ ${CONTAINER_RUNTIME} == "docker" ]]; then
    generate_image_bom_for_docker
else
    echo "Unknown container runtime: ${CONTAINER_RUNTIME}"
    exit 1
fi

chmod a+r $IMAGE_BOM_PATH