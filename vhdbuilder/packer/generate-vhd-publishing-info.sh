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
           IMAGE_VERSION=$(date +%Y.%m.%d)
           echo "$v was not set, set it to ${!v}"
        else
            echo "$v was not set!"
            exit 1
        fi
    fi
done

start_date=$(date +"%Y-%m-%dT00:00Z" -d "-1 day")
expiry_date=$(date +"%Y-%m-%dT00:00Z" -d "+1 year")
sas_token=$(az storage container generate-sas --name vhds --permissions lr --connection-string ${CLASSIC_SA_CONNECTION_STRING} --start ${start_date} --expiry ${expiry_date} | tr -d '"')
vhd_url="${STORAGE_ACCT_BLOB_URL}/${VHD_NAME}?$sas_token"

echo "COPY ME ---> ${vhd_url}"
sku_name=$(echo $SKU_NAME | tr -d '.')

# Note: The offer_name is the value from OS_SKU (eg. Ubuntu)
cat <<EOF > vhd-publishing-info.json
{
    "vhd_url" : "$vhd_url",
    "os_name" : "$OS_NAME",
    "sku_name" : "$sku_name",
    "offer_name" : "$OFFER_NAME",
    "hyperv_generation": "${HYPERV_GENERATION}",
    "image_version": "${IMAGE_VERSION}"
}
EOF


cat vhd-publishing-info.json