#!/bin/bash
set -e

source ./parts/linux/cloud-init/artifacts/cse_benchmark_functions.sh

# Find the absolute path of the directory containing this script
SCRIPTS_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
CONFIG=$IMG_CUSTOMIZER_CONFIG
AGENTBAKER_DIR=`realpath $SCRIPTS_DIR/../../../../`
OUT_DIR="${AGENTBAKER_DIR}/out"
CREATE_TIME="$(date +%s)"

required_env_vars=(
    "AZURE_MSI_RESOURCE_STRING"
    "RESOURCE_GROUP_NAME"
    "SIG_IMAGE_NAME"
    "IMAGE_NAME"
    "SUBSCRIPTION_ID"
    "CAPTURED_SIG_VERSION"
    "PACKER_BUILD_LOCATION"
    "GENERATE_PUBLISHING_INFO"
)

for v in "${required_env_vars[@]}"
do
    if [ -z "${!v}" ]; then
        echo "$v was not set!"
        exit 1
    fi
done

# Default to this hard-coded value for Linux does not pass this environment variable into here
if [ -z "$SIG_GALLERY_NAME" ]; then
  SIG_GALLERY_NAME="PackerSigGalleryEastUS"
fi

capture_benchmark "${SCRIPT_NAME}_prepare_upload_vhd_to_blob"

echo "Uploading ${OUT_DIR}/${CONFIG}.vhd to ${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd"

az storage blob upload --container-name "$VHD_CONTAINER_NAME" --file "${OUT_DIR}/${CONFIG}.vhd" --name "${CAPTURED_SIG_VERSION}.vhd" --account-name "$STORAGE_ACCOUNT_NAME" --overwrite true --auth-mode login --type page

echo "Uploaded ${OUT_DIR}/${CONFIG}.vhd to ${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd"
capture_benchmark "${SCRIPT_NAME}_upload_vhd_to_blob"

# Use the domain name from the classic blob URL to get the storage account name.
# If the CLASSIC_BLOB var is not set create a new var called BLOB_STORAGE_NAME in the pipeline.
BLOB_URL_REGEX="^https:\/\/.+\.blob\.core\.windows\.net\/vhd(s)?$"
# shellcheck disable=SC3010
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

GALLERY_RESOURCE_ID=/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${RESOURCE_GROUP_NAME}/providers/Microsoft.Compute/galleries/${SIG_GALLERY_NAME}
SIG_IMAGE_RESOURCE_ID="${GALLERY_RESOURCE_ID}/images/${SIG_IMAGE_NAME}/versions/${CAPTURED_SIG_VERSION}"
MANAGED_IMAGE_RESOURCE_ID="/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${RESOURCE_GROUP_NAME}/providers/Microsoft.Compute/images/${IMAGE_NAME}"

# Determine target regions for image replication.
# Images must replicate to SIG region, and testing expects PACKER_BUILD_LOCATION
TARGET_REGIONS=${PACKER_BUILD_LOCATION}
GALLERY_LOCATION=$(az sig show --ids ${GALLERY_RESOURCE_ID} --query location -o tsv)
if [ "$GALLERY_LOCATION" != "$PACKER_BUILD_LOCATION" ]; then
    TARGET_REGIONS="${TARGET_REGIONS} ${GALLERY_LOCATION}"
fi

echo "Creating managed image ${MANAGED_IMAGE_RESOURCE_ID} from VHD ${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd"
az image create \
    --resource-group ${RESOURCE_GROUP_NAME} \
    --name ${IMAGE_NAME} \
    --source "${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd" \
    --os-type Linux \
    --storage-sku Standard_LRS \
    --hyper-v-generation V2 \
    --tags "buildDefinitionName=${BUILD_DEFINITION_NAME}" "buildNumber=${BUILD_NUMBER}" "buildId=${BUILD_ID}" "SkipLinuxAzSecPack=true" "os=Linux" "now=${CREATE_TIME}" "createdBy=aks-vhd-pipeline" "image_sku=${IMG_SKU}" "branch=${BRANCH}" \

echo "Creating SIG image version $SIG_IMAGE_RESOURCE_ID from managed image $MANAGED_IMAGE_RESOURCE_ID"
echo "Uploading to ${TARGET_REGIONS}"
az sig image-version create \
    --resource-group ${RESOURCE_GROUP_NAME} \
    --gallery-name ${SIG_GALLERY_NAME} \
    --gallery-image-definition ${SIG_IMAGE_NAME} \
    --gallery-image-version ${CAPTURED_SIG_VERSION} \
    --managed-image ${MANAGED_IMAGE_RESOURCE_ID} \
    --tags "buildDefinitionName=${BUILD_DEFINITION_NAME}" "buildNumber=${BUILD_NUMBER}" "buildId=${BUILD_ID}" "SkipLinuxAzSecPack=true" "os=Linux" "now=${CREATE_TIME}" "createdBy=aks-vhd-pipeline" "image_sku=${IMG_SKU}" "branch=${BRANCH}" \
    --target-regions ${TARGET_REGIONS}
capture_benchmark "${SCRIPT_NAME}_create_sig_image_version"

if [ "${GENERATE_PUBLISHING_INFO,,}" != "true" ]; then
    echo "Cleaning up ${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd from the storage account"
    azcopy remove "${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd" --recursive=true
else
    echo "GENERATE_PUBLISHING_INFO is true, skipping cleanup of ${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd"
fi

# Set SIG ID in pipeline for use during testing
echo "##vso[task.setvariable variable=MANAGED_SIG_ID]$SIG_IMAGE_RESOURCE_ID"
