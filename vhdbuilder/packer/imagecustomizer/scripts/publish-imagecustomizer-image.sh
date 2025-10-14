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

# Optional env vars for UEFI secure boot
optional_env_vars=(
    "UEFI_SECURE_BOOT_CERT"
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

# Build the az sig image-version create command
SIG_CREATE_CMD="az sig image-version create \
    --resource-group ${RESOURCE_GROUP_NAME} \
    --gallery-name ${SIG_GALLERY_NAME} \
    --gallery-image-definition ${SIG_IMAGE_NAME} \
    --gallery-image-version ${CAPTURED_SIG_VERSION} \
    --managed-image ${MANAGED_IMAGE_RESOURCE_ID} \
    --tags \"buildDefinitionName=${BUILD_DEFINITION_NAME}\" \"buildNumber=${BUILD_NUMBER}\" \"buildId=${BUILD_ID}\" \"SkipLinuxAzSecPack=true\" \"os=Linux\" \"now=${CREATE_TIME}\" \"createdBy=aks-vhd-pipeline\" \"image_sku=${IMG_SKU}\" \"branch=${BRANCH}\" \
    --target-regions ${TARGET_REGIONS}"

# Convert PEM certificate to base64 DER format if ca_cert.pem exists
CERT_FILE="${OUT_DIR}/ca_cert.pem"
if [ -f "$CERT_FILE" ]; then
    echo "Found certificate file at $CERT_FILE, converting PEM to base64 DER format"
    
    # Convert PEM to DER, then to base64
    UEFI_SECURE_BOOT_CERT=$(openssl x509 -in "$CERT_FILE" -outform DER | base64 -w 0)
    
    if [ $? -eq 0 ] && [ -n "$UEFI_SECURE_BOOT_CERT" ]; then
        echo "Successfully converted certificate to base64 DER format"
        export UEFI_SECURE_BOOT_CERT
    else
        echo "Error: Failed to convert certificate from $CERT_FILE"
        exit 1
    fi
fi

# Add UEFI security profile if certificate is provided
if [ -n "${UEFI_SECURE_BOOT_CERT:-}" ]; then
    echo "Adding UEFI secure boot certificate to SIG image version"
    SIG_CREATE_CMD="${SIG_CREATE_CMD} --security-type TrustedLaunch --uefi-settings '{\"signatureTemplateNames\":[\"NoSignatureTemplate\"],\"additionalSignatures\":{\"pk\":{\"type\":\"x509\",\"value\":[\"${UEFI_SECURE_BOOT_CERT}\"]}}}'"
fi

# Execute the command
eval $SIG_CREATE_CMD
capture_benchmark "${SCRIPT_NAME}_create_sig_image_version"

if [ "${GENERATE_PUBLISHING_INFO,,}" != "true" ]; then
    echo "Cleaning up ${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd from the storage account"
    azcopy remove "${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd" --recursive=true
else
    echo "GENERATE_PUBLISHING_INFO is true, skipping cleanup of ${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd"
fi

# Set SIG ID in pipeline for use during testing
echo "##vso[task.setvariable variable=MANAGED_SIG_ID]$SIG_IMAGE_RESOURCE_ID"
