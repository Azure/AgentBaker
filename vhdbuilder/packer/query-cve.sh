#!/bin/bash
set -eux

QUERY_RESOURCE_PREFIX="cve-query"
QUERY_VM_NAME="$QUERY_RESOURCE_PREFIX-vm-$(date +%s)-$RANDOM"
VHD_IMAGE="/subscriptions/c4c3550e-a965-4993-a50c-628fd38cd3e1/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/2204gen2arm64containerd" # to variable
RESOURCE_GROUP_NAME="$QUERY_RESOURCE_PREFIX-$(date +%s)-$RANDOM"

QUERY_VM_USERNAME="azureuser"
VM_LOCATION="westus2" # to variable

QUERY_SCRIPT_PATH="query-cve-vm.sh"

set +x
QUERY_VM_PASSWORD="QueryVM@$(date +%s)"
set -x

function cleanup() {
    echo "Deleting resource group ${RESOURCE_GROUP_NAME}"
    az group delete --name $RESOURCE_GROUP_NAME --yes --no-wait
}
trap cleanup EXIT

az group create --name $RESOURCE_GROUP_NAME --location ${VM_LOCATION}

az vm create --resource-group $RESOURCE_GROUP_NAME \
  --name $QUERY_VM_NAME \
  --image $VHD_IMAGE \
  --admin-username $QUERY_VM_USERNAME \
  --admin-password $QUERY_VM_PASSWORD \
  --assign-identity "${UMSI_RESOURCE_ID}"

FULL_PATH=$(realpath $0)
CDIR=$(dirname $FULL_PATH)
QUERY_SCRIPT_PATH="$CDIR/$QUERY_SCRIPT_PATH"

az vm run-command invoke \
  --command-id RunShellScript \
  --name $QUERY_VM_NAME \
  --resource-group $RESOURCE_GROUP_NAME \
  --scripts @$QUERY_SCRIPT_PATH\
  --parameters "UMSI_PRINCIPAL_ID"=${UMSI_PRINCIPAL_ID} \
      "ACCOUNT_NAME"=${ACCOUNT_NAME}
