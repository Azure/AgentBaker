#!/usr/bin/env bash
set -euxo pipefail

CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-docker}"

function generate_image_bom_for_containerd() {
    IFS_backup=$IFS
    IFS=$'\n'
    ctr_list=$(ctr --namespace k8s.io image list | sed 1d | awk '{print $1, $3}')
    crictl_list=$(crictl images --no-trunc | sed 1d | awk '{printf "%s:%s %s\n", $1,$2,$3;}')

    touch /opt/azure/containers/images.json
    for image in $ctr_list; do
        tag=$(echo $image | awk '{print $1}')
        # skip image tags that consist of a sha256 hash
        [[ $tag == sha256* ]] && continue
        digest=$(echo $image | awk '{print $2}')
        id=$(echo "$crictl_list" | grep -e "$tag" | awk '{print $2}')

        jq --arg repoTag "$tag" --arg repoDigest "$digest" --arg id "$id" -n '{id:$id, repoTags:[$repoTag], repoDigests:[$repoDigest]}' >> /opt/azure/containers/images.json
    done

    IFS=$IFS_backup
    jq --slurpfile images images.json -n '$images | group_by(.id) | map({id:.[0].id, repoTags:map(.repoTags | add) | unique, repoDigests:map(.repoDigests | add) | unique})' > /opt/azure/containers/image-bom.json
    rm -f /opt/azure/containers/images.json
    jq < /opt/azure/containers/image-bom.json
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

