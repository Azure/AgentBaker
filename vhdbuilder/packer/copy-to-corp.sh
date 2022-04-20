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
endtime=`date -u -d "60 minutes" '+%Y-%m-%dT%H:%MZ'`
sas_token=`az storage container generate-sas --account-name $STORAGE_ACCOUNT_NAME --name $CONTAINER_NAME --permissions dlrw --expiry $endtime --auth-mode login --as-user -o tsv`

BLOB_NAME="$OFFER_NAME.$SKU_NAME.$HYPERV_GEN.$IMAGE_VERSION"
target_uri="https://${STORAGE_ACCOUNT_NAME}.blob.core.windows.net/${CONTAINER_NAME}/${BLOB_NAME}.vhd?$sas_token"

echo $SOURCE_VHD_URL $target_uri
azcopy copy ${SOURCE_VHD_URL} "${target_uri}"
