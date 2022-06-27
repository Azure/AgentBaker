#!/bin/bash -e

TEST_CREATE_TIME="$(date +%s)"
TEST_STORAGE_ACCOUNT_NAME="aksimages${TEST_CREATE_TIME}$RANDOM"

# Create a test storage account (TO-DO: this will not be cleaned up in cleanup)
avail=$(az storage account check-name -n ${TEST_STORAGE_ACCOUNT_NAME} -o json | jq -r .nameAvailable)
if $avail ; then
	echo "creating new storage account in azcopy ${TEST_STORAGE_ACCOUNT_NAME}"
    echo "Azure RG is ${AZURE_RESOURCE_GROUP_NAME}"
	az storage account create -n $TEST_STORAGE_ACCOUNT_NAME -g $AZURE_RESOURCE_GROUP_NAME --sku "Standard_RAGRS" --tags "now=${TEST_CREATE_TIME}" --location ${AZURE_LOCATION}
	echo "creating new container system"
	key=$(az storage account keys list -n $TEST_STORAGE_ACCOUNT_NAME -g $AZURE_RESOURCE_GROUP_NAME | jq -r '.[0].value')
	az storage container create --name system --account-key=$key --account-name=$TEST_STORAGE_ACCOUNT_NAME
else
	echo "storage account ${TEST_STORAGE_ACCOUNT_NAME} already exists."
fi

echo "test storage name: ${TEST_STORAGE_ACCOUNT_NAME}"

TEST_EXPIRY_DATE=$(date -u -d "180 minutes" '+%Y-%m-%dT%H:%MZ')

TEST_SAS_TOKEN=$(az storage account generate-sas --account-name $TEST_STORAGE_ACCOUNT_NAME --permissions r --account-key $key --resource-types o --services b --expiry ${TEST_EXPIRY_DATE} | tr -d '"')

TEST_IMAGE_NAME="windows-${TEST_CREATE_TIME}-${RANDOM}"

echo "Test image name ${TEST_IMAGE_NAME}"

TEST_IMAGE_URL="https://${TEST_STORAGE_ACCOUNT_NAME}.blob.core.windows.net/system/${TEST_IMAGE_NAME}.vhd"
azcopy-preview copy "${OS_DISK_SAS}" "${TEST_IMAGE_URL}${TEST_SAS_TOKEN}" --recursive=true
