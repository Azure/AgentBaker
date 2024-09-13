#!/bin/bash
set -eux

TRIVY_SCRIPT_PATH="trivy-scan.sh"
EXE_SCRIPT_PATH="vhd-scanning-exe-on-vm.sh"
SCAN_RESOURCE_PREFIX="vhd-scanning"
SCAN_VM_NAME="$SCAN_RESOURCE_PREFIX-vm-$(date +%s)-$RANDOM"
VHD_IMAGE="$MANAGED_SIG_ID"

SIG_CONTAINER_NAME="vhd-scans"
SCAN_VM_ADMIN_USERNAME="azureuser"

# we must create VMs in a vnet which has access to the storage account, otherwise they will not be able to access the VHD blobs
VNET_NAME="nodesig-pool-vnet-${PACKER_BUILD_LOCATION}"
SUBNET_NAME="scanning"

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
SCAN_VM_ADMIN_PASSWORD="ScanVM@$(date +%s)"
set -x

RESOURCE_GROUP_NAME="$SCAN_RESOURCE_PREFIX-$(date +%s)-$RANDOM"
az group create --name $RESOURCE_GROUP_NAME --location ${PACKER_BUILD_LOCATION} --tags "source=AgentBaker" "branch=${GIT_BRANCH}"

function cleanup() {
    echo "Deleting resource group ${RESOURCE_GROUP_NAME}"
    az group delete --name $RESOURCE_GROUP_NAME --yes --no-wait
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
    --name $SCAN_VM_NAME \
    --image $VHD_IMAGE \
    --vnet-name $VNET_NAME \
    --subnet $SUBNET_NAME \
    --admin-username $SCAN_VM_ADMIN_USERNAME \
    --admin-password $SCAN_VM_ADMIN_PASSWORD \
    --os-disk-size-gb 50 \
    ${VM_OPTIONS} \
    --assign-identity "${UMSI_RESOURCE_ID}"

FULL_PATH=$(realpath $0)
CDIR=$(dirname $FULL_PATH)
TRIVY_SCRIPT_PATH="$CDIR/$TRIVY_SCRIPT_PATH"

TIMESTAMP=$(date +%s%3N)
TRIVY_UPLOAD_REPORT_NAME="trivy-report-${BUILD_ID}-${TIMESTAMP}.json"
TRIVY_UPLOAD_TABLE_NAME="trivy-table-${BUILD_ID}-${TIMESTAMP}.txt"

# Extract date, revision from build number
BUILD_RUN_NUMBER=$(echo $BUILD_RUN_NUMBER | cut -d_ -f 1)
az vm run-command invoke \
    --command-id RunShellScript \
    --name $SCAN_VM_NAME \
    --resource-group $RESOURCE_GROUP_NAME \
    --scripts @$TRIVY_SCRIPT_PATH \
    --parameters "OS_SKU=${OS_SKU}" \
        "OS_VERSION=${OS_VERSION}" \
        "SCAN_VM_ADMIN_USERNAME=${SCAN_VM_ADMIN_USERNAME}" \
        "ARCHITECTURE=${ARCHITECTURE}" \
        "SIG_CONTAINER_NAME"=${SIG_CONTAINER_NAME} \
        "STORAGE_ACCOUNT_NAME"=${STORAGE_ACCOUNT_NAME} \
        "ENABLE_TRUSTED_LAUNCH"=${ENABLE_TRUSTED_LAUNCH} \
        "VHD_NAME"=${VHD_NAME} \
        "SKU_NAME"=${SKU_NAME} \
        "KUSTO_ENDPOINT"=${KUSTO_ENDPOINT} \
        "KUSTO_DATABASE"=${KUSTO_DATABASE} \
        "KUSTO_TABLE"=${KUSTO_TABLE} \
        "TRIVY_UPLOAD_REPORT_NAME"=${TRIVY_UPLOAD_REPORT_NAME} \
        "TRIVY_UPLOAD_TABLE_NAME"=${TRIVY_UPLOAD_TABLE_NAME} \
        "ACCOUNT_NAME"=${ACCOUNT_NAME} \
        "BLOB_URL"=${BLOB_URL} \
        "SEVERITY"=${SEVERITY} \
        "MODULE_VERSION"=${MODULE_VERSION} \
        "UMSI_PRINCIPAL_ID"=${UMSI_PRINCIPAL_ID} \
        "UMSI_CLIENT_ID"=${UMSI_CLIENT_ID} \
        "BUILD_RUN_NUMBER"=${BUILD_RUN_NUMBER} \
        "BUILD_REPOSITORY_NAME"=${BUILD_REPOSITORY_NAME} \
        "BUILD_SOURCEBRANCH"=${GIT_BRANCH} \
        "BUILD_SOURCEVERSION"=${BUILD_SOURCEVERSION} \
        "SYSTEM_COLLECTIONURI"=${SYSTEM_COLLECTIONURI} \
        "SYSTEM_TEAMPROJECT"=${SYSTEM_TEAMPROJECT} \
        "BUILDID"=${BUILD_ID}

az storage blob download --container-name ${SIG_CONTAINER_NAME} --name  ${TRIVY_UPLOAD_REPORT_NAME} --file trivy-report.json --account-name ${STORAGE_ACCOUNT_NAME} --auth-mode login
az storage blob download --container-name ${SIG_CONTAINER_NAME} --name  ${TRIVY_UPLOAD_TABLE_NAME} --file  trivy-images-table.txt --account-name ${STORAGE_ACCOUNT_NAME} --auth-mode login

az storage blob delete --account-name ${STORAGE_ACCOUNT_NAME} --container-name ${SIG_CONTAINER_NAME} --name ${TRIVY_UPLOAD_REPORT_NAME} --auth-mode login
az storage blob delete --account-name ${STORAGE_ACCOUNT_NAME} --container-name ${SIG_CONTAINER_NAME} --name ${TRIVY_UPLOAD_TABLE_NAME} --auth-mode login

echo -e "Trivy Scan Script Completed\n\n\n"
