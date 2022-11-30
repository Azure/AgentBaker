#!/bin/bash

set -euxo pipefail

source e2e-helper.sh

log "Starting to check windows nodepool"

windowsNodepool="win19"
export windowsNodepool

out=$(az aks nodepool list --cluster-name $CLUSTER_NAME -g $RESOURCE_GROUP_NAME | grep "$windowsNodepool")
if [ -n "$out"]; then
    log "Already create windows nodepool"
else
    log "Creating windows nodepool"
    az aks nodepool add --resource-group $RESOURCE_GROUP_NAME --cluster-name $CLUSTER_NAME --name $windowsNodepool --os-type Windows --node-count 1
    log "Created windows nodepool"
fi

