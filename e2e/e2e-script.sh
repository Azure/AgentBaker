#!/bin/bash

set -euxo pipefail

source e2e-helper.sh

log() {
    printf "\\033[1;33m%s\\033[0m\\n" "$*"
}

ok() {
    printf "\\033[1;32m%s\\033[0m\\n" "$*"
}

err() {
    printf "\\033[1;31m%s\\033[0m\\n" "$*"
}

log "Starting e2e tests"

: "${SUBSCRIPTION_ID:=8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8}" #Azure Container Service - Test Subscription
: "${RESOURCE_GROUP_NAME:=agentbaker-e2e-tests}"
: "${LOCATION:=eastus}"
: "${CLUSTER_NAME:=agentbaker-e2e-test-cluster}"

globalStartTime=$(date +%s)

# Create a resource group for the cluster
log "Creating resource group"
rgStartTime=$(date +%s)
az group create -l $LOCATION -n $RESOURCE_GROUP_NAME --subscription $SUBSCRIPTION_ID -ojson
rgEndTime=$(date +%s)
log "Created resource group in $((rgEndTime-rgStartTime)) seconds"

# Create the AKS cluster and get the kubeconfig
out=$(az aks list -g $RESOURCE_GROUP_NAME -ojson | jq '.[].name')
if [ -z "$out" ]; then
    log "Creating cluster"
    clusterStartTime=$(date +%s)
    az aks create -g $RESOURCE_GROUP_NAME -n $CLUSTER_NAME --node-count 1 --generate-ssh-keys -ojson
    clusterEndTime=$(date +%s)
    log "Created cluster in $((clusterEndTime-clusterStartTime)) seconds"
fi

az aks get-credentials -g $RESOURCE_GROUP_NAME -n $CLUSTER_NAME --file kubeconfig --overwrite-existing

# Store the contents of az aks show to a file to reduce API call overhead
az aks show -n $CLUSTER_NAME -g $RESOURCE_GROUP_NAME -ojson > cluster_info.json

MC_RESOURCE_GROUP_NAME=$(jq -r '.nodeResourceGroup' < cluster_info.json)
VMSS_NAME=$(az vmss list -g $MC_RESOURCE_GROUP_NAME -ojson | jq -r '.[length -1].name')
CLUSTER_ID=$(echo $VMSS_NAME | cut -d '-' -f3)

# Retrieve the etc/kubernetes/azure.json file for cluster related info
log "Retrieving cluster info"
clusterInfoStartTime=$(date +%s)
az vmss run-command invoke \
            -n $VMSS_NAME \
            -g $MC_RESOURCE_GROUP_NAME \
            --command-id RunShellScript \
            --instance-id 0 \
            --scripts "cat /etc/kubernetes/azure.json" \
            -ojson | \
            jq -r '.value[].message' | awk '/{/{flag=1}/}/{print;flag=0}flag' \
            > fields.json
clusterInfoEndTime=$(date +%s)
log "Retrieved cluster info in $((clusterInfoEndTime-clusterInfoStartTime)) seconds"


# Retrieve the keys and certificates

# TODO 2: If TLS Bootstrapping is not enabled, the client.crt takes some time to be generated. The run-command throws
#       an error saying that extension is still being applied. Need to introduce some delay before this piece of code is
#       called and the file is ready to be read else the whole flow will break. 

log "Retrieving TLS data"
tlsStartTime=$(date +%s)
declare -a files=("apiserver.crt" "ca.crt" "client.key")
for file in "${files[@]}"; do
    for i in $(seq 1 10); do
        set +e
        content=$(az vmss run-command invoke \
                -n $VMSS_NAME \
                -g $MC_RESOURCE_GROUP_NAME \
                --command-id RunShellScript \
                --instance-id 0 \
                --scripts "cat /etc/kubernetes/certs/$file | base64 -w 0" \
                -ojson | \
                jq -r '.value[].message' | \
                awk '/stdout/{flag=1;next}/stderr/{flag=0}flag' | \
                awk NF \
        )
        retval=$?
        set -e
        if [ "$retval" -ne 0 ]; then
            log "retrying attempt $i"
            sleep 10s
            continue
        fi
        break;
    done
    [ "$retval" -eq 0 ]
    addJsonToFile "$file" "$content"
done
tlsEndTime=$(date +%s)
log "Retrieved TLS data in $((tlsEndTime-tlsStartTime)) seconds"

# Add other relevant information needed by AgentBaker for bootstrapping later
getAgentPoolProfileValues
getFQDN
getMSIResourceID

addJsonToFile "mcRGName" $MC_RESOURCE_GROUP_NAME
addJsonToFile "clusterID" $CLUSTER_ID
addJsonToFile "subID" $SUBSCRIPTION_ID

