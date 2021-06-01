echo "Starting e2e tests"

SUBSCRIPTION_ID="8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8" #Azure Container Service - Test Subscription
RESOURCE_GROUP_NAME="agentbaker-e2e-tests"
LOCATION="eastus"
CLUSTER_NAME="agentbaker-e2e-test-cluster"
declare -a files=("apiserver.crt" "ca.crt" "client.key" "client.crt")

if [ $(az group exists -n $RESOURCE_GROUP_NAME --subscription $SUBSCRIPTION_ID) == "false" ]; then
    echo "Creating resource group"
    az group create -l $LOCATION -n $RESOURCE_GROUP_NAME --subscription $SUBSCRIPTION_ID
fi

if [ -z $(az aks list -g $RESOURCE_GROUP_NAME | jq '.[].name') ]; then
    echo "Cluster doesnt exist, creating"
    az aks create -g $RESOURCE_GROUP_NAME -n $CLUSTER_NAME --node-count 1 --generate-ssh-keys
fi

MC_RESOURCE_GROUP_NAME=$(az aks show -n $CLUSTER_NAME -g $RESOURCE_GROUP_NAME | jq -r '.nodeResourceGroup')
VMSS_NAME=$(az vmss list -g $MC_RESOURCE_GROUP_NAME | jq -r '.[].name')
CLUSTER_ID=$(echo $VMSS_NAME | cut -d '-' -f3)

echo $MC_RESOURCE_GROUP_NAME
echo $VMSS_NAME
echo $CLUSTER_ID

az vmss run-command invoke \
            -n $VMSS_NAME \
            -g $MC_RESOURCE_GROUP_NAME \
            --command-id RunShellScript \
            --instance-id 0 \
            --scripts "cat /etc/kubernetes/azure.json" | jq -r '.value[].message' | awk '/{/{flag=1}/}/{print;flag=0}flag' \
            > fields.json

for file in "${files[@]}"; do
    content=$(az vmss run-command invoke \
                -n $VMSS_NAME \
                -g $MC_RESOURCE_GROUP_NAME \
                --command-id RunShellScript \
                --instance-id 0 \
                --scripts "cat /etc/kubernetes/certs/$file | base64 -w0" | \
                jq -r '.value[].message' | \
                awk '/stdout/{flag=1;next}/stderr/{flag=0}flag' \
            )
    jq -r --arg key $file --arg value $content '. + { ($key) : $value }' < fields.json > dummy.json && mv dummy.json fields.json
done

#make this a function
#we dont want to populate every value like this

#TODO 1) : Make the following a function : Adding a field to the JSON file(moving from one to another)
#TODO 2) : 
#Have all the fields that we want in an array and get them through there, 
#eg get all agentPoolProfiles[] field in an array and iterate over them to fetch instead of seperate calls

fqdn=$(az aks show -n $CLUSTER_NAME -g $RESOURCE_GROUP_NAME | jq -r '.fqdn')
jq -r --arg value $fqdn '. + { "fqdn" : $value }' < fields.json > dummy.json && mv dummy.json fields.json

mode=$(az aks show -n $CLUSTER_NAME -g $RESOURCE_GROUP_NAME | jq -r '.agentPoolProfiles[].mode')
jq -r --arg value $mode '. + { "mode" : $value }' < fields.json > dummy.json && mv dummy.json fields.json

nodepool_name=$(az aks show -n $CLUSTER_NAME -g $RESOURCE_GROUP_NAME | jq -r '.agentPoolProfiles[].name')
jq -r --arg value $nodepool_name '. + { "nodepoolname" : $value }' < fields.json > dummy.json && mv dummy.json fields.json

image_version=$(az aks show -n $CLUSTER_NAME -g $RESOURCE_GROUP_NAME | jq -r '.agentPoolProfiles[].nodeImageVersion')
jq -r --arg value $image_version '. + { "nodeImageVersion" : $value }' < fields.json > dummy.json && mv dummy.json fields.json

tenantID=$(az aks show -n $CLUSTER_NAME -g $RESOURCE_GROUP_NAME | jq -r '.identity.tenantId')
jq -r --arg value $tenantID '. + { "tenantID" : $value }' < fields.json > dummy.json && mv dummy.json fields.json

jq -r --arg value $MC_RESOURCE_GROUP_NAME '. + { "mcRGName" : $value }' < fields.json > dummy.json && mv dummy.json fields.json

jq -r --arg value $CLUSTER_ID '. + { "clusterID" : $value }' < fields.json > dummy.json && mv dummy.json fields.json

jq -r --arg value $SUBSCRIPTION_ID '. + { "subID" : $value }' < fields.json > dummy.json && mv dummy.json fields.json

go test -v