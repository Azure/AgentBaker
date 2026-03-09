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

az storage blob copy start \
  --destination-blob "${BLOB_NAME}" \
  --destination-container "${VHD_STAGING_CONTAINER_NAME}" \
  --account-name "${TME_STORAGE_ACCOUNT_NAME}" \
  --source-uri "${CLASSIC_BLOB}/${TEST_VERSION}?${SAS_TOKEN}" \
  --auth-mode login
