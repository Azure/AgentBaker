#!/bin/bash

echo "Starting e2e tests"

# TODO 1: Probably move the helper functions to some other file to imrpove readability

addJsonToFile() {
    k=$1; v=$2
    jq -r --arg key $k --arg value $v '. + { ($key) : $value}' < fields.json > dummy.json && mv dummy.json fields.json
}

getAgentPoolProfileValues() {
    declare -a properties=("mode" "name" "nodeImageVersion")

    for property in "${properties[@]}"; do
        value=$(az aks show -n $CLUSTER_NAME -g $RESOURCE_GROUP_NAME | jq -r .agentPoolProfiles[].${property})
        addJsonToFile $property $value
    done
}

getFQDN() {
    fqdn=$(az aks show -n $CLUSTER_NAME -g $RESOURCE_GROUP_NAME | jq -r '.fqdn')
    addJsonToFile "fqdn" $fqdn
}

getMSIResourceID() {
    msiResourceID=$(az aks show -n $CLUSTER_NAME -g $RESOURCE_GROUP_NAME | jq -r '.identityProfile.kubeletidentity.resourceId')
    addJsonToFile "msiResourceID" $msiResourceID
}

getTenantID() {
    tenantID=$(az aks show -n $CLUSTER_NAME -g $RESOURCE_GROUP_NAME | jq -r '.identity.tenantId')
    addJsonToFile "tenantID" $tenantID
}

SUBSCRIPTION_ID="8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8" #Azure Container Service - Test Subscription
RESOURCE_GROUP_NAME="agentbaker-e2e-tests"
LOCATION="eastus"
CLUSTER_NAME="agentbaker-e2e-test-cluster"

# Clear the kube/config file for any conflicts
truncate -s 0 ~/.kube/config

# Create a resource group for the cluster
if [ $(az group exists -n $RESOURCE_GROUP_NAME --subscription $SUBSCRIPTION_ID) == "false" ]; then
    echo "Creating resource group"
    az group create -l $LOCATION -n $RESOURCE_GROUP_NAME --subscription $SUBSCRIPTION_ID
fi

# Create the aks cluster and get the credentials(kube/config populated) to kubectl 
if [ -z $(az aks list -g $RESOURCE_GROUP_NAME | jq '.[].name') ]; then
    echo "Cluster doesnt exist, creating"
    az aks create -g $RESOURCE_GROUP_NAME -n $CLUSTER_NAME --node-count 1 --generate-ssh-keys
    az aks get-credentials -g $RESOURCE_GROUP_NAME -n $CLUSTER_NAME
fi

MC_RESOURCE_GROUP_NAME=$(az aks show -n $CLUSTER_NAME -g $RESOURCE_GROUP_NAME | jq -r '.nodeResourceGroup')
VMSS_NAME=$(az vmss list -g $MC_RESOURCE_GROUP_NAME | jq -r '.[length -1].name')
CLUSTER_ID=$(echo $VMSS_NAME | cut -d '-' -f3)

# Retrieve the etc/kubernetes/azure.json file for cluster related info
az vmss run-command invoke \
            -n $VMSS_NAME \
            -g $MC_RESOURCE_GROUP_NAME \
            --command-id RunShellScript \
            --instance-id 0 \
            --scripts "cat /etc/kubernetes/azure.json" | jq -r '.value[].message' | awk '/{/{flag=1}/}/{print;flag=0}flag' \
            > fields.json

# Retrieve the keys and certificates

# TODO 2: If TLS Bootstrapping is not enabled, the client.crt takes some time to be generated. The run-command throws
#       an error saying that extension is still being applied. Need to introduce some delay before this piece of code is
#       called and the file is ready to be read else the whole flow will break. 

declare -a files=("apiserver.crt" "ca.crt" "client.key" "client.crt")
for file in "${files[@]}"; do
    content=$(az vmss run-command invoke \
                -n $VMSS_NAME \
                -g $MC_RESOURCE_GROUP_NAME \
                --command-id RunShellScript \
                --instance-id 0 \
                --scripts "cat /etc/kubernetes/certs/$file" | \
                jq -r '.value[].message' | \
                awk '/stdout/{flag=1;next}/stderr/{flag=0}flag' | \
                awk NF | base64 \
            )
    jq -r --arg key $file --arg value $content '. + { ($key) : $value }' < fields.json > dummy.json && mv dummy.json fields.json
done

# Add other relevant information needed by AgentBaker for bootstrapping later
getAgentPoolProfileValues
getFQDN
getMSIResourceID

addJsonToFile "mcRGName" $MC_RESOURCE_GROUP_NAME
addJsonToFile "clusterID" $CLUSTER_ID
addJsonToFile "subID" $SUBSCRIPTION_ID

# Check if TLS Bootstrapping is enabled(no client.crt in that case, retrieve the tlsbootstrap token)
tlsbootstrap=$(az vmss run-command invoke \
                -n $VMSS_NAME \
                -g $MC_RESOURCE_GROUP_NAME \
                --command-id RunShellScript \
                --instance-id 0 \
                --scripts "cat /var/lib/kubelet/bootstrap-kubeconfig" | \
                jq -r '.value[].message' | \
                grep "token" | \
                cut -f2 -d ":" | tr -d '"'
            )

if [[ -z ${tlsbootstrap} ]]; then
    echo "TLS Bootstrap disabled"
else
    addJsonToFile "tlsbootstraptoken" $tlsbootstrap
fi

# Call AgentBaker to generate CustomData and cseCmd
go test -v

# Create a test VMSS with 1 instance 
# TODO 3: Discuss about the --image version, probably go with aks-ubuntu-1804-gen2-2021-q2:latest
#       However, how to incorporate chaning quarters?

# TODO 4: Random name for the VMSS for when we have multiple scenarios to run

az vmss create -n agentbaker-test-vmss \
    -g $MC_RESOURCE_GROUP_NAME \
    --admin-username azureuser \
    --custom-data cloud-init.txt \
    --lb kubernetes --backend-pool-name aksOutboundBackendPool \
    --vm-sku Standard_DS2_v2 \
    --instance-count 1 \
    --assign-identity $msiResourceID \
    --image "microsoft-aks:aks:aks-ubuntu-1804-gen2-2021-q2:2021.05.19" \
    --upgrade-policy-mode Automatic

# Get the name of the VM instance to later check with kubectl get nodes
computerName=$(az vmss list-instances \
                -n agentbaker-test-vmss \
                -g $MC_RESOURCE_GROUP_NAME | \
                jq -r '.[].osProfile.computerName'
            )

# Generate the extension from cseCmd
jq -Rs '{commandToExecute: . }' cseCmd > settings.json

# Apply extension to the VM
az vmss extension set --resource-group $MC_RESOURCE_GROUP_NAME \
    --name CustomScript \
    --vmss-name agentbaker-test-vmss \
    --publisher Microsoft.Azure.Extensions \
    --protected-settings settings.json \
    --version 2.0

# Sleep to let the automatic upgrade of the VM finish
sleep 60s

# Check if the node joined the cluster
if [[ -z $(kubectl get nodes | grep $computerName) ]]; then
	echo "Node did not join the cluster"
else
	echo "Test succeeded, node joined the cluster"
fi
