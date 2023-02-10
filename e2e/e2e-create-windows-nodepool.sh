#!/bin/bash

set -euxo pipefail

source e2e-helper.sh

log "Starting to create windows nodepool"

if [[ "$RESOURCE_GROUP_NAME" == *"windows"*  ]]; then
    RESOURCE_GROUP_NAME="$RESOURCE_GROUP_NAME"-"$WINDOWS_E2E_IMAGE"
fi

out=$(az aks nodepool list --cluster-name $CLUSTER_NAME -g $RESOURCE_GROUP_NAME | jq '.[].name')

if [[ "$out" != *"winnp"* ]]; then
    log "Creating windows nodepool"
    if [ "$WINDOWS_E2E_IMAGE" == "2019-containerd" ]; then
        az aks nodepool add --resource-group $RESOURCE_GROUP_NAME --cluster-name $CLUSTER_NAME --name "winnp" --os-type Windows --node-count 1
    elif [ "$WINDOWS_E2E_IMAGE" == "2022-containerd" ]; then
        az aks nodepool add --resource-group $RESOURCE_GROUP_NAME --cluster-name $CLUSTER_NAME --name "winnp" --os-type Windows --os-sku Windows2022 --node-vm-size Standard_D2_v2 --node-count 1
    elif [ "$WINDOWS_E2E_IMAGE" == "2022-containerd-gen2" ]; then
        az aks nodepool add --resource-group $RESOURCE_GROUP_NAME --cluster-name $CLUSTER_NAME --name "winnp" --os-type Windows --os-sku Windows2022 --node-vm-size Standard_DS2_v2 --node-count 1
    else
        err "Invalid windows e2e image version"
        exit 1
    fi
    log "Created windows nodepool"
else
    log "Already create windows nodepool"
fi
