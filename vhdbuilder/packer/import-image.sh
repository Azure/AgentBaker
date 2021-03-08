#!/bin/bash -e

required_env_vars=(
    "IMPORT_IMAGE_URL"
    "IMPORT_IMAGE_SAS"
)

SETTINGS_JSON="${SETTINGS_JSON:-./vhdbuilder/packer/settings.json}"

for v in "${required_env_vars[@]}"
do
    if [ -z "${!v}" ]; then
        echo "$v was not set!"
        exit 1
    fi
done

STORAGE_ACCOUNT_NAME=$(jq -r .storage_account_name ${SETTINGS_JSON})

expiry_date=$(date -u -d "10 minutes" '+%Y-%m-%dT%H:%MZ')
sas_token=$(az storage account generate-sas --account-name $STORAGE_ACCOUNT_NAME --permissions rcw --resource-types o --services b --expiry ${expiry_date} | tr -d '"')
destination_url="https://${STORAGE_ACCOUNT_NAME}.blob.core.windows.net/system/imported.vhd?${sas_token}"

echo Importing VHD from $IMPORT_IMAGE_URL
azcopy-preview copy $IMPORT_IMAGE_URL$IMPORT_IMAGE_SAS $destination_url
