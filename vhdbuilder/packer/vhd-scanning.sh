#!/bin/bash
set -eux

source ./parts/linux/cloud-init/artifacts/cse_benchmark_functions.sh

# This variable is used to determine where we need to deploy the VM on which we'll run trivy.
# We must be sure this location matches the location used by packer when delivering the output image
# version to the staging gallery, as the particular image version will only have a single replica in this region.
if [ -z "$PACKER_BUILD_LOCATION" ]; then
    echo "PACKER_BUILD_LOCATION must be set to run VHD scanning"
    exit 1
fi

TRIVY_SCRIPT_PATH="trivy-scan.sh"
SCAN_RESOURCE_PREFIX="vhd-scanning"
SCAN_VM_NAME="$SCAN_RESOURCE_PREFIX-vm-$(date +%s)-$RANDOM"
VHD_IMAGE="$MANAGED_SIG_ID"

SIG_CONTAINER_NAME="vhd-scans"
SCAN_VM_ADMIN_USERNAME="azureuser"

# we must create VMs in a vnet subnet which has access to the storage account, otherwise they will not be able to access the VHD blobs
SCANNING_SUBNET_ID="/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${PACKER_VNET_RESOURCE_GROUP_NAME}/providers/Microsoft.Network/virtualNetworks/${PACKER_VNET_NAME}/subnets/scanning"
if [ -z "$(az network vnet subnet show --ids $SCANNING_SUBNET_ID | jq -r '.id')" ]; then
    echo "scanning subnet $SCANNING_SUBNET_ID seems to be missing, unable to create scanning VM"
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
az group create --name $RESOURCE_GROUP_NAME --location ${PACKER_BUILD_LOCATION} --tags "source=AgentBaker" "now=$(date +%s)" "branch=${GIT_BRANCH}"

function cleanup() {
    echo "Deleting resource group ${RESOURCE_GROUP_NAME}"
    az group delete --name $RESOURCE_GROUP_NAME --yes --no-wait
}
trap cleanup EXIT
capture_benchmark "${SCRIPT_NAME}_set_variables_and_create_scan_resource_group"

VM_OPTIONS="--size Standard_D8ds_v5"
if [[ "${ARCHITECTURE,,}" == "arm64" ]]; then
    VM_OPTIONS="--size Standard_D8pds_v5"
fi

if [[ "${OS_TYPE}" == "Linux" && "${ENABLE_TRUSTED_LAUNCH}" == "True" ]]; then
    VM_OPTIONS+=" --security-type TrustedLaunch --enable-secure-boot true --enable-vtpm true"
fi

if [ "${OS_TYPE}" == "Linux" ] && [[ "${IMG_SKU}" == "20_04-lts-cvm" ] || [ "${IMG_SKU}" == "cvm" ]]; then
    # We completely re-assign the VM_OPTIONS string here to ensure that no artifacts from earlier conditionals are included
    VM_OPTIONS="--size Standard_DC8ads_v5 --security-type ConfidentialVM --enable-secure-boot true --enable-vtpm true --os-disk-security-encryption-type VMGuestStateOnly --specialized true"
fi

SCANNING_NIC_ID=$(az network nic create --resource-group $RESOURCE_GROUP_NAME --name "scanning$(date +%s)${RANDOM}" --subnet $SCANNING_SUBNET_ID | jq -r '.NewNIC.id')
if [ -z "$SCANNING_NIC_ID" ]; then
    echo "unable to create new NIC for scanning VM"
    exit 1
fi

az vm create --resource-group $RESOURCE_GROUP_NAME \
    --name $SCAN_VM_NAME \
    --image $VHD_IMAGE \
    --nics $SCANNING_NIC_ID \
    --admin-username $SCAN_VM_ADMIN_USERNAME \
    --admin-password $SCAN_VM_ADMIN_PASSWORD \
    --os-disk-size-gb 50 \
    ${VM_OPTIONS} \
    --assign-identity "${UMSI_RESOURCE_ID}"
    
capture_benchmark "${SCRIPT_NAME}_create_scan_vm"
set +x

# for scanning storage account/container upload access
az vm identity assign -g $RESOURCE_GROUP_NAME --name $SCAN_VM_NAME --identities $AZURE_MSI_RESOURCE_STRING

FULL_PATH=$(realpath $0)
CDIR=$(dirname $FULL_PATH)
TRIVY_SCRIPT_PATH="$CDIR/$TRIVY_SCRIPT_PATH"

TIMESTAMP=$(date +%s%3N)
TRIVY_UPLOAD_REPORT_NAME="trivy-report-${BUILD_ID}-${TIMESTAMP}.json"
TRIVY_UPLOAD_TABLE_NAME="trivy-table-${BUILD_ID}-${TIMESTAMP}.txt"

# Extract date, revision from build number
BUILD_RUN_NUMBER=$(echo $BUILD_RUN_NUMBER | cut -d_ -f 1)

# set image version locally, if it is not set in environment variable
if [ -z "${IMAGE_VERSION:-}" ]; then
    IMAGE_VERSION=$(date +%Y%m.%d.0)
    echo "IMAGE_VERSION was not set, setting it to ${IMAGE_VERSION} for trivy scan and Kusto ingestion"
fi

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
        "VHD_ARTIFACT_NAME"=${VHD_ARTIFACT_NAME} \
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
        "AZURE_MSI_RESOURCE_STRING"=${AZURE_MSI_RESOURCE_STRING} \
        "BUILD_RUN_NUMBER"=${BUILD_RUN_NUMBER} \
        "BUILD_REPOSITORY_NAME"=${BUILD_REPOSITORY_NAME} \
        "BUILD_SOURCEBRANCH"=${GIT_BRANCH} \
        "BUILD_SOURCEVERSION"=${BUILD_SOURCEVERSION} \
        "SYSTEM_COLLECTIONURI"=${SYSTEM_COLLECTIONURI} \
        "SYSTEM_TEAMPROJECT"=${SYSTEM_TEAMPROJECT} \
        "BUILDID"=${BUILD_ID} \
        "IMAGE_VERSION"=${IMAGE_VERSION}

capture_benchmark "${SCRIPT_NAME}_run_az_scan_command"

az storage blob download --container-name ${SIG_CONTAINER_NAME} --name  ${TRIVY_UPLOAD_REPORT_NAME} --file trivy-report.json --account-name ${STORAGE_ACCOUNT_NAME} --auth-mode login
az storage blob download --container-name ${SIG_CONTAINER_NAME} --name  ${TRIVY_UPLOAD_TABLE_NAME} --file  trivy-images-table.txt --account-name ${STORAGE_ACCOUNT_NAME} --auth-mode login

az storage blob delete --account-name ${STORAGE_ACCOUNT_NAME} --container-name ${SIG_CONTAINER_NAME} --name ${TRIVY_UPLOAD_REPORT_NAME} --auth-mode login
az storage blob delete --account-name ${STORAGE_ACCOUNT_NAME} --container-name ${SIG_CONTAINER_NAME} --name ${TRIVY_UPLOAD_TABLE_NAME} --auth-mode login
capture_benchmark "${SCRIPT_NAME}_download_and_delete_blobs"

echo -e "Trivy Scan Script Completed\n\n\n"
capture_benchmark "${SCRIPT_NAME}_overall" true
process_benchmarks