# TODO(ace): generate fresh bootstrap token since one on node will expire.
# Check if TLS Bootstrapping is enabled(no client.crt in that case, retrieve the tlsbootstrap token)
log "Reading TLS bootstrap data"
tlsBootstrapStartTime=$(date +%s)
for i in $(seq 1 10); do
    set +e
    tlsbootstrap=$(az vmss run-command invoke \
                -n $VMSS_NAME \
                -g $MC_RESOURCE_GROUP_NAME \
                --command-id RunShellScript \
                --instance-id 0 \
                --scripts "cat /var/lib/kubelet/bootstrap-kubeconfig" \
                -ojson | \
                jq -r '.value[].message' | \
                grep "token" | \
                cut -f2 -d ":" | tr -d '"'
    )
    retval=$?
    set -e
    if [ "$retval" -ne 0 ]; then
        log "retrying attempt $i"
        sleep 10s
        continue
    fi
    break;
done
tlsBootstrapEndTime=$(date +%s)
[ "$retval" -eq 0 ]
log "Read TLS bootstrap data in $((tlsBootstrapEndTime-tlsBootstrapStartTime)) seconds"

if [[ -z "${tlsbootstrap}" ]]; then
    log "TLS Bootstrap disabled"
else
    addJsonToFile "tlsbootstraptoken" $tlsbootstrap
fi

# Call AgentBaker to generate CustomData and cseCmd
go test -run TestE2EBasic

# Create a test VMSS with 1 instance 
# TODO 3: Discuss about the --image version, probably go with aks-ubuntu-1804-gen2-2021-q2:latest
#       However, how to incorporate chaning quarters?
log "Creating VMSS"
VMSS_NAME="$(mktemp --dry-run abtest-XXXXXXX | tr '[:upper:]' '[:lower:]')"
vmssStartTime=$(date +%s)
az vmss create -n ${VMSS_NAME} \
    -g $MC_RESOURCE_GROUP_NAME \
    --admin-username azureuser \
    --custom-data cloud-init.txt \
    --lb kubernetes --backend-pool-name aksOutboundBackendPool \
    --vm-sku Standard_DS2_v2 \
    --instance-count 1 \
    --assign-identity $msiResourceID \
    --image "microsoft-aks:aks:aks-ubuntu-1804-gen2-2021-q2:2021.05.19" \
    --upgrade-policy-mode Automatic \
    -ojson
vmssEndTime=$(date +%s)
log "Created VMSS in $((vmssEndTime-vmssStartTime)) seconds"

# Get the name of the VM instance to later check with kubectl get nodes
vmInstanceName=$(az vmss list-instances \
                -n ${VMSS_NAME} \
                -g $MC_RESOURCE_GROUP_NAME \
                -ojson | \
                jq -r '.[].osProfile.computerName'
            )
export vmInstanceName

# Generate the extension from csecmd
jq -Rs '{commandToExecute: . }' csecmd > settings.json

# Apply extension to the VM
log "Applying extensions to VMSS"
vmssExtStartTime=$(date +%s)
az vmss extension set --resource-group $MC_RESOURCE_GROUP_NAME \
    --name CustomScript \
    --vmss-name ${VMSS_NAME} \
    --publisher Microsoft.Azure.Extensions \
    --protected-settings settings.json \
    --version 2.0 \
    -ojson
vmssExtEndTime=$(date +%s)
log "Applied extensions in $((vmssExtEndTime-vmssExtStartTime)) seconds"

KUBECONFIG=$(pwd)/kubeconfig; export KUBECONFIG

# Sleep to let the automatic upgrade of the VM finish
waitForNodeStartTime=$(date +%s)
for i in $(seq 1 10); do
    set +e
    kubectl get nodes | grep -q $vmInstanceName
    retval=$?
    set -e
    if [ "$retval" -ne 0 ]; then
        log "retrying attempt $i"
        sleep 10s
        continue
    fi
    break;
done
waitForNodeEndTime=$(date +%s)
log "Waited $((waitForNodeEndTime-waitForNodeStartTime)) seconds for node to join"

# Check if the node joined the cluster
if [[ "$retval" -eq 0 ]]; then
    ok "Test succeeded, node joined the cluster"
else
    err "Node did not join cluster"
    exit 1
fi

# Run a nginx pod on the node to check if pod runs
envsubst < pod-nginx-template.yaml > pod-nginx.yaml
kubectl apply -f pod-nginx.yaml

# Sleep to let Pod Status=Running
waitForPodStartTime=$(date +%s)
for i in $(seq 1 10); do
    set +e
    kubectl get pods -o wide | grep -q 'Running'
    retval=$?
    set -e
    if [ "$retval" -ne 0 ]; then
        log "retrying attempt $i"
        sleep 10s
        continue
    fi
    break;
done
waitForPodEndTime=$(date +%s)
log "Waited $((waitForPodEndTime-waitForPodStartTime)) seconds for pod to come up"

if [[ "$retval" -eq 0 ]]; then
    ok "Pod ran successfully"
else
    err "Pod pending/not running"
    exit 1
fi

globalEndTime=$(date +%s)
log "Finished after $((globalEndTime-globalStartTime)) seconds"
