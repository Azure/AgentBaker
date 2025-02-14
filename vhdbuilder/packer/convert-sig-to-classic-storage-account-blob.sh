#!/bin/bash
set -e
# avoid using set -x in this pipeline as you'll end up logging a sensitive access token down below.

source ./parts/linux/cloud-init/artifacts/cse_benchmark_functions.sh

required_env_vars=(
    "AZURE_MSI_RESOURCE_STRING"
    "SUBSCRIPTION_ID"
    "RESOURCE_GROUP_NAME"
    "CAPTURED_SIG_VERSION"
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

if [ "${OS_TYPE,,}" == "windows" ]; then
  if [ -z "$LOCATION" ]; then
    echo "LOCATION must be set for windows builds"
    exit 1
  fi
fi

if [ "${OS_TYPE,,}" == "linux" ]; then
  if [ -z "$PACKER_BUILD_LOCATION" ]; then
    echo "PACKER_BUILD_LOCATION must be set for linux builds"
    exit 1
  fi
  LOCATION=$PACKER_BUILD_LOCATION
fi

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
capture_benchmark "${SCRIPT_NAME}_set_variables_for_converting_to_disk"

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
elif [ "${OS_TYPE}" == "Linux" ] && [[ "${IMG_SKU}" == "20_04-lts-cvm"  ||  "${IMG_SKU}" == "cvm" ]]; then
  az resource create --id $disk_resource_id  --is-full-object --location $LOCATION --properties "{\"location\": \"$LOCATION\", \
    \"properties\": { \
      \"osType\": \"$OS_TYPE\", \
      \"securityProfile\": { \
        \"securityType\": \"ConfidentialVM_VMGuestStateOnlyEncryptedWithPlatformKey\" \
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
capture_benchmark "${SCRIPT_NAME}_convert_image_version_to_disk"

echo "Granting access to $disk_resource_id for 1 hour"
# shellcheck disable=SC2102
sas=$(az disk grant-access --ids $disk_resource_id --duration-in-seconds 3600 --query [accessSas] -o tsv)
if [[ "$sas" == "None" ]]; then
 echo "sas token empty. Trying alternative query string"
# shellcheck disable=SC2102
 sas=$(az disk grant-access --ids $disk_resource_id --duration-in-seconds 3600 --query [accessSAS] -o tsv)
fi

if [[ "$sas" == "None" ]]; then
 echo "sas token empty after trying both queries. Can't continue"
 exit 1
fi

capture_benchmark "${SCRIPT_NAME}_grant_access_to_disk"

echo "Uploading $disk_resource_id to ${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd"

echo "Setting azcopy environment variables with pool identity: $AZURE_MSI_RESOURCE_STRING"
# TBD: Need to investigate why `azcopy-preview login --login-type=MSI` does not work for Windows
export AZCOPY_AUTO_LOGIN_TYPE="MSI"
export AZCOPY_MSI_RESOURCE_STRING="$AZURE_MSI_RESOURCE_STRING"

if [[ "${OS_TYPE}" == "Linux" ]]; then
  export AZCOPY_CONCURRENCY_VALUE="AUTO"
  azcopy copy "${sas}" "${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd" --recursive=true || exit $?
else
  azcopy-preview copy "${sas}" "${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd" --recursive=true || exit $?
fi

echo "Uploaded $disk_resource_id to ${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd"
capture_benchmark "${SCRIPT_NAME}_upload_disk_to_blob"

az disk revoke-access --ids $disk_resource_id 

az resource delete --ids $disk_resource_id

echo "Deleted $disk_resource_id"
capture_benchmark "${SCRIPT_NAME}_revoke_access_and_delete_disk"

capture_benchmark "${SCRIPT_NAME}_overall" true
process_benchmarks