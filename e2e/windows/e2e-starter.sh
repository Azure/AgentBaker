#!/bin/bash

set -euxo pipefail

source e2e-helper.sh

log "Starting e2e tests"

# Create a resource group for the cluster
log "Creating resource group"
declare -l E2E_RESOURCE_GROUP_NAME="$AZURE_E2E_RESOURCE_GROUP_NAME-$WINDOWS_E2E_IMAGE$WINDOWS_GPU_DRIVER_SUFFIX-$K8S_VERSION"

rgStartTime=$(date +%s)
az group create -l $AZURE_BUILD_LOCATION -n $E2E_RESOURCE_GROUP_NAME --subscription $AZURE_E2E_SUBSCRIPTION_ID -ojson
rgEndTime=$(date +%s)
log "Created resource group in $((rgEndTime-rgStartTime)) seconds"

# Check if there exists a cluster in the RG. If yes, check if the MC_RG associated with it still exists.
# MC_RG gets deleted due to ACS-Test Garbage Collection but the cluster hangs around
out=$(az aks list -g $E2E_RESOURCE_GROUP_NAME -ojson | jq '.[].name')
create_cluster="false"
if [ -n "$out" ]; then
    provisioning_state=$(az aks show -n $AZURE_E2E_CLUSTER_NAME -g $E2E_RESOURCE_GROUP_NAME -ojson | jq '.provisioningState' | tr -d "\"")
    MC_RG_NAME="MC_${E2E_RESOURCE_GROUP_NAME}_${AZURE_E2E_CLUSTER_NAME}_$AZURE_BUILD_LOCATION"
    exists=$(az group exists -n $MC_RG_NAME)
    if [ "$exists" == "false" ] || [ "$provisioning_state" == "Failed" ] || [ "$provisioning_state" == "Canceled" ]; then
        # The cluster is in a broken state
        log "Cluster $AZURE_E2E_CLUSTER_NAME is in an unusable state, deleting..."
        clusterDeleteStartTime=$(date +%s)
        az aks delete -n $AZURE_E2E_CLUSTER_NAME -g $E2E_RESOURCE_GROUP_NAME --yes
        clusterDeleteEndTime=$(date +%s)
        log "Deleted cluster $AZURE_E2E_CLUSTER_NAME in $((clusterDeleteEndTime-clusterDeleteStartTime)) seconds"
        create_cluster="true"
    elif [ "$provisioning_state" == "Creating" ]; then
        # Other pipeline is creating this cluster
        log "Cluster $AZURE_E2E_CLUSTER_NAME is being created, waiting for ready"
        az aks wait --name $AZURE_E2E_CLUSTER_NAME --resource-group $E2E_RESOURCE_GROUP_NAME --created --interval 60 --timeout 1800
        provisioning_state=$(az aks show -n $AZURE_E2E_CLUSTER_NAME -g $E2E_RESOURCE_GROUP_NAME -ojson | jq '.provisioningState' | tr -d "\"")
        if [ "$provisioning_state" == "Succeeded" ]; then
            log "Cluster created by other pipeline successfully"
        else
            err "Other pipeline failed to create the cluster. Current state of cluster is $provisioning_state."
            exit 1
        fi
    elif [ "$provisioning_state" == "Updating" ]; then
        # Other pipeline is updating this cluster
        log "Cluster $AZURE_E2E_CLUSTER_NAME is being updated, waiting for ready"
        az aks wait --name $AZURE_E2E_CLUSTER_NAME --resource-group $E2E_RESOURCE_GROUP_NAME --updated --interval 60 --timeout 1800
        provisioning_state=$(az aks show -n $AZURE_E2E_CLUSTER_NAME -g $E2E_RESOURCE_GROUP_NAME -ojson | jq '.provisioningState' | tr -d "\"")
        if [ "$provisioning_state" == "Succeeded" ]; then
            log "Cluster updated by other pipeline successfully"
        else
            err "Other pipeline failed to update the cluster. Current state of cluster is $provisioning_state."
            exit 1
        fi
    elif [ "$provisioning_state" == "Deleting" ]; then
        log "Cluster $AZURE_E2E_CLUSTER_NAME is being deleted, waiting for ready"
        az aks wait --name $AZURE_E2E_CLUSTER_NAME --resource-group $E2E_RESOURCE_GROUP_NAME --deleted --interval 60 --timeout 1800
        retval=0
        az aks show -n $AZURE_E2E_CLUSTER_NAME -g $E2E_RESOURCE_GROUP_NAME -ojson || retval=$?
        if [ "$retval" -ne 0  ]; then
            log "Cluster deleted successfully"
            create_cluster="true"
        else
            err "Failed to delete the cluster."
            exit 1
        fi
    fi
