#!/bin/bash
set -e

# This script publishes OSGuard VHD images built by Image Customizer to Azure Shared Image Gallery
# OSGuard images require certificates for secure boot, passed via DB_CERT_CA_PATH environment variable
# This script uses Bicep template for deployment to support certificate configuration

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
    "DB_CERT_CA_PATH"
)

for v in "${required_env_vars[@]}"
do
    if [ -z "${!v}" ]; then
        echo "$v was not set!"
        exit 1
    fi
done

# Validate certificate file exists
if [ ! -f "${DB_CERT_CA_PATH}" ]; then
    echo "Certificate file not found at: ${DB_CERT_CA_PATH}"
    exit 1
fi

# Default to this hard-coded value for Linux does not pass this environment variable into here
if [ -z "$SIG_GALLERY_NAME" ]; then
  SIG_GALLERY_NAME="PackerSigGalleryEastUS"
fi

capture_benchmark "${SCRIPT_NAME}_prepare_upload_vhd_to_blob"

echo "Uploading ${OUT_DIR}/${CONFIG}.vhd to ${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd"

echo "Setting azcopy environment variables with pool identity: $AZURE_MSI_RESOURCE_STRING"
export AZCOPY_AUTO_LOGIN_TYPE="MSI"
export AZCOPY_MSI_RESOURCE_STRING="$AZURE_MSI_RESOURCE_STRING"
export AZCOPY_CONCURRENCY_VALUE="AUTO"

export AZCOPY_LOG_LOCATION="$(pwd)/azcopy-log-files/"
export AZCOPY_JOB_PLAN_LOCATION="$(pwd)/azcopy-job-plan-files/"
mkdir -p "${AZCOPY_LOG_LOCATION}"
mkdir -p "${AZCOPY_JOB_PLAN_LOCATION}"

if ! azcopy copy "${OUT_DIR}/${CONFIG}.vhd" "${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd" --recursive=true ; then
    azExitCode=$?
    # loop through azcopy log files
    shopt -s nullglob
    for f in "${AZCOPY_LOG_LOCATION}"/*.log; do
        echo "Azcopy log file: $f"
        # upload the log file as an attachment to vso
        echo "##vso[build.uploadlog]$f"
        # check if the log file contains any errors
        if grep -q '"level":"Error"' "$f"; then
		 	echo "log file $f contains errors"
			echo "##vso[task.logissue type=error]Azcopy log file $f contains errors"
			# print the log file
			cat "$f"
        fi
    done
    shopt -u nullglob
	echo "Exiting with azcopy exit code $azExitCode"
    exit $azExitCode
fi

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
STORAGE_ACCOUNT_RESOURCE_ID="/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${RESOURCE_GROUP_NAME}/providers/Microsoft.Storage/storageAccounts/${STORAGE_ACCOUNT_NAME}"
VHD_BLOB_URL="${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd"

# Determine target regions for image replication.
# Images must replicate to SIG region, and testing expects PACKER_BUILD_LOCATION
TARGET_REGIONS=${PACKER_BUILD_LOCATION}
GALLERY_LOCATION=$(az sig show --ids ${GALLERY_RESOURCE_ID} --query location -o tsv)
if [ "$GALLERY_LOCATION" != "$PACKER_BUILD_LOCATION" ]; then
    TARGET_REGIONS="${TARGET_REGIONS} ${GALLERY_LOCATION}"
fi

echo "Creating OSGuard SIG image version $SIG_IMAGE_RESOURCE_ID using Bicep template"
echo "Uploading to ${TARGET_REGIONS}"
echo "DB_CERT_CA_PATH: ${DB_CERT_CA_PATH}"

# Process certificate for OSGuard secure boot
echo "Processing certificate from ${DB_CERT_CA_PATH} for OSGuard image"
CERT_BASE64=$(sed '0,/-----BEGIN CERTIFICATE-----/d;/-----END CERTIFICATE-----/d' "${DB_CERT_CA_PATH}" | tr -d "\n")

# Convert TARGET_REGIONS to JSON array for bicep template
REGIONS_JSON=$(echo "$TARGET_REGIONS" | awk '{
  printf "[";
  for(i=1;i<=NF;i++) {
    printf "\"%s\"", $i;
    if(i<NF) printf ",";
  }
  printf "]";
}')

# Use Bicep template for OSGuard images with certificate
echo "Deploying OSGuard image version with certificate using Bicep template"
az deployment group create \
    --subscription ${SUBSCRIPTION_ID} \
    --name osguard-${CAPTURED_SIG_VERSION} \
    --resource-group ${RESOURCE_GROUP_NAME} \
    --template-file ${AGENTBAKER_DIR}/osguard.bicep \
    --parameters galleryName=${SIG_GALLERY_NAME} \
                 imageDefinitionName=${SIG_IMAGE_NAME} \
                 versionName=${CAPTURED_SIG_VERSION} \
                 regions="$REGIONS_JSON" \
                 sourceDiskUrl=${VHD_BLOB_URL} \
                 certificateBase64=${CERT_BASE64}

capture_benchmark "${SCRIPT_NAME}_create_sig_image_version"

# Create a sig-info.json like in the main AZL AKS pipelines to provide the BYOI header to triggered test pipelines
cat <<EOF > "sig-info.json"
{
  "custom_header": "AKSHTTPCustomFeatures=Microsoft.ContainerService/UseCustomizedOSImage,OSImageSubscriptionID=${SUBSCRIPTION_ID},OSImageResourceGroup=${RESOURCE_GROUP_NAME},OSImageGallery=${SIG_GALLERY_NAME},OSImageName=${SIG_IMAGE_NAME},OSImageVersion=${CAPTURED_SIG_VERSION},OSSKU=AzureLinux,OSDistro=CustomizedImageLinuxGuard",
  "subscription_id": "${SUBSCRIPTION_ID}",
  "resource_group": "${RESOURCE_GROUP_NAME}",
  "gallery_name": "${SIG_GALLERY_NAME}",
  "image_name": "${SIG_IMAGE_NAME}",
  "image_version": "${CAPTURED_SIG_VERSION}",
  "create_time": "$(date +'%Y-%m-%dT%H:%M:%SZ')",
  "vhd_url_sas": "",
  "unique_id": "$(uuidgen)"
}
EOF

cat sig-info.json

if [ "${GENERATE_PUBLISHING_INFO,,}" != "true" ]; then
    echo "Cleaning up ${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd from the storage account"
    azcopy remove "${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd" --recursive=true
else
    echo "GENERATE_PUBLISHING_INFO is true, skipping cleanup of ${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd"
fi

# Set SIG ID in pipeline for use during testing
echo "##vso[task.setvariable variable=MANAGED_SIG_ID]$SIG_IMAGE_RESOURCE_ID"
