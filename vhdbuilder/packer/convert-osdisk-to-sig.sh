#!/bin/bash -e

required_env_vars=(
    "STORAGE_ACCOUNT_SUB_ID"
    "STORAGE_ACCOUNT_RESOURCE_GROUP"
    "STORAGE_ACCOUNT_NAME"
    "VHD_URL"
    "SKU_NAME"
    "IMAGE_VERSION"
)

for v in "${required_env_vars[@]}"
do
    if [ -z "${!v}" ]; then
        echo "$v was not set!"
        exit 1
    fi
done

STORAGE_ACCT_ID="/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${STORAGE_ACCOUNT_RESOURCE_GROUP}/providers/Microsoft.Storage/storageAccounts/${STORAGE_ACCOUNT_NAME}"
az account set --subscription f3b504bb-826e-46c7-a1b7-674a5a0ae43a
az sig image-version create --location eastus --resource-group aks-dev-global --gallery-name NodeImages \
     --gallery-image-definition ${SKU_NAME} --gallery-image-version ${IMAGE_VERSION}\
     --os-vhd-uri ${VHD_URL} --os-vhd-storage-account ${STORAGE_ACCT_ID}