else
    create_cluster="true"
fi

# Create the AKS cluster and get the kubeconfig
if [ "$create_cluster" == "true" ]; then
    log "Creating cluster $AZURE_E2E_CLUSTER_NAME"
    clusterCreateStartTime=$(date +%s)
    retval=0
    
    az aks create -g $E2E_RESOURCE_GROUP_NAME -n $AZURE_E2E_CLUSTER_NAME --node-vm-size Standard_D2s_v3  --enable-managed-identity --assign-identity $AZURE_MSI_RESOURCE_STRING --assign-kubelet-identity $AZURE_MSI_RESOURCE_STRING --node-count 1 --generate-ssh-keys --network-plugin azure -ojson || retval=$?

    if [ "$retval" -ne 0  ]; then
        log "Other pipelines may be creating cluster $AZURE_E2E_CLUSTER_NAME, waiting for ready"
        create_cluster="false"
        az aks wait --name $AZURE_E2E_CLUSTER_NAME --resource-group $E2E_RESOURCE_GROUP_NAME --created --interval 60 --timeout 1800
    fi
    provisioning_state=$(az aks show -n $AZURE_E2E_CLUSTER_NAME -g $E2E_RESOURCE_GROUP_NAME -ojson | jq '.provisioningState' | tr -d "\"")
    if [ "$provisioning_state" == "Succeeded" ]; then
        log "Created cluster successfully"
    else
        err "Failed to create cluster. Current state is $provisioning_state."
        exit 1
    fi
    clusterCreateEndTime=$(date +%s)
    log "Created cluster $AZURE_E2E_CLUSTER_NAME in $((clusterCreateEndTime-clusterCreateStartTime)) seconds"
fi

az aks get-credentials -g $E2E_RESOURCE_GROUP_NAME -n $AZURE_E2E_CLUSTER_NAME --file kubeconfig --overwrite-existing
KUBECONFIG=$(pwd)/kubeconfig
export KUBECONFIG

# Store the contents of az aks show to a file to reduce API call overhead
az aks show -n $AZURE_E2E_CLUSTER_NAME -g $E2E_RESOURCE_GROUP_NAME -ojson > cluster_info.json

E2E_MC_RESOURCE_GROUP_NAME="MC_${E2E_RESOURCE_GROUP_NAME}_${AZURE_E2E_CLUSTER_NAME}_eastus"
az vmss list -g $E2E_MC_RESOURCE_GROUP_NAME --query "[?contains(name, 'nodepool')]" -otable
MC_VMSS_NAME=$(az vmss list -g $E2E_MC_RESOURCE_GROUP_NAME --query "[?contains(name, 'nodepool')]" -ojson | jq -r '.[0].name')
CLUSTER_ID=$(echo $MC_VMSS_NAME | cut -d '-' -f3)

backfill_clean_storage_container
if [ "$create_cluster" == "true" ]; then
    create_storage_container
    upload_linux_file_to_storage_account
    if [ "$WINDOWS_E2E_IMAGE" == "2019-containerd" ]; then
        cleanupOutdatedFiles
    fi
else
    if [[ "$(check_linux_file_exists_in_storage_account)" == *"Linux file does not exist in storage account."* ]]; then 
        upload_linux_file_to_storage_account
    fi
fi
download_linux_file_from_storage_account
log "Download of linux file from storage account completed"

set +x
addJsonToFile "apiserverCrt" "$(cat apiserver.crt)"
addJsonToFile "caCrt" "$(cat ca.crt)"
addJsonToFile "clientKey" "$(cat client.key)"
if [ -f "bootstrap-kubeconfig" ] && [ -n "$(cat bootstrap-kubeconfig)" ]; then
    tlsToken="$(grep "token" < bootstrap-kubeconfig | cut -f2 -d ":" | tr -d '"')"
    addJsonToFile "tlsbootstraptoken" "$tlsToken"
fi
set -x

# # Add other relevant information needed by AgentBaker for bootstrapping later
getAgentPoolProfileValues
getFQDN
getMSIResourceID

addJsonToFile "mcRGName" $E2E_MC_RESOURCE_GROUP_NAME
addJsonToFile "clusterID" $CLUSTER_ID
addJsonToFile "subID" $AZURE_E2E_SUBSCRIPTION_ID

set +x
# shellcheck disable=SC2091
$(jq -r 'keys[] as $k | "export \($k)=\(.[$k])"' fields.json)
envsubst < percluster_template.json > percluster_config.json
jq -s '.[0] * .[1]' nodebootstrapping_template.json percluster_config.json > nodebootstrapping_config.json
log "Node bootstrapping config generated"
