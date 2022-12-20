#!/bin/bash

set -euxo pipefail

source e2e-helper.sh

log "Starting to create windows nodepool"

out=$(az aks nodepool list --cluster-name $CLUSTER_NAME -g $RESOURCE_GROUP_NAME | jq '.[1].name')

if [ "$out" == null ]; then
    log "Creating windows nodepool"
    # If you want to specify the k8s version for windows nodepool, you should also specify the version when creating cluster
    # and modify the value of "customKubeBinaryURL" and "customKubeProxyImage" in property-windows.json
    az aks nodepool add --resource-group $RESOURCE_GROUP_NAME --cluster-name $CLUSTER_NAME --name $WINDOWS_NODEPOOL --os-type Windows --node-count 1
    log "Created windows nodepool"
else
    log "Already create windows nodepool"
fi
