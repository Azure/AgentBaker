#!/usr/bin/env bash
set -euxo pipefail

CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-docker}"

if [[ ${CONTAINER_RUNTIME} == "containerd" ]]; then
    crictl images -o json
    crictl images -o json | jq '[.images | .[] | {id:.id, repoTags:.repoTags, repoDigests:.repoDigests}]' > /opt/azure/containers/image-bom.json
elif [[ ${CONTAINER_RUNTIME} == "docker" ]]; then
    docker inspect $(docker images -aq) -f '{"id":"{{.ID}}","repoTags":{{json .RepoTags}},"repoDigests":{{json .RepoDigests}}}' | jq --slurp . > /opt/azure/containers/image-bom.json
else
    echo "Unknown container runtime: ${CONTAINER_RUNTIME}"
    exit 1
fi

chmod a+r /opt/azure/containers/image-bom.json
