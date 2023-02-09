#!/usr/bin/env bash
set -euxo pipefail

CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-docker}"

function generate_image_bom_for_containerd() {
    temp_image_bom=/opt/azure/containers/temp-image-bom.json
    IFS_backup=$IFS
    IFS=$'\n'
    ctr_list=$(ctr -n k8s.io image list | sed 1d | awk '{print $1, $3}')
    digests=$(echo "$ctr_list" | awk '{print $2}' | xargs -n1 | sort -u)

    for digest in $digests; do
        digest_entries=$(echo "$ctr_list" | grep -e "$digest")
        tags=$(echo "$digest_entries" | awk '{print $1}' | grep -v "sha256")
        id=$(echo "$digest_entries" | awk '{print $1}' | grep "sha256")

        jq --arg tags "$tags" --arg digest "$digest" --arg id "$id" -n '{id:$id, repoTags:$tags | split("\n"), repoDigests:[$digest]}' >> $temp_image_bom
    done

    IFS=$IFS_backup
    jq --slurpfile images $temp_image_bom -n '$images | group_by(.id) | map({id:.[0].id, repoTags:.[0].repoTags | unique, repoDigests:map(.repoDigests | add) | unique})' > /opt/azure/containers/image-bom.json
    rm -f $temp_image_bom
}

if [[ ${CONTAINER_RUNTIME} == "containerd" ]]; then
    generate_image_bom_for_containerd
elif [[ ${CONTAINER_RUNTIME} == "docker" ]]; then
    docker inspect $(docker images -aq) -f '{"id":"{{.ID}}","repoTags":{{json .RepoTags}},"repoDigests":{{json .RepoDigests}}}' | jq --slurp . > /opt/azure/containers/image-bom.json
else
    echo "Unknown container runtime: ${CONTAINER_RUNTIME}"
    exit 1
fi

chmod a+r /opt/azure/containers/image-bom.json