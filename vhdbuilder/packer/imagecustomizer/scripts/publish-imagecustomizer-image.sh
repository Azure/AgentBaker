#!/bin/bash
set -e

# Enhanced VHD Publishing Script with UEFI Certificate Support
#
# This script handles VHD publishing to Azure Shared Image Gallery with optional
# UEFI secure boot certificate injection.
#
# Required Environment Variables:
#   - AZURE_MSI_RESOURCE_STRING: MSI resource string for authentication
#   - RESOURCE_GROUP_NAME: Azure resource group name
#   - SIG_IMAGE_NAME: Shared Image Gallery image name
#   - IMAGE_NAME: Managed image name
#   - SUBSCRIPTION_ID: Azure subscription ID
#   - CAPTURED_SIG_VERSION: Version for the SIG image
#   - PACKER_BUILD_LOCATION: Azure region for the build
#   - GENERATE_PUBLISHING_INFO: Whether to generate publishing information
#   - IMG_CUSTOMIZER_CONFIG: Image customizer configuration name
#   - CLASSIC_BLOB: Blob storage URL for VHD storage
#
# Optional Environment Variables for UEFI:
#   - ENABLE_UEFI_SECURE_BOOT: Set to "true" to enable UEFI certificate injection
#   - DEBUG: Set to "true" to enable debug output in injection script
#
# The script will:
#   1. Download VHD from blob storage if not present locally
#   2. Download UEFI certificate (ca-cert.pem) from blob storage if available
#   3. Process and inject UEFI certificate into VHD if enabled
#   4. Upload modified VHD to blob storage
#   5. Create managed image and SIG image version
#   6. Set pipeline variables with certificate information

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
echo "Contents of OUT_DIR (${OUT_DIR}):"
ls -l "${OUT_DIR}"

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

# Check if VHD exists locally, if not try to download from blob storage
VHD_FILE="${OUT_DIR}/${CONFIG}.vhd"
VHD_BLOB_URL="${CLASSIC_BLOB}/${CONFIG}.vhd"

if [ ! -f "$VHD_FILE" ]; then
    echo "VHD not found locally at ${VHD_FILE}, attempting to download from blob storage"
    echo "Downloading from ${VHD_BLOB_URL}"
    
    if azcopy copy "${VHD_BLOB_URL}" "${VHD_FILE}" --check-length=false; then
        echo "Successfully downloaded VHD from blob storage"
        
        # Verify VHD file integrity
        if [ -f "$VHD_FILE" ]; then
            VHD_SIZE=$(stat -c%s "$VHD_FILE" 2>/dev/null || stat -f%z "$VHD_FILE" 2>/dev/null)
            echo "Downloaded VHD size: ${VHD_SIZE} bytes"
        else
            echo "Error: VHD file not found after download"
            exit 1
        fi
    else
        echo "Failed to download VHD from blob storage, will use local build process"
    fi
else
    echo "Using existing VHD file at ${VHD_FILE}"
fi

echo "Uploading ${VHD_FILE} to ${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd"

echo "Setting azcopy environment variables with pool identity: $AZURE_MSI_RESOURCE_STRING"
export AZCOPY_AUTO_LOGIN_TYPE="MSI"
export AZCOPY_MSI_RESOURCE_STRING="$AZURE_MSI_RESOURCE_STRING"
export AZCOPY_CONCURRENCY_VALUE="AUTO"

export AZCOPY_LOG_LOCATION="$(pwd)/azcopy-log-files/"
export AZCOPY_JOB_PLAN_LOCATION="$(pwd)/azcopy-job-plan-files/"
mkdir -p "${AZCOPY_LOG_LOCATION}"
mkdir -p "${AZCOPY_JOB_PLAN_LOCATION}"

if ! azcopy copy "${VHD_FILE}" "${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd" --recursive=true ; then
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

echo "Uploaded ${VHD_FILE} to ${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd"
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

# Download and process UEFI certificate from blob storage
CERT_FILE="${OUT_DIR}/ca-cert.pem"
CERT_BLOB_URL="${CLASSIC_BLOB}/ca-cert.pem"

echo "Checking for UEFI certificate at ${CERT_BLOB_URL}"

# Download certificate from blob storage if it exists
if azcopy copy "${CERT_BLOB_URL}" "${CERT_FILE}" --check-length=false 2>/dev/null; then
    echo "Downloaded certificate from blob storage: ${CERT_BLOB_URL}"
elif [ -f "$CERT_FILE" ]; then
    echo "Using existing certificate file at $CERT_FILE"
else
    echo "No UEFI certificate found at ${CERT_BLOB_URL} or ${CERT_FILE}"
    echo "Continuing without UEFI certificate processing"
fi

# Process certificate if it exists
if [ -f "$CERT_FILE" ]; then
    echo "Processing UEFI certificate from $CERT_FILE"
    
    # Verify certificate format
    if openssl x509 -in "$CERT_FILE" -noout 2>/dev/null; then
        echo "Certificate verified as valid PEM format"
        
        # Convert PEM to DER, then to base64
        UEFI_SECURE_BOOT_CERT=$(openssl x509 -in "$CERT_FILE" -outform DER | base64 -w 0)
        
        if [ $? -eq 0 ] && [ -n "$UEFI_SECURE_BOOT_CERT" ]; then
            echo "Successfully converted certificate to base64 DER format (length: ${#UEFI_SECURE_BOOT_CERT})"
            export UEFI_SECURE_BOOT_CERT
            
            # Log certificate details for debugging
            CERT_SUBJECT=$(openssl x509 -in "$CERT_FILE" -noout -subject 2>/dev/null | sed 's/subject=//')
            CERT_ISSUER=$(openssl x509 -in "$CERT_FILE" -noout -issuer 2>/dev/null | sed 's/issuer=//')
            CERT_NOT_BEFORE=$(openssl x509 -in "$CERT_FILE" -noout -startdate 2>/dev/null | sed 's/notBefore=//')
            CERT_NOT_AFTER=$(openssl x509 -in "$CERT_FILE" -noout -enddate 2>/dev/null | sed 's/notAfter=//')
            
            echo "Certificate Subject: $CERT_SUBJECT"
            echo "Certificate Issuer: $CERT_ISSUER"
            echo "Certificate Valid From: $CERT_NOT_BEFORE"
            echo "Certificate Valid To: $CERT_NOT_AFTER"
        else
            echo "Error: Failed to convert certificate from $CERT_FILE"
            exit 1
        fi
    else
        echo "Error: Invalid certificate format in $CERT_FILE"
        exit 1
    fi
fi

# UEFI Certificate Integration with VHD
if [ -n "${UEFI_SECURE_BOOT_CERT:-}" ]; then
    echo "UEFI secure boot certificate found and processed (length: ${#UEFI_SECURE_BOOT_CERT})"
    
    # Check if UEFI certificate injection is enabled
    if [ "${ENABLE_UEFI_SECURE_BOOT:-false}" = "true" ]; then
        echo "UEFI secure boot enabled - attempting certificate injection into VHD"
        
        # Create temporary directory for VHD manipulation
        TEMP_VHD_DIR="/tmp/vhd-uefi-$$"
        mkdir -p "$TEMP_VHD_DIR"
        
        # Use dedicated UEFI certificate injection script
        echo "Using dedicated script for UEFI certificate injection"
        
        # Create temporary certificate file
        echo "${UEFI_SECURE_BOOT_CERT}" | base64 -d > "$TEMP_VHD_DIR/uefi-secure-boot.der"
        openssl x509 -inform DER -in "$TEMP_VHD_DIR/uefi-secure-boot.der" -outform PEM -out "$TEMP_VHD_DIR/uefi-secure-boot.pem"
        
        # Use the injection script
        INJECTION_SCRIPT="${SCRIPTS_DIR}/inject-uefi-certificate.sh"
        if [ -f "$INJECTION_SCRIPT" ]; then
            if bash "$INJECTION_SCRIPT" "${VHD_FILE}" "$TEMP_VHD_DIR/uefi-secure-boot.pem" "${DEBUG:-false}"; then
                echo "UEFI certificate successfully injected into VHD using dedicated script"
                echo "##vso[task.setvariable variable=UEFI_CERT_INJECTED]true"
            else
                echo "Warning: UEFI certificate injection failed, using pipeline variable method"
                echo "##vso[task.setvariable variable=UEFI_CERT_INJECTED]false"
            fi
        else
            echo "Warning: UEFI injection script not found at $INJECTION_SCRIPT"
            echo "##vso[task.setvariable variable=UEFI_CERT_INJECTED]false"
        fi
        
        # Cleanup temporary directory
        rm -rf "$TEMP_VHD_DIR"
    else
        echo "UEFI secure boot not enabled, certificate will be stored as pipeline variable only"
        echo "##vso[task.setvariable variable=UEFI_CERT_INJECTED]false"
    fi
    
    # Always set the certificate as pipeline variable for later use
    echo "##vso[task.setvariable variable=UEFI_SECURE_BOOT_CERT]${UEFI_SECURE_BOOT_CERT}"
    
    # Note: UEFI secure boot settings are not supported by 'az sig image-version create'
    # UEFI settings must be configured at VM creation time, not at SIG image creation time
    echo "Note: Certificate processed for UEFI secure boot compatibility"
else
    echo "No UEFI secure boot certificate found"
    echo "##vso[task.setvariable variable=UEFI_CERT_INJECTED]false"
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
