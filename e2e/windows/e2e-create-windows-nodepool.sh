#!/bin/bash

set -euxo pipefail

source e2e-helper.sh

log "Starting to create windows nodepool"

declare -l E2E_RESOURCE_GROUP_NAME="$AZURE_E2E_RESOURCE_GROUP_NAME-$WINDOWS_E2E_IMAGE$WINDOWS_GPU_DRIVER_SUFFIX-$K8S_VERSION"

out=$(az aks nodepool list --cluster-name $AZURE_E2E_CLUSTER_NAME -g $E2E_RESOURCE_GROUP_NAME | jq '.[].name')

if [[ "$out" != *"winnp"* ]]; then
    log "Creating windows nodepool"
    retval=0
    az aks nodepool add --resource-group $E2E_RESOURCE_GROUP_NAME --cluster-name $AZURE_E2E_CLUSTER_NAME --name "winnp" --os-type Windows --os-sku $WINDOWS_E2E_OSSKU --node-vm-size $WINDOWS_E2E_VMSIZE --node-count 1 || retval=$?
    if [ "$retval" -ne 0 ]; then
        log "Other pipeline may be creating the same nodepool, waiting for ready"
    else
        log "Created windows nodepool"
    fi
else
    log "Already create windows nodepool"
fi

provisioning_state=$(az aks nodepool show --cluster-name $AZURE_E2E_CLUSTER_NAME -g $E2E_RESOURCE_GROUP_NAME -n "winnp" -ojson | jq '.provisioningState' | tr -d "\"")
if [ "$provisioning_state" == "Creating" ]; then
    log "Other pipeline may be creating the same nodepool, waiting for ready"
    az aks nodepool wait --nodepool-name "winnp" --cluster-name $AZURE_E2E_CLUSTER_NAME --resource-group $E2E_RESOURCE_GROUP_NAME --created --interval 60 --timeout 1800
fi

provisioning_state=$(az aks nodepool show --cluster-name $AZURE_E2E_CLUSTER_NAME -g $E2E_RESOURCE_GROUP_NAME -n "winnp" -ojson | jq '.provisioningState' | tr -d "\"")
if [ "$provisioning_state" == "Succeeded" ]; then
    log "Windows nodepool is in succeed state"
else
    err "Windows nodepool is not in succeeded state. Current state is $provisioning_state."
    exit 1
fi
