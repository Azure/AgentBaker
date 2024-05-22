#!/bin/bash
set -eux

TRIVY_SCRIPT_PATH="trivy-scan.sh"
TEST_RESOURCE_PREFIX="vhd-scanning"
VM_NAME="$TEST_RESOURCE_PREFIX-vm"
VHD_IMAGE="$MANAGED_SIG_ID"
SIG_CONTAINER_NAME="vhd-scans"
TEST_VM_ADMIN_USERNAME="azureuser"
STORAGE_ACCOUNT_NAME="vhdbuildereastustest"

set +x
TEST_VM_ADMIN_PASSWORD="TestVM@$(date +%s)"
set -x


RESOURCE_GROUP_NAME="$TEST_RESOURCE_PREFIX-$(date +%s)-$RANDOM"
az group create --name $RESOURCE_GROUP_NAME --location ${AZURE_LOCATION} --tags 'source=AgentBaker'

if [ "$OS_VERSION" == "18.04" ]; then
    echo "Skipping scanning for 18.04"
    exit 0
fi

function cleanup() {
    echo "Deleting resource group ${RESOURCE_GROUP_NAME}"
    az group delete --name $RESOURCE_GROUP_NAME --yes --no-wait
}
trap cleanup EXIT

VM_OPTIONS="--size Standard_DS1_v2"
if [[ "${ARCHITECTURE,,}" == "arm64" ]]; then
    VM_OPTIONS="--size Standard_D2pds_v5"
elif [[ "${FEATURE_FLAGS,,}" == "kata" ]]; then
    VM_OPTIONS="--size Standard_D4ds_v5"
fi

if [[ "${OS_TYPE}" == "Linux" && "${ENABLE_TRUSTED_LAUNCH}" == "True" ]]; then
    VM_OPTIONS+="--security-type TrustedLaunch --enable-secure-boot true --enable-vtpm true"
fi

#fix identity string
az vm create --resource-group $RESOURCE_GROUP_NAME \
    --name $VM_NAME \
    --image $VHD_IMAGE \
    --admin-username $TEST_VM_ADMIN_USERNAME \
    --admin-password $TEST_VM_ADMIN_PASSWORD \
    --os-disk-size-gb 50 \
    --assign-identity "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/vhd-scanning-UAMI" \
    ${VM_OPTIONS}

FULL_PATH=$(realpath $0)
CDIR=$(dirname $FULL_PATH)
SCRIPT_PATH="$CDIR/$TRIVY_SCRIPT_PATH"
az vm run-command invoke \
    --command-id RunShellScript \
    --name $VM_NAME \
    --resource-group $RESOURCE_GROUP_NAME \
    --scripts @$SCRIPT_PATH

if [ "$OS_SKU" = "Ubuntu" ] && [ "$SIG_IMAGE_NAME" = *2004* ]; then
    az vm run-command invoke \
        --command-id RunShellScript \
        --name $VM_NAME \
        --resource-group $RESOURCE_GROUP_NAME \
        --scripts "sudo apt-get install -y azure-cli"

elif [ "$OS_SKU" = "Ubuntu" ] && [ "$SIG_IMAGE_NAME" = *2204* ]; then
    az vm run-command invoke \
        --command-id RunShellScript \
        --name $VM_NAME \
        --resource-group $RESOURCE_GROUP_NAME \
        --scripts "sudo apt-get install -y ca-certificates curl apt-transport-https lsb-release gnupg &&
            curl -sL https://packages.microsoft.com/keys/microsoft.asc | sudo apt-key add - &&
            echo \"deb [arch=amd64] https://packages.microsoft.com/repos/azure-cli/ \$(lsb_release -cs) main\" | sudo tee /etc/apt/sources.list.d/azure-cli.list"
    
    az vm run-command invoke \
        --command-id RunShellScript \
        --name $VM_NAME \
        --resource-group $RESOURCE_GROUP_NAME \
        --scripts "sudo apt-get update -y && sudo apt-get upgrade -y"

    az vm run-command invoke \
        --command-id RunShellScript \
        --name $VM_NAME \
        --resource-group $RESOURCE_GROUP_NAME \
        --scripts "sudo apt-get install -y azure-cli"

elif [ "$OS_SKU" = "CBLMariner" ] || [ "$OS_SKU" = "AzureLinux" ]; then
    az vm run-command invoke \
        --command-id RunShellScript \
        --name $VM_NAME \
        --resource-group $RESOURCE_GROUP_NAME \
        --scripts "sudo rpm --import https://packages.microsoft.com/keys/microsoft.asc &&
            sudo sh -c 'echo -e \"[azure-cli]\nname=Azure CLI\nbaseurl=https://packages.microsoft.com/yumrepos/azure-cli\nenabled=1\ngpgcheck=1\ngpgkey=https://packages.microsoft.com/keys/microsoft.asc\" > /etc/yum.repos.d/azure-cli.repo' &&
            sudo dnf install -y azure-cli"
else
    echo "I don't know what you are, exiting shell script."
    exit 1
fi

az vm run-command invoke \
    --command-id RunShellScript \
    --name $VM_NAME \
    --resource-group $RESOURCE_GROUP_NAME \
    --scripts "az login --identity"

TIMESTAMP=$(date +%s%3N)
TRIVY_REPORT_NAME="trivy-report-${BUILD_ID}-${TIMESTAMP}.json"
az vm run-command invoke \
    --command-id RunShellScript \
    --name $VM_NAME \
    --resource-group $RESOURCE_GROUP_NAME \
    --scripts "az storage blob upload --file /opt/azure/containers/trivy-report.json \
    --container-name ${SIG_CONTAINER_NAME} \
    --name ${TRIVY_REPORT_NAME} \
    --account-name ${STORAGE_ACCOUNT_NAME} \
    --auth-mode login"

TRIVY_TABLE_NAME="trivy-table-${BUILD_ID}-${TIMESTAMP}.txt"
az vm run-command invoke \
    --command-id RunShellScript \
    --name $VM_NAME \
    --resource-group $RESOURCE_GROUP_NAME \
    --scripts "az storage blob upload --file /opt/azure/containers/trivy-images-table.txt \
    --container-name ${SIG_CONTAINER_NAME} \
    --name ${TRIVY_TABLE_NAME} \
    --account-name ${STORAGE_ACCOUNT_NAME} \
    --auth-mode login"


az storage blob download --container-name ${SIG_CONTAINER_NAME} --name  ${TRIVY_REPORT_NAME} --file trivy-report.json --account-name ${STORAGE_ACCOUNT_NAME} --auth-mode login
az storage blob download --container-name ${SIG_CONTAINER_NAME} --name  ${TRIVY_TABLE_NAME} --file  trivy-images-table.txt --account-name ${STORAGE_ACCOUNT_NAME} --auth-mode login

az storage blob delete --account-name ${STORAGE_ACCOUNT_NAME} --container-name ${SIG_CONTAINER_NAME} --name ${TRIVY_REPORT_NAME} --auth-mode login
az storage blob delete --account-name ${STORAGE_ACCOUNT_NAME} --container-name ${SIG_CONTAINER_NAME} --name ${TRIVY_TABLE_NAME} --auth-mode login