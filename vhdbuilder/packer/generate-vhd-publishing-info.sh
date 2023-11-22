#!/bin/bash -e

required_env_vars=(
    "CLASSIC_SA_CONNECTION_STRING"
    "STORAGE_ACCT_BLOB_URL"
    "VHD_NAME"
    "OS_NAME"
    "OFFER_NAME"
    "SKU_NAME"
    "HYPERV_GENERATION"
    "IMAGE_VERSION"
)

for v in "${required_env_vars[@]}"
do
    if [ -z "${!v}" ]; then
        if [ "$v" == "IMAGE_VERSION" ]; then
           IMAGE_VERSION=$(date +%Y%m.%d.0)
           echo "$v was not set, set it to ${!v}"
        else
            echo "$v was not set!"
            exit 1
        fi
    fi
done

# SIG image definition for AMD64/ARM64 has subtle difference, otherwise a SIG version cannot be used to create VM/VMSS of corresponding sku.
# 'az sig image-definition create' will have a new property (--architecture Arm64|x64) for this soon. We need this in the publishing-info
# in order that the VHD publish EV2 pipeline can create image-definition with right architecture.
if [[ ${ARCHITECTURE,,} == "arm64" ]]; then
    IMAGE_ARCH="Arm64"
else
    IMAGE_ARCH="x64"
fi

echo "generating traditional SAS token with CLASSIC_SA_CONNECTION_STRING..."
start_date=$(date +"%Y-%m-%dT00:00Z" -d "-1 day")
expiry_date=$(date +"%Y-%m-%dT00:00Z" -d "+1 year")
if [[ "${OS_NAME,,}" != "windows" ]]; then
    [ -z "${OUTPUT_STORAGE_CONTAINER_NAME}" ] && echo "OUTPUT_STORAGE_CONTAINER_NAME should be set..." && exit 1
    echo "storage container name: ${OUTPUT_STORAGE_CONTAINER_NAME}"
    # max of 7 day expiration time when using user delegation SAS
    sas_token=$(az storage container generate-sas --name ${OUTPUT_STORAGE_CONTAINER_NAME} --permissions lr --connection-string ${CLASSIC_SA_CONNECTION_STRING} --start ${start_date} --expiry ${expiry_date} | tr -d '"')
else
    # we still need to use the original connection string when not using a system-assigned identity on 1ES pools
    sas_token=$(az storage container generate-sas --name vhds --permissions lr --connection-string ${CLASSIC_SA_CONNECTION_STRING} --start ${start_date} --expiry ${expiry_date} | tr -d '"')
fi

if [ "$sas_token" == "" ]; then
    echo "sas_token is empty"
    exit 1
fi
vhd_url="${STORAGE_ACCT_BLOB_URL}/${VHD_NAME}?$sas_token"

echo "Testing whether the generated sas token works"
vhd_size=$(curl -sI $vhd_url | grep -i Content-Length | awk '{print $2}')
if [ "$vhd_size" == "" ]; then
    echo "The genrated sas token does not work"
    exit 1
fi
echo "The generated sas token works"

# Do not log sas token
echo "COPY ME ---> ${STORAGE_ACCT_BLOB_URL}/${VHD_NAME}?***"

# Note: The offer_name is the value from OS_SKU (eg. Ubuntu)
cat <<EOF > vhd-publishing-info.json
{
    "vhd_url" : "$vhd_url",
    "os_name" : "$OS_NAME",
    "sku_name" : "$SKU_NAME",
    "offer_name" : "$OFFER_NAME",
    "hyperv_generation": "${HYPERV_GENERATION}",
    "image_architecture": "${IMAGE_ARCH}",
    "image_version": "${IMAGE_VERSION}"
}
EOF

# Do not log sas token
sed 's/?.*\",/?***\",/g' < vhd-publishing-info.json
