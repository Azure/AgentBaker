#!/bin/bash

set -euxo pipefail

source e2e-helper.sh

log "Starting e2e tests"

# Create a resource group for the cluster
log "Creating resource group"
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
    fi
else
    create_cluster="true"
fi

# Create the AKS cluster and get the kubeconfig
if [ "$create_cluster" == "true" ]; then
    log "Creating cluster $CLUSTER_NAME"
    clusterCreateStartTime=$(date +%s)
    az aks create -g $RESOURCE_GROUP_NAME -n $CLUSTER_NAME --node-count 1 --generate-ssh-keys --network-plugin kubenet -ojson
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

# privileged ds with nsenter for host file exfiltration
kubectl apply -f deploy.yaml
kubectl rollout status deploy/debug

# Retrieve the etc/kubernetes/azure.json file for cluster related info
log "Retrieving cluster info"
clusterInfoStartTime=$(date +%s)

exec_on_host "cat /etc/kubernetes/azure.json" fields.json
exec_on_host "cat /etc/kubernetes/certs/apiserver.crt | base64 -w 0" apiserver.crt
exec_on_host "cat /etc/kubernetes/certs/ca.crt | base64 -w 0" ca.crt
exec_on_host "cat /etc/kubernetes/certs/client.key | base64 -w 0" client.key
exec_on_host "cat /var/lib/kubelet/bootstrap-kubeconfig" bootstrap-kubeconfig

clusterInfoEndTime=$(date +%s)
log "Retrieved cluster info in $((clusterInfoEndTime-clusterInfoStartTime)) seconds"

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

set +x
# shellcheck disable=SC2091
$(jq -r 'keys[] as $k | "export \($k)=\(.[$k])"' fields.json)
envsubst < percluster_template.json > percluster_config.json
jq -s '.[0] * .[1]' nodebootstrapping_template.json percluster_config.json > nodebootstrapping_config.json
