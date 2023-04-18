#!/bin/bash

set -euxo pipefail

source e2e-helper.sh

log "Starting to create windows nodepool"

if echo "$windowsPackageURL" | grep -q "hotfix"; then
    RESOURCE_GROUP_NAME="$RESOURCE_GROUP_NAME-$WINDOWS_E2E_IMAGE-$K8S_VERSION-h"
else
    RESOURCE_GROUP_NAME="$RESOURCE_GROUP_NAME-$WINDOWS_E2E_IMAGE-$K8S_VERSION"
fi

out=$(az aks nodepool list --cluster-name $CLUSTER_NAME -g $RESOURCE_GROUP_NAME | jq '.[].name')

if [[ "$out" != *"winnp"* ]]; then
    log "Creating windows nodepool"
    retval=0
    az aks nodepool add --resource-group $RESOURCE_GROUP_NAME --cluster-name $CLUSTER_NAME --name "winnp" --os-type Windows --os-sku $WINDOWS_E2E_OSSKU --node-vm-size $WINDOWS_E2E_VMSIZE --node-count 1 || retval=$?
    if [ "$retval" -ne 0 ]; then
        log "Other pipeline may be creating the same nodepool, waiting for ready"
    else
        log "Created windows nodepool"
    fi
else
    log "Already create windows nodepool"
fi

provisioning_state=$(az aks nodepool show --cluster-name $CLUSTER_NAME -g $RESOURCE_GROUP_NAME -n "winnp" -ojson | jq '.provisioningState' | tr -d "\"")
if [ "$provisioning_state" == "Creating" ]; then
    log "Other pipeline may be creating the same nodepool, waiting for ready"
    az aks nodepool wait --nodepool-name "winnp" --cluster-name $CLUSTER_NAME --resource-group $RESOURCE_GROUP_NAME --created --interval 60 --timeout 1800
fi

provisioning_state=$(az aks nodepool show --cluster-name $CLUSTER_NAME -g $RESOURCE_GROUP_NAME -n "winnp" -ojson | jq '.provisioningState' | tr -d "\"")
if [ "$provisioning_state" == "Succeeded" ]; then
    log "Windows nodepool is in succeed state"
else
    err "Windows nodepool is not in succeeded state. Current state is $provisioning_state."
    exit 1
fi
