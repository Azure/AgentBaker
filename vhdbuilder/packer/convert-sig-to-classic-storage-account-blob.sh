#!/bin/bash -e

required_env_vars=(
    "SUBSCRIPTION_ID"
    "RESOURCE_GROUP_NAME"
    "CAPTURED_SIG_VERSION"
    "LOCATION"
    "OS_TYPE"
    "SIG_IMAGE_NAME"
)

if [[ "${OS_NAME,,}" != "windows" ]]; then
    echo "linux VHD - generating user-delegation SAS token for classic storage account conversion"
    [ -z "${OUTPUT_STORAGE_ACCOUNT_NAME}" ] && echo "OUTPUT_STORAGE_ACCOUNT_NAME should be set when generating user-delegation SAS token with agent identity" && exit 1
    [ -z "${OUTPUT_STORAGE_CONTAINER_NAME}" ] && echo "OUTPUT_STORAGE_CONTAINER_NAME should be set when generating user-delegation SAS token with agent identity" && exit 1
    echo "storage account name: ${OUTPUT_STORAGE_ACCOUNT_NAME}"
    echo "storage container name: ${OUTPUT_STORAGE_CONTAINER_NAME}"
    # max of 7 day expiration time when using user delegation SAS
    expiry_date=$(date +"%Y-%m-%dT00:00Z" -d "+7 day")
    storage_sas_token=$(az storage container generate-sas --account-name ${OUTPUT_STORAGE_ACCOUNT_NAME} --name ${OUTPUT_STORAGE_CONTAINER_NAME} --permissions acwlr --expiry ${expiry_date} --auth-mode login --as-user | tr -d '"')
else
    echo "windows VHD - generating traditional SAS token with CLASSIC_SA_CONNECTION_STRING for classic storage account conversion"
    [ -z "${CLASSIC_SA_CONNECTION_STRING}" ] && echo "CLASSIC_SA_CONNECTION_STRING should be set when generating traditional SAS token" && exit 1
    start_date=$(date +"%Y-%m-%dT00:00Z" -d "-1 day")
    expiry_date=$(date +"%Y-%m-%dT00:00Z" -d "+1 year")
    storage_sas_token=$(az storage container generate-sas --name vhds --permissions acwlr --connection-string ${CLASSIC_SA_CONNECTION_STRING} --start ${start_date} --expiry ${expiry_date} | tr -d '"')
fi

if [ "$storage_sas_token" == "" ]; then
    echo "sas_token is empty"
    exit 1
fi

for v in "${required_env_vars[@]}"
do
    if [ -z "${!v}" ]; then
        echo "$v was not set!"
        exit 1
    fi
done

# Default to this hard-coded value for Linux does not pass this environment variable into here
if [[ -z "$SIG_GALLERY_NAME" ]]; then
  SIG_GALLERY_NAME="PackerSigGalleryEastUS"
fi

echo "SIG_IMAGE_VERSION before checking and assigning is $SIG_IMAGE_VERSION"
# Windows Gen 2: use the passed environment variable $SIG_IMAGE_VERSION
# Linux Gen 2: assign $CAPTURED_SIG_VERSION to $SIG_IMAGE_VERSION
if [[ -z "$SIG_IMAGE_VERSION" ]]; then
  SIG_IMAGE_VERSION=${CAPTURED_SIG_VERSION}
fi

# Use $SIG_IMAGE_VERSION for the sig resource, and $CAPTURED_SIG_VERSION (a random number) for the disk resource
sig_resource_id="/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${RESOURCE_GROUP_NAME}/providers/Microsoft.Compute/galleries/${SIG_GALLERY_NAME}/images/${SIG_IMAGE_NAME}/versions/${SIG_IMAGE_VERSION}"
disk_resource_id="/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${RESOURCE_GROUP_NAME}/providers/Microsoft.Compute/disks/${CAPTURED_SIG_VERSION}"

echo "Converting $sig_resource_id to $disk_resource_id"
if [[ ${OS_TYPE} == "Linux" && ${ENABLE_TRUSTED_LAUNCH} == "True" ]]; then
  az resource create --id $disk_resource_id  --is-full-object --location $LOCATION --properties "{\"location\": \"$LOCATION\", \
    \"properties\": { \
      \"osType\": \"$OS_TYPE\", \
      \"securityProfile\": { \
        \"securityType\": \"TrustedLaunch\" \
      }, \
      \"creationData\": { \
        \"createOption\": \"FromImage\", \
        \"galleryImageReference\": { \
          \"id\": \"${sig_resource_id}\" \
        } \
      } \
    } \
  }"
else
  az resource create --id $disk_resource_id  --is-full-object --location $LOCATION --properties "{\"location\": \"$LOCATION\", \
    \"properties\": { \
      \"osType\": \"$OS_TYPE\", \
      \"creationData\": { \
        \"createOption\": \"FromImage\", \
        \"galleryImageReference\": { \
          \"id\": \"${sig_resource_id}\" \
        } \
      } \
    } \
  }"
fi
echo "Converted $sig_resource_id to $disk_resource_id"

# shellcheck disable=SC2102
sas=$(az disk grant-access --ids $disk_resource_id --duration-in-seconds 3600 --query [accessSas] -o tsv)

echo "Uploading $disk_resource_id to ${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd"

azcopy-preview copy "${sas}" "${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd?${storage_sas_token}" --recursive=true

echo "Uploaded $disk_resource_id to ${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd"

az disk revoke-access --ids $disk_resource_id 

az resource delete --ids $disk_resource_id

echo "Deleted $disk_resource_id"