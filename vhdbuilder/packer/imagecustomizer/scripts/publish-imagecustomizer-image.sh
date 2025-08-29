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
    "INSERT_KMOD_CERT"
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

# Linux Kmod PCA cert to be added in image version
KMOD_UEFI_DB = "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"

capture_benchmark "${SCRIPT_NAME}_prepare_upload_vhd_to_blob"

echo "Uploading ${OUT_DIR}/${CONFIG}.vhd to ${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd"

echo "Setting azcopy environment variables with pool identity: $AZURE_MSI_RESOURCE_STRING"
export AZCOPY_AUTO_LOGIN_TYPE="MSI"
export AZCOPY_MSI_RESOURCE_STRING="$AZURE_MSI_RESOURCE_STRING"
export AZCOPY_CONCURRENCY_VALUE="AUTO"
azcopy copy "${OUT_DIR}/${CONFIG}.vhd" "${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd" --recursive=true || exit $?

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

# Build target regions JSON array for ARM template
TARGET_REGIONS_JSON=""
for region in ${TARGET_REGIONS}; do
        # Default storage account type and regional replica count
        if [ -n "${TARGET_REGIONS_JSON}" ]; then
                TARGET_REGIONS_JSON+=","
        fi
        TARGET_REGIONS_JSON+="{\"name\":\"${region}\",\"regionalReplicaCount\":1,\"storageAccountType\":\"Standard_LRS\"}"
done

# Add security profile if custom UEFI db key present
SECURITY_PROFILE_JSON=""
if [ -n "${INSERT_KMOD_CERT}" ]; then
    SECURITY_PROFILE_JSON=",
    \"securityProfile\": {
        \"uefiSettings\": {
            \"signatureTemplateNames\": [\"MicrosoftUefiCertificateAuthorityTemplate\"],
            \"additionalSignatures\": {
                \"db\": [
                    {
                        \"type\": \"x509\",
                        \"value\": [
                            \"${KMOD_UEFI_DB}\"
                        ]
                    }
                ]
            }
        }
    }"
fi

# Create ARM template to deploy gallery image version. Include securityProfile.uefiSettings only when custom UEFI key is provided.
if command -v mktemp >/dev/null 2>&1; then
        TMP_TEMPLATE=$(mktemp /tmp/create-imageversion-arm.json)
else
        TMP_TEMPLATE="/tmp/create-imageversion-arm.json"
fi

cat > "${TMP_TEMPLATE}" <<EOF
{
    "\$schema": "https://schema.management.azure.com/schemas/2019-04-01/deploymentTemplate.json#",
    "contentVersion": "1.0.0.0",
    "resources": [
        {
            "type": "Microsoft.Compute/galleries/images/versions",
            "apiVersion": "2025-03-03",
            "name": "${SIG_GALLERY_NAME}/${SIG_IMAGE_NAME}/${CAPTURED_SIG_VERSION}",
            "location": "${GALLERY_LOCATION}",
            "tags": {
                "buildDefinitionName": "${BUILD_DEFINITION_NAME}",
                "buildNumber": "${BUILD_NUMBER}",
                "buildId": "${BUILD_ID}",
                "SkipLinuxAzSecPack": "true",
                "os": "Linux",
                "now": "${CREATE_TIME}",
                "createdBy": "aks-vhd-pipeline",
                "image_sku": "${IMG_SKU}",
                "branch": "${BRANCH}"
            },
            "properties": {
                "storageProfile": {
                    "source": {
                        "id": "${MANAGED_IMAGE_RESOURCE_ID}"
                    }
                },
                "publishingProfile": {
                    "targetRegions": [${TARGET_REGIONS_JSON}]
                }${SECURITY_PROFILE_JSON}
            }
        }
    ]
}
EOF

# Deploy using an incremental deployment so we only create/update the image version resource
DEPLOY_NAME="create-sig-imageversion-${CAPTURED_SIG_VERSION}"
echo "Deploying ARM template to resource group ${RESOURCE_GROUP_NAME} as ${DEPLOY_NAME}"
az deployment group create \
        --resource-group "${RESOURCE_GROUP_NAME}" \
        --name "${DEPLOY_NAME}" \
        --template-file "${TMP_TEMPLATE}"


rm -f "${TMP_TEMPLATE}"

capture_benchmark "${SCRIPT_NAME}_create_sig_image_version"

if [ "${GENERATE_PUBLISHING_INFO,,}" != "true" ]; then
    echo "Cleaning up ${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd from the storage account"
    azcopy remove "${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd" --recursive=true
else
    echo "GENERATE_PUBLISHING_INFO is true, skipping cleanup of ${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd"
fi

# Set SIG ID in pipeline for use during testing
echo "##vso[task.setvariable variable=MANAGED_SIG_ID]$SIG_IMAGE_RESOURCE_ID"
