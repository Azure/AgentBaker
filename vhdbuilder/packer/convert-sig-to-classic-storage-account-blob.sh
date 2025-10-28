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

if [ "${OS_TYPE,,}" = "windows" ]; then
  if [ -z "$LOCATION" ]; then
    echo "LOCATION must be set for windows builds"
    exit 1
  fi
fi

if [ "${OS_TYPE,,}" = "linux" ]; then
  if [ -z "$PACKER_BUILD_LOCATION" ]; then
    echo "PACKER_BUILD_LOCATION must be set for linux builds"
    exit 1
  fi
  LOCATION=$PACKER_BUILD_LOCATION
fi

# Default to this hard-coded value for Linux does not pass this environment variable into here
if [ -z "$SIG_GALLERY_NAME" ]; then
  SIG_GALLERY_NAME="PackerSigGalleryEastUS"
fi

echo "SIG_IMAGE_VERSION before checking and assigning is $SIG_IMAGE_VERSION"
# Windows Gen 2: use the passed environment variable $SIG_IMAGE_VERSION
# Linux Gen 2: assign $CAPTURED_SIG_VERSION to $SIG_IMAGE_VERSION
if [ -z "$SIG_IMAGE_VERSION" ]; then
  SIG_IMAGE_VERSION=${CAPTURED_SIG_VERSION}
fi

# Use $SIG_IMAGE_VERSION for the sig resource, and $CAPTURED_SIG_VERSION (a random number) for the disk resource
sig_resource_id="/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${RESOURCE_GROUP_NAME}/providers/Microsoft.Compute/galleries/${SIG_GALLERY_NAME}/images/${SIG_IMAGE_NAME}/versions/${SIG_IMAGE_VERSION}"
disk_resource_id="/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${RESOURCE_GROUP_NAME}/providers/Microsoft.Compute/disks/${CAPTURED_SIG_VERSION}"
capture_benchmark "${SCRIPT_NAME}_set_variables_for_converting_to_disk"

echo "Converting $sig_resource_id to $disk_resource_id"
if [ "${OS_TYPE}" = "Linux" ] && [ "${ENABLE_TRUSTED_LAUNCH}" = "True" ]; then
  az resource create --id $disk_resource_id  --api-version 2024-03-02 --is-full-object --location $LOCATION --properties "{\"location\": \"$LOCATION\", \
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
elif [ "${OS_TYPE}" = "Linux" ] && grep -q "cvm" <<< "$FEATURE_FLAGS"; then
  az resource create --id $disk_resource_id --api-version 2024-03-02 --is-full-object --location $LOCATION --properties "{\"location\": \"$LOCATION\", \
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
elif [ "${OS_TYPE}" = "Linux" ] && grep -q "GB200" <<< "$FEATURE_FLAGS"; then
  echo "GB200: Creating standard disk from SIG image"
  # GB200 uses standard disk creation for now, but can be customized in the future if needed
  az resource create --id $disk_resource_id  --api-version 2024-03-02 --is-full-object --location $LOCATION --properties "{\"location\": \"$LOCATION\", \
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
else
  az resource create --id $disk_resource_id  --api-version 2024-03-02 --is-full-object --location $LOCATION --properties "{\"location\": \"$LOCATION\", \
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
if [ "$sas" = "None" ]; then
 echo "sas token empty. Trying alternative query string"
# shellcheck disable=SC2102
 sas=$(az disk grant-access --ids $disk_resource_id --duration-in-seconds 3600 --query [accessSAS] -o tsv)
fi

if [ "$sas" = "None" ]; then
 echo "sas token empty after trying both queries. Can't continue"
 exit 1
fi
capture_benchmark "${SCRIPT_NAME}_grant_access_to_disk"

echo "Uploading $disk_resource_id to ${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd"

echo "Setting azcopy environment variables with pool identity: $AZURE_MSI_RESOURCE_STRING"
export AZCOPY_AUTO_LOGIN_TYPE="MSI"
export AZCOPY_MSI_RESOURCE_STRING="$AZURE_MSI_RESOURCE_STRING"
export AZCOPY_CONCURRENCY_VALUE="AUTO"
export AZCOPY_LOG_LOCATION="$(pwd)/azcopy-log-files/"
export AZCOPY_JOB_PLAN_LOCATION="$(pwd)/azcopy-job-plan-files/"
mkdir -p "${AZCOPY_LOG_LOCATION}"
mkdir -p "${AZCOPY_JOB_PLAN_LOCATION}"

if ! azcopy copy "${sas}" "${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd" --recursive=true ; then
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
  exit $azExitCode
fi

echo "Uploaded $disk_resource_id to ${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd"
capture_benchmark "${SCRIPT_NAME}_upload_disk_to_blob"

if ! az disk revoke-access --ids $disk_resource_id; then
  echo "##vso[task.logissue type=warning]unable to revoke access to $disk_resource_id"
fi

if ! az resource delete --ids $disk_resource_id; then
  echo "##vso[task.logissue type=warning]unable to delete $disk_resource_id"
else
  echo "deleted $disk_resource_id"
fi

capture_benchmark "${SCRIPT_NAME}_revoke_access_and_delete_disk"

capture_benchmark "${SCRIPT_NAME}_overall" true
process_benchmarks