#!/bin/bash
[[ -z "${MANAGED_IMAGE_NAME}" ]] && (echo "MANAGED_IMAGE_NAME is not set"; exit 1)
[[ -z "${MANAGED_IMAGE_RG_NAME}" ]] && (echo "MANAGED_IMAGE_RG_NAME is not set"; exit 1)

echo "Deleting managed image ${MANAGED_IMAGE_NAME}, from resource group ${MANAGED_IMAGE_RG_NAME}"
az image delete \
   --resource-group ${MANAGED_IMAGE_RG_NAME} \
   --name ${MANAGED_IMAGE_NAME}