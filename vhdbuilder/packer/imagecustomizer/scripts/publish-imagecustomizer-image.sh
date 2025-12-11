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

echo "Setting azcopy environment variables with pool identity: $AZURE_MSI_RESOURCE_STRING"
export AZCOPY_AUTO_LOGIN_TYPE="AZCLI"
export AZCOPY_CONCURRENCY_VALUE="AUTO"

export AZCOPY_LOG_LOCATION="$(pwd)/azcopy-log-files/"
export AZCOPY_JOB_PLAN_LOCATION="$(pwd)/azcopy-job-plan-files/"
mkdir -p "${AZCOPY_LOG_LOCATION}"
mkdir -p "${AZCOPY_JOB_PLAN_LOCATION}"

if [ "${ENVIRONMENT,,}" = "tme" ]; then
    # If environment is TME, we use a staging container in order to later copy the blob to an immutable container.
    DESTINATION_STORAGE_CONTAINER=${CLASSIC_BLOB_STAGING}
    STAGING_CONTAINER_EXISTS=$(az storage container exists --account-name ${STORAGE_ACCOUNT_NAME} --name $VHD_STAGING_CONTAINER_NAME --auth-mode login | jq -r '.exists')
    if [ "${STAGING_CONTAINER_EXISTS,,}" = "false" ]; then
        echo "Creating staging container $VHD_STAGING_CONTAINER_NAME in storage account $STORAGE_ACCOUNT_NAME"
        az storage container create --account-name "$STORAGE_ACCOUNT_NAME" --name "$VHD_STAGING_CONTAINER_NAME" --auth-mode login || exit 1
    else
        echo "Staging container $VHD_STAGING_CONTAINER_NAME already exists in storage account $STORAGE_ACCOUNT_NAME"
    fi
else
    DESTINATION_STORAGE_CONTAINER=${CLASSIC_BLOB}
fi

AZCOPYCMD="azcopy copy \"${OUT_DIR}/${CONFIG}.vhd\" \"${DESTINATION_STORAGE_CONTAINER}/${CAPTURED_SIG_VERSION}.vhd\""

echo "Uploading ${OUT_DIR}/${CONFIG}.vhd to ${DESTINATION_STORAGE_CONTAINER}/${CAPTURED_SIG_VERSION}.vhd"
if ! "${AZCOPYCMD}" --recursive=true ; then
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

echo "Uploaded ${OUT_DIR}/${CONFIG}.vhd to ${DESTINATION_STORAGE_CONTAINER}/${CAPTURED_SIG_VERSION}.vhd"

if [ "${ENVIRONMENT,,}" = "tme"  ] && [ "${GENERATE_PUBLISHING_INFO,,}" = "true" ]; then
    echo "Copying ${DESTINATION_STORAGE_CONTAINER}/${CAPTURED_SIG_VERSION}.vhd to immutable storage container"
    az storage blob copy start --account-name "$STORAGE_ACCOUNT_NAME" --destination-blob "${CAPTURED_SIG_VERSION}.vhd" --destination-container "$VHD_CONTAINER_NAME" --source-uri "${DESTINATION_STORAGE_CONTAINER}/${CAPTURED_SIG_VERSION}.vhd" --auth-mode login || exit 1
    echo "Successfully copied to immutable container"
else
    echo "GENERATE_PUBLISHING_INFO is false or we are in a testing / prod environment, skipping copying ${DESTINATION_STORAGE_CONTAINER}/${CAPTURED_SIG_VERSION}.vhd to immutable storage container"
fi
capture_benchmark "${SCRIPT_NAME}_upload_vhd_to_blob"

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

echo "Creating managed image ${MANAGED_IMAGE_RESOURCE_ID} from VHD ${DESTINATION_STORAGE_CONTAINER}/${CAPTURED_SIG_VERSION}.vhd"
az image create \
    --resource-group ${RESOURCE_GROUP_NAME} \
    --name ${IMAGE_NAME} \
    --source "${DESTINATION_STORAGE_CONTAINER}/${CAPTURED_SIG_VERSION}.vhd" \
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

if [ "${ENVIRONMENT,,}" = "tme" ] || [ "${GENERATE_PUBLISHING_INFO,,}" = "false" ]; then
    azcopy remove "${DESTINATION_STORAGE_CONTAINER}/${CAPTURED_SIG_VERSION}.vhd" --recursive=true
fi

# Set SIG ID in pipeline for use during testing
echo "##vso[task.setvariable variable=MANAGED_SIG_ID]$SIG_IMAGE_RESOURCE_ID"
