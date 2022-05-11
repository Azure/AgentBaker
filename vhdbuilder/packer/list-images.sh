#!/usr/bin/env bash
set -euxo 

CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-docker}"

if [[ ${CONTAINER_RUNTIME} == "containerd" ]]; then
    crictl images -o json | jq '.runtime = "containerd"' > /opt/azure/containers/image-bom.json
elif [[ ${CONTAINER_RUNTIME} == "docker" ]]; then
    docker images --all --digests --format "{{json .}}" | jq --slurp | jq '{"images": ., "runtime": "docker"}' > /opt/azure/containers/image-bom.json
else
    echo "Unknown container runtime: ${CONTAINER_RUNTIME}"
fi
