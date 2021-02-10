#!/bin/bash -x

required_env_vars=(
  "CLIENT_ID"
  "CLIENT_SECRET"
  "TENANT_ID"
  "SUBSCRIPTION_ID"
  "PKR_RG_NAME"
  "MODE"
  "AZURE_RESOURCE_GROUP_NAME"
)

for v in "${required_env_vars[@]}"; do
  if [ -z "${!v}" ]; then
    echo "$v was not set!"
    exit 1
  fi
done

#clean up the temporary storage account
id=$(az storage account show -n ${SA_NAME} -g ${AZURE_RESOURCE_GROUP_NAME} | jq .id)
if [ ! -z "$id" ]; then
  az storage account delete -n ${SA_NAME} -g ${AZURE_RESOURCE_GROUP_NAME} --yes
fi

#clean up managed image
if [[ "$MODE" != "default" ]]; then
  id=$(az image show -n ${IMAGE_NAME} -g ${AZURE_RESOURCE_GROUP_NAME} | jq .id)
  if [ ! -z "$id" ]; then
    az image delete -n ${IMAGE_NAME} -g ${AZURE_RESOURCE_GROUP_NAME}
  fi
fi

#clean up the packer generated resource group
id=$(az group show --name ${PKR_RG_NAME} | jq .id)
if [ ! -z "$id" ]; then
  echo "Deleting packer resource group ${PKR_RG_NAME}"
  az group delete --name ${PKR_RG_NAME} --yes
fi
