#!/bin/bash
KUBE_BINARY_VERSIONS="$(jq -r .kubernetes.versions[] parts/linux/cloud-init/artifacts/manifest.json)"
echo $KUBE_BINARY_VERSIONS
for KUBE_BINARY_VERSION in ${KUBE_BINARY_VERSIONS}; do
    echo $KUBE_BINARY_VERSION | cut -f1,2 -d '.'
done
