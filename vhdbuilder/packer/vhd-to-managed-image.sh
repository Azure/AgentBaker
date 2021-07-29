
#!/bin/bash
set -x

[[ -z "${RAW_IMAGE_BLOB_URL}" ]] && echo "RAW_IMAGE_BLOB_URL is not set" && exit 1
[[ -z "${AZURE_RESOURCE_GROUP_NAME}" ]] && echo "AZURE_RESOURCE_GROUP_NAME is not set" && exit 1
[[ -z "${HYPERV_GENERATION}" ]] && echo "HYPERV_GENERATION is not set" && exit 1
[[ -z "${OS_TYPE}" ]] && echo "OS_TYPE is not set" && exit 1

# azure disk source is expected in the same region as the image, so we use azcopy-preview we have the
# correct input to create a managed image
CREATE_TIME="$(date +%s)"
IMPORTED_IMAGE_NAME="imported-$CREATE_TIME-$RANDOM"
expiry_date=$(date -u -d "10 minutes" '+%Y-%m-%dT%H:%MZ')
sas_token=$(az storage account generate-sas --account-name $STORAGE_ACCOUNT_NAME --permissions rcw --resource-types o --services b --expiry ${expiry_date} | tr -d '"')
IMPORTED_IMAGE_URL="https://${STORAGE_ACCOUNT_NAME}.blob.core.windows.net/system/$IMPORTED_IMAGE_NAME.vhd"
DESTINATION_WITH_SAS="${IMPORTED_IMAGE_URL}?${sas_token}"
azcopy-preview copy $RAW_IMAGE_BLOB_URL $DESTINATION_WITH_SAS

echo "Creating new image for imported vhd ${IMPORTED_IMAGE_URL}"
az image create \
    --resource-group $AZURE_RESOURCE_GROUP_NAME \
    --name $IMPORTED_IMAGE_NAME \
    --source $IMPORTED_IMAGE_URL \
    --hyper-v-generation ${HYPERV_GENERATION} \
    --os-type ${OS_TYPE}

until [ ! -z "$managed_image_uri" ]; do
    echo "sleeping for 1m before getting managed image ${AZURE_RESOURCE_GROUP_NAME}/${IMPORTED_IMAGE_NAME}..."
    sleep 1m
    managed_image_uri=$(az image show --resource-group ${AZURE_RESOURCE_GROUP_NAME} --name ${IMPORTED_IMAGE_NAME} -o json | jq -r ".id")
done
echo "managed image ${managed_image_uri} found"
