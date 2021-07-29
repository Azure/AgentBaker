
#!/bin/bash
set -x

[[ -z "${RAW_IMAGE_URL}" ]] && echo "RAW_IMAGE_URL is not set" && exit 1
[[ -z "${AZURE_RESOURCE_GROUP_NAME}" ]] && echo "AZURE_RESOURCE_GROUP_NAME is not set" && exit 1
[[ -z "${HYPERV_GENERATION}" ]] && echo "HYPERV_GENERATION is not set" && exit 1
[[ -z "${OS_TYPE}" ]] && echo "OS_TYPE is not set" && exit 1

CREATE_TIME="$(date +%s)"
IMPORTED_IMAGE_NAME="imported-$CREATE_TIME-$RANDOM"

echo "Creating new image for a custom VHD ${RAW_IMAGE_URL}"
az image create --resource-group ${AZURE_RESOURCE_GROUP_NAME} --name ${IMPORTED_IMAGE_NAME} --os-type ${OS_TYPE} --hyper-v-generation ${HYPERV_GENERATION} --source ${RAW_IMAGE_URL}
until [ ! -z "$managed_image_uri" ]; do
    echo "${log_prefix}: sleeping for 1m before getting managed image ${AZURE_RESOURCE_GROUP_NAME}/${IMPORTED_IMAGE_NAME}..."
    sleep 1m
    managed_image_uri=$(az image show --resource-group ${AZURE_RESOURCE_GROUP_NAME} --name ${IMPORTED_IMAGE_NAME} -o json | jq -r ".id")
done
echo "managed image ${managed_image_uri} found"
