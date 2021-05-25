echo "Starting e2e tests"

SUBSCRIPTION_ID="8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8" #Azure Container Service - Test Subscription
RESOURCE_GROUP_NAME="agentbaker-e2e-tests"
LOCATION="eastus"
CLUSTER_NAME="agentbaker-e2e-test-cluster"

if [ $(az group exists -n $RESOURCE_GROUP_NAME --subscription $SUBSCRIPTION_ID) == "false" ]; then
    echo "Creating resource group"
    az group create -l $LOCATION -n $RESOURCE_GROUP_NAME --subscription $SUBSCRIPTION_ID
fi

if [ -z $(az aks list -g $RESOURCE_GROUP_NAME | jq '.[].name') ]; then
    echo "Cluster doesnt exist, creating"
    az aks create -g $RESOURCE_GROUP_NAME -n $CLUSTER_NAME --node-count 1 --generate-ssh-keys
fi
