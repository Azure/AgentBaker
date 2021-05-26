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

echo $MC_RESOURCE_GROUP_NAME
echo $VMSS_NAME

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