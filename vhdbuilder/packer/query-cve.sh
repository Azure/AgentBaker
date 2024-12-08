#!/bin/bash
set -euxo pipefail

QUERY_RESOURCE_PREFIX="cve-query"
QUERY_VM_NAME="$QUERY_RESOURCE_PREFIX-vm-$(date +%s)-$RANDOM"
RESOURCE_GROUP_NAME="$QUERY_RESOURCE_PREFIX-$(date +%s)-$RANDOM"

QUERY_VM_USERNAME="azureuser"

QUERY_SCRIPT_PATH="query-cve-vm.sh"

set +x
QUERY_VM_PASSWORD="QueryVM@$(date +%s)"
set -x

MODULE_NAME="vuln-to-kusto-vhd"
MODULE_VERSION="v0.0.2-ac18d186094"
GO_ARCH="amd64"

function cleanup() {
    echo "Deleting resource group ${RESOURCE_GROUP_NAME}"
    az group delete --name $RESOURCE_GROUP_NAME --yes --no-wait
}
trap cleanup EXIT

az group create --name $RESOURCE_GROUP_NAME --location ${QUERY_VM_LOCATION}

az vm create --resource-group $RESOURCE_GROUP_NAME \
  --name $QUERY_VM_NAME \
  --image $QUERY_VHD_IMAGE \
  --admin-username $QUERY_VM_USERNAME \
  --admin-password $QUERY_VM_PASSWORD \
  --assign-identity "${UMSI_RESOURCE_ID}"

FULL_PATH=$(realpath $0)
CDIR=$(dirname $FULL_PATH)
QUERY_SCRIPT_PATH="$CDIR/$QUERY_SCRIPT_PATH"

# storage account specific variables

# Use the domain name from the classic blob URL to get the storage account name.
# If the CLASSIC_BLOB var is not set create a new var called BLOB_STORAGE_NAME in the pipeline.
BLOB_URL_REGEX="^https:\/\/.+\.blob\.core\.windows\.net\/vhd(s)?$"
if [[ $CLASSIC_BLOB =~ $BLOB_URL_REGEX ]]; then
    STORAGE_ACCOUNT_NAME=$(echo $CLASSIC_BLOB | sed -E 's|https://(.*)\.blob\.core\.windows\.net(:443)?/(.*)?|\1|')
else
    # Used in the 'AKS Linux VHD Build - PR check-in gate' pipeline.
    if [ -z "$BLOB_STORAGE_NAME" ]; then
        echo "BLOB_STORAGE_NAME is not set, please either set the CLASSIC_BLOB var or create a new var BLOB_STORAGE_NAME in the pipeline."
        exit 1
    fi
    STORAGE_ACCOUNT_NAME=${BLOB_STORAGE_NAME}
fi

# for scanning storage account/container upload access
az vm identity assign -g $RESOURCE_GROUP_NAME --name $QUERY_VM_NAME --identities $AZURE_MSI_RESOURCE_STRING

TIMESTAMP=$(date +%s%3N)
CVE_REPORT_OUTPUT_NAME="cve-report-${BUILD_SOURCEVERSION}-${TIMESTAMP}.out"
CVE_REPORT_CONTAINER_NAME="vhd-scans"

ret=$(az vm run-command invoke \
  --command-id RunShellScript \
  --name $QUERY_VM_NAME \
  --resource-group $RESOURCE_GROUP_NAME \
  --scripts @$QUERY_SCRIPT_PATH\
  --parameters "UMSI_PRINCIPAL_ID"=${UMSI_PRINCIPAL_ID} \
      "UMSI_CLIENT_ID"=${UMSI_CLIENT_ID} \
      "ACCOUNT_NAME"=${ACCOUNT_NAME} \
      "KUSTO_ENDPOINT"=${KUSTO_ENDPOINT} \
      "KUSTO_DATABASE"=${KUSTO_DATABASE} \
      "KUSTO_TABLE"=${KUSTO_TABLE} \
      "COMMIT_HASH"=${BUILD_SOURCEVERSION} \
      "STORAGE_ACCOUNT_NAME"=${STORAGE_ACCOUNT_NAME} \
      "CVE_REPORT_OUTPUT_NAME"=${CVE_REPORT_OUTPUT_NAME} \
      "CVE_REPORT_CONTAINER_NAME"=${CVE_REPORT_CONTAINER_NAME} \
      "AZURE_MSI_RESOURCE_STRING"=${AZURE_MSI_RESOURCE_STRING})

errMsg=$(echo -e $(echo $ret | jq ".value[] | .message" | grep -oP '(?<=stderr]).*(?=\\n")'))
if [[ $errMsg != '' ]]; then
  az storage blob download --account-name $STORAGE_ACCOUNT_NAME --container-name $CVE_REPORT_CONTAINER_NAME --name $CVE_REPORT_OUTPUT_NAME --file cve-report.out --auth-mode login
  az storage blob delete --account-name $STORAGE_ACCOUNT_NAME --container-name $CVE_REPORT_CONTAINER_NAME --name $CVE_REPORT_OUTPUT_NAME --auth-mode login
  exit 1
fi
