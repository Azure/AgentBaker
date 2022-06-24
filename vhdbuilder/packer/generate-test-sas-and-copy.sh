#!/bin/bash -e

TEST_CREATE_TIME="$(date +%s)"
TEST_STORAGE_ACCOUNT_NAME="aksimages${TEST_CREATE_TIME}$RANDOM"
TEST_EXPIRY_DATE=$(date -u -d "180 minutes" '+%Y-%m-%dT%H:%MZ')
TEST_SAS_TOKEN=$(az storage account generate-sas --account-name $TEST_STORAGE_ACCOUNT_NAME --permissions r --resource-types o --services b --expiry ${TEST_EXPIRY_DATE} | tr -d '"')
azcopy-preview copy "${OS_DISK_SAS}" "${CLASSIC_BLOB}${TEST_SAS_TOKEN}" --recursive=true