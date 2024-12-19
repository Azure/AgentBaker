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
      "COMMIT_HASH"=${BUILD_SOURCEVERSION})

errMsg=$(echo -e $(echo $ret | jq ".value[] | .message" | grep -oP '(?<=stderr]).*(?=\\n")'))
if [[ $errMsg != '' ]]; then
  exit 1
fi
