#!/bin/bash -e

required_env_vars=(
    "SOURCE_VHD_URL"
    "OFFER_NAME"
    "SKU_NAME"
    "HYPERV_GEN"
    "IMAGE_VERSION"
)

for v in "${required_env_vars[@]}"
do
    if [ -z "${!v}" ]; then
        echo "$v was not set!"
        exit 1
    fi
done

STORAGE_ACCOUNT_NAME="vhdbuildercorpwestus2"
CONTAINER_NAME="vhds"
BLOB_NAME="$OFFER_NAME.$SKU_NAME.$HYPERV_GEN.$IMAGE_VERSION"

az storage blob copy start --account-name $STORAGE_ACCOUNT_NAME --destination-container $CONTAINER_NAME --destination-blob $BLOB_NAME --source-uri $SOURCE_VHD_URL
sleep 5m

while : ; do
  status=$(az storage blob show --account-name $STORAGE_ACCOUNT_NAME --container-name vhds --name osdisk2.vhd | jq -r '.properties.copy.status')
  if [[ $status != "pending" ]]; then
    break
  fi
  echo "Copy pending. Waiting 2 mins."
  sleep 2m
done
