#!/bin/bash -e

required_env_vars=(
    "SUBSCRIPTION_ID"
    "RESOURCE_GROUP_NAME"
    "CAPTURED_SIG_VERSION"
    "LOCATION"
    "OS_TYPE"
    "SIG_IMAGE_NAME"
)


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
# shellcheck disable=SC2102
sas=$(az disk grant-access --ids $disk_resource_id --duration-in-seconds 3600 --query [accessSas] -o tsv)

azcopy-preview copy "${sas}" "${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd${CLASSIC_SAS_TOKEN}" --recursive=true

echo "Converted $sig_resource_id to ${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd"

az disk revoke-access --ids $disk_resource_id 

az resource delete --ids $disk_resource_id

echo "Deleted $disk_resource_id"