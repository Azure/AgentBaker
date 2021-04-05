#!/bin/bash -e

required_env_vars=(
    "IMPORT_IMAGE_URL"
    "IMPORT_IMAGE_SAS"
    "HYPERV_GENERATION"
    "OS_VERSION"
    "OS_SKU"
)

SETTINGS_JSON="${SETTINGS_JSON:-./vhdbuilder/packer/settings.json}"

for v in "${required_env_vars[@]}"
do
    if [ -z "${!v}" ]; then
        echo "$v was not set!"
        exit 1
    fi
done

if [[ $HYPERV_GENERATION == "V2" ]]; then
    IMPORT_IMAGE_URL=${IMPORT_IMAGE_URL//-gen1-/-gen2-}
fi
if [[ $HYPERV_GENERATION == "V1" ]]; then
    IMPORT_IMAGE_URL=${IMPORT_IMAGE_URL//-gen2-/-gen1-}
fi

STORAGE_ACCOUNT_NAME=$(jq -r .storage_account_name ${SETTINGS_JSON})
AZURE_RESOURCE_GROUP_NAME=$(jq -r .resource_group_name ${SETTINGS_JSON})
AZURE_LOCATION=$(jq -r .location ${SETTINGS_JSON})
SIG_GALLERY_NAME=$(jq -r .sig_gallery_name ${SETTINGS_JSON})
CREATE_TIME=$(jq -r .create_time ${SETTINGS_JSON})

expiry_date=$(date -u -d "10 minutes" '+%Y-%m-%dT%H:%MZ')
sas_token=$(az storage account generate-sas --account-name $STORAGE_ACCOUNT_NAME --permissions rcw --resource-types o --services b --expiry ${expiry_date} | tr -d '"')

IMPORT_NAME=imported-$CREATE_TIME-$RANDOM
IMPORTED_IMAGE_URL="https://${STORAGE_ACCOUNT_NAME}.blob.core.windows.net/system/$IMPORT_NAME.vhd"
DESTINATION_WITH_SAS="${IMPORTED_IMAGE_URL}?${sas_token}"

echo Importing VHD from $IMPORT_IMAGE_URL
azcopy-preview copy $IMPORT_IMAGE_URL$IMPORT_IMAGE_SAS $DESTINATION_WITH_SAS

cp $SETTINGS_JSON $SETTINGS_JSON.tmp
jq --arg imgurl "$IMPORTED_IMAGE_URL" '.imported_image_url = $imgurl' $SETTINGS_JSON.tmp > $SETTINGS_JSON
rm $SETTINGS_JSON.tmp

if [[ $HYPERV_GENERATION == "V2" ]]; then
    az image create \
        --resource-group $AZURE_RESOURCE_GROUP_NAME \
        --name $IMPORT_NAME \
        --source $IMPORTED_IMAGE_URL \
        --hyper-v-generation V2 \
        --os-type Linux

    az sig image-definition create \
        --resource-group $AZURE_RESOURCE_GROUP_NAME \
        --gallery-name $SIG_GALLERY_NAME \
        --gallery-image-definition $IMPORT_NAME \
        --location $AZURE_LOCATION \
        --os-type Linux \
        --publisher microsoft-aks \
        --offer $IMPORT_NAME \
        --sku $OS_SKU \
        --hyper-v-generation V2 \
        --os-state generalized \
        --description "Imported image for AKS Packer build"

    az sig image-version create \
        --location $AZURE_LOCATION \
        --resource-group $AZURE_RESOURCE_GROUP_NAME \
        --gallery-name $SIG_GALLERY_NAME \
        --gallery-image-definition $IMPORT_NAME \
        --gallery-image-version 1.0.0 \
        --managed-image $IMPORT_NAME

    cp $SETTINGS_JSON $SETTINGS_JSON.tmp
    jq --arg imgname "$IMPORT_NAME" '.imported_sig_image = $imgname' $SETTINGS_JSON.tmp > $SETTINGS_JSON
    rm $SETTINGS_JSON.tmp
fi