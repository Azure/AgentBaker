#!/bin/bash

set -euxo pipefail

source e2e-helper.sh

log "Starting e2e tests"

# Create a resource group for the cluster
log "Creating resource group"
if [[ "$RESOURCE_GROUP_NAME" == *"windows"*  ]]; then
    K8S_VERSION=$(echo $KUBERNETES_VERSION | tr '.' '-')
    RESOURCE_GROUP_NAME="$RESOURCE_GROUP_NAME"-"$WINDOWS_E2E_IMAGE"-"$K8S_VERSION"
fi

rgStartTime=$(date +%s)
az group create -l $LOCATION -n $RESOURCE_GROUP_NAME --subscription $SUBSCRIPTION_ID -ojson
rgEndTime=$(date +%s)
log "Created resource group in $((rgEndTime-rgStartTime)) seconds"

# Check if there exists a cluster in the RG. If yes, check if the MC_RG associated with it still exists.
# MC_RG gets deleted due to ACS-Test Garbage Collection but the cluster hangs around
out=$(az aks list -g $RESOURCE_GROUP_NAME -ojson | jq '.[].name')
create_cluster="false"
if [ -n "$out" ]; then
    provisioning_state=$(az aks show -n $CLUSTER_NAME -g $RESOURCE_GROUP_NAME -ojson | jq '.provisioningState' | tr -d "\"")
    MC_RG_NAME="MC_${RESOURCE_GROUP_NAME}_${CLUSTER_NAME}_$LOCATION"
    exists=$(az group exists -n $MC_RG_NAME)
    if [ "$exists" == "false" ] || [ "$provisioning_state" == "Failed" ]; then
        # The cluster is in a broken state
        log "Cluster $CLUSTER_NAME is in an unusable state, deleting..."
        clusterDeleteStartTime=$(date +%s)
        az aks delete -n $CLUSTER_NAME -g $RESOURCE_GROUP_NAME --yes
        clusterDeleteEndTime=$(date +%s)
        log "Deleted cluster $CLUSTER_NAME in $((clusterDeleteEndTime-clusterDeleteStartTime)) seconds"
        create_cluster="true"
    elif [ "$provisioning_state" == "Creating" ]; then
        # Other pipeline is creating this cluster
        log "Cluster $CLUSTER_NAME is being created, waiting for ready"
        az aks wait --name $CLUSTER_NAME --resource-group $RESOURCE_GROUP_NAME --created --interval 60 --timeout 1800
        provisioning_state=$(az aks show -n $CLUSTER_NAME -g $RESOURCE_GROUP_NAME -ojson | jq '.provisioningState' | tr -d "\"")
        if [ "$provisioning_state" == "Succeeded" ]; then
            log "Cluster created by other pipeline successfully"
        else
            err "Other pipeline failed to create the cluster. Current state of cluster is $provisioning_state."
            exit 1
        fi
    fi
else
    create_cluster="true"
fi

# Create the AKS cluster and get the kubeconfig
if [ "$create_cluster" == "true" ]; then
    log "Creating cluster $CLUSTER_NAME"
    clusterCreateStartTime=$(date +%s)
    retval=0
    
    az aks create -g $RESOURCE_GROUP_NAME -n $CLUSTER_NAME --node-count 1 --generate-ssh-keys --network-plugin azure --kubernetes-version $KUBERNETES_VERSION -ojson || retval=$?

    if [ "$retval" -ne 0  ]; then
        log "Other pipelines may be creating cluster $CLUSTER_NAME, waiting for ready"
        create_cluster="false"
        az aks wait --name $CLUSTER_NAME --resource-group $RESOURCE_GROUP_NAME --created --interval 60 --timeout 1800
    fi
    provisioning_state=$(az aks show -n $CLUSTER_NAME -g $RESOURCE_GROUP_NAME -ojson | jq '.provisioningState' | tr -d "\"")
    if [ "$provisioning_state" == "Succeeded" ]; then
        log "Created cluster successfully"
    else
        err "Failed to create cluster. Current state is $provisioning_state."
        exit 1
    fi
    clusterCreateEndTime=$(date +%s)
    log "Created cluster $CLUSTER_NAME in $((clusterCreateEndTime-clusterCreateStartTime)) seconds"
fi

az aks get-credentials -g $RESOURCE_GROUP_NAME -n $CLUSTER_NAME --file kubeconfig --overwrite-existing
KUBECONFIG=$(pwd)/kubeconfig
export KUBECONFIG

# Store the contents of az aks show to a file to reduce API call overhead
az aks show -n $CLUSTER_NAME -g $RESOURCE_GROUP_NAME -ojson > cluster_info.json

MC_RESOURCE_GROUP_NAME="MC_${RESOURCE_GROUP_NAME}_${CLUSTER_NAME}_eastus"
az vmss list -g $MC_RESOURCE_GROUP_NAME --query "[?contains(name, 'nodepool')]" -otable
MC_VMSS_NAME=$(az vmss list -g $MC_RESOURCE_GROUP_NAME --query "[?contains(name, 'nodepool')]" -ojson | jq -r '.[0].name')
CLUSTER_ID=$(echo $MC_VMSS_NAME | cut -d '-' -f3)

if [ "$create_cluster" == "true" ]; then
    create_storage_account
    upload_linux_file_to_storage_account
    if [ "$WINDOWS_E2E_IMAGE" == "2019-containerd" ]; then
        cleanupOutdatedFiles
    fi
fi
download_linux_file_from_storage_account

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

addJsonToFile "mcRGName" $MC_RESOURCE_GROUP_NAME
addJsonToFile "clusterID" $CLUSTER_ID
addJsonToFile "subID" $SUBSCRIPTION_ID
addJsonToFile "orchestratorVersion" $KUBERNETES_VERSION

set +x
# shellcheck disable=SC2091
$(jq -r 'keys[] as $k | "export \($k)=\(.[$k])"' fields.json)
envsubst < percluster_template.json > percluster_config.json
cat percluster_config.json

jq -s '.[0] * .[1]' nodebootstrapping_template.json percluster_config.json > nodebootstrapping_config.json
log "Node bootstrapping configuration done"
cat nodebootstrapping_config.json