#!/bin/bash
set -eux

TRIVY_SCRIPT_PATH="trivy-scan.sh"
EXE_SCRIPT_PATH="vhd-scanning-exe-on-vm.sh"
TEST_RESOURCE_PREFIX="vhd-scanning"
VM_NAME="$TEST_RESOURCE_PREFIX-vm"
VHD_IMAGE="$MANAGED_SIG_ID"

SIG_CONTAINER_NAME="vhd-scans"
TEST_VM_ADMIN_USERNAME="azureuser"

# This variable is used to determine where we need to deploy the VM on which we'll run trivy.
# We must be sure this location matches the location used by packer when delivering the output image
# version to the staging gallery, as the particular image version will only have a single replica in this region.
if [ -z "$PACKER_BUILD_LOCATION" ]; then
    echo "PACKER_BUILD_LOCATION must be set to run VHD scanning"
    exit 1
fi 

# Use the domain name from the classic blob URL to get the storage account name.
# If the CLASSIC_BLOB var is not set create a new var called BLOB_STORAGE_NAME in the pipeline.
BLOB_URL_REGEX="^https:\/\/.+\.blob\.core\.windows\.net\/vhd(s)?$"
if [[ $CLASSIC_BLOB =~ $BLOB_URL_REGEX ]]; then
    STORAGE_ACCOUNT_NAME=$(echo $CLASSIC_BLOB | sed -E 's|https://(.*)\.blob\.core\.windows\.net(:443)?/(.*)?|\1|')
else
    # Used in the 'AKS Linux VHD Build - PR check-in gate' pipeline.
    if [ -z "$BLOB_STORAGE_NAME" ]; then
        echo "BLOB_STORAGE_NAME is not set, please either set the CLASSIC_BLOB var or create a new var BLOB_STORAGE_NAME in the pipeline."
        exit 1
    fi
    STORAGE_ACCOUNT_NAME=${BLOB_STORAGE_NAME}
fi

set +x
TEST_VM_ADMIN_PASSWORD="TestVM@$(date +%s)"
set -x


RESOURCE_GROUP_NAME="$TEST_RESOURCE_PREFIX-$(date +%s)-$RANDOM"
az group create --name $RESOURCE_GROUP_NAME --location ${PACKER_BUILD_LOCATION} --tags 'source=AgentBaker'

# 18.04 VMs don't have access to new enough 'az' versions to be able to run the az commands in vhd-scanning-vm-exe.sh
if [ "$OS_VERSION" == "18.04" ]; then
    echo "Skipping scanning for 18.04"
    exit 0
fi

function cleanup() {
    echo "Deleting resource group ${RESOURCE_GROUP_NAME}"
    az group delete --name $RESOURCE_GROUP_NAME --yes --no-wait

    if [ -n "${VM_PRINCIPLE_ID}" ]; then
        az role assignment delete --assignee $VM_PRINCIPLE_ID --role "Storage Blob Data Contributor" --scope "/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${AZURE_RESOURCE_GROUP_NAME}"
        echo "Role assignment deleted."
    fi 
}
trap cleanup EXIT

VM_OPTIONS="--size Standard_D8ds_v5"
if [[ "${ARCHITECTURE,,}" == "arm64" ]]; then
    VM_OPTIONS="--size Standard_D8pds_v5"
fi

if [[ "${OS_TYPE}" == "Linux" && "${ENABLE_TRUSTED_LAUNCH}" == "True" ]]; then
    VM_OPTIONS+=" --security-type TrustedLaunch --enable-secure-boot true --enable-vtpm true"
fi

az vm create --resource-group $RESOURCE_GROUP_NAME \
    --name $VM_NAME \
    --image $VHD_IMAGE \
    --admin-username $TEST_VM_ADMIN_USERNAME \
    --admin-password $TEST_VM_ADMIN_PASSWORD \
    --os-disk-size-gb 50 \
    ${VM_OPTIONS} \
    --assign-identity "[system]"

VM_PRINCIPLE_ID=$(az vm identity show --name $VM_NAME --resource-group $RESOURCE_GROUP_NAME --query principalId --output tsv)
az role assignment create --assignee $VM_PRINCIPLE_ID --role "Storage Blob Data Contributor" --scope "/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${AZURE_RESOURCE_GROUP_NAME}"

FULL_PATH=$(realpath $0)
CDIR=$(dirname $FULL_PATH)
TRIVY_SCRIPT_PATH="$CDIR/$TRIVY_SCRIPT_PATH"
az vm run-command invoke \
    --command-id RunShellScript \
    --name $VM_NAME \
    --resource-group $RESOURCE_GROUP_NAME \
    --scripts @$TRIVY_SCRIPT_PATH


TIMESTAMP=$(date +%s%3N)
TRIVY_REPORT_NAME="trivy-report-${BUILD_ID}-${TIMESTAMP}.json"
TRIVY_TABLE_NAME="trivy-table-${BUILD_ID}-${TIMESTAMP}.txt"
EXE_SCRIPT_PATH="$CDIR/$EXE_SCRIPT_PATH"
az vm run-command invoke \
    --command-id RunShellScript \
    --name $VM_NAME \
    --resource-group $RESOURCE_GROUP_NAME \
    --scripts @$EXE_SCRIPT_PATH \
    --parameters "OS_SKU=${OS_SKU}" \
        "OS_VERSION=${OS_VERSION}" \
        "TEST_VM_ADMIN_USERNAME=${TEST_VM_ADMIN_USERNAME}" \
        "ARCHITECTURE=${ARCHITECTURE}" \
        "TRIVY_REPORT_NAME=${TRIVY_REPORT_NAME}" \
        "TRIVY_TABLE_NAME=${TRIVY_TABLE_NAME}" \
        "SIG_CONTAINER_NAME"=${SIG_CONTAINER_NAME} \
        "STORAGE_ACCOUNT_NAME"=${STORAGE_ACCOUNT_NAME} \
        "ENABLE_TRUSTED_LAUNCH"=${ENABLE_TRUSTED_LAUNCH}


az storage blob download --container-name ${SIG_CONTAINER_NAME} --name  ${TRIVY_REPORT_NAME} --file trivy-report.json --account-name ${STORAGE_ACCOUNT_NAME} --auth-mode login
az storage blob download --container-name ${SIG_CONTAINER_NAME} --name  ${TRIVY_TABLE_NAME} --file  trivy-images-table.txt --account-name ${STORAGE_ACCOUNT_NAME} --auth-mode login

az storage blob delete --account-name ${STORAGE_ACCOUNT_NAME} --container-name ${SIG_CONTAINER_NAME} --name ${TRIVY_REPORT_NAME} --auth-mode login
az storage blob delete --account-name ${STORAGE_ACCOUNT_NAME} --container-name ${SIG_CONTAINER_NAME} --name ${TRIVY_TABLE_NAME} --auth-mode login