#!/bin/bash

set -euxo pipefail

source e2e-helper.sh

log "Starting to create windows nodepool"

windowsNodepool="win19"
export windowsNodepool

# az aks nodepool add --resource-group $RESOURCE_GROUP_NAME --cluster-name $CLUSTER_NAME --name $windowsNodepool --os-type Windows --node-count 1

log "Created windows nodepool"
out=$(az aks nodepool list --cluster-name $CLUSTER_NAME -g $RESOURCE_GROUP_NAME | grep "$windowsNodepool")

if [ "$out" == "" ]; then
    log "Creating windows nodepool"
    az aks nodepool add --resource-group $RESOURCE_GROUP_NAME --cluster-name $CLUSTER_NAME --name $windowsNodepool --os-type Windows --node-count 1
    log "Created windows nodepool"
else
    log "Already create windows nodepool"
fi

