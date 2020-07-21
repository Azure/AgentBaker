#!/bin/bash

# Create the necessary infrastructure if it doesn't exist, so that we can publish the SIG image version
[[ -z "${CLIENT_ID}" ]] && (echo "CLIENT_ID is not set"; exit 1)
[[ -z "${CLIENT_SECRET}" ]] && (echo "CLIENT_SECRET is not set"; exit 1)
[[ -z "${TENANT_ID}" ]] && (echo "TENANT_ID is not set"; exit 1)
[[ -z "${SUBSCRIPTION_ID}" ]] && (echo "SUBSCRIPTION_ID is not set"; exit 1)
[[ -z "${REGION}" ]] && (echo "REGION is not set"; exit 1)

[[ -z "${RG_NAME}" ]] && (echo "RG_NAME is not set"; exit 1)
[[ -z "${OS_NAME}" ]] && (echo "OS_NAME is not set"; exit 1)
[[ -z "${GALLERY_NAME}" ]] && (echo "GALLERY_NAME is not set"; exit 1)
[[ -z "${IMAGEDEFINITION_NAME}" ]] && (echo "IMAGEDEFINITION_NAME is not set"; exit 1)
[[ -z "${HYPERV_GENERATION}" ]] && (echo "HYPERV_GENERATION is not set"; exit 1)

echo "az login --service-principal -u ${CLIENT_ID} -p *** --tenant ${TENANT_ID}"
az login --service-principal -u ${CLIENT_ID} -p ${CLIENT_SECRET} --tenant ${TENANT_ID}

echo "az account set --subscription ${SUBSCRIPTION_ID}"
az account set --subscription ${SUBSCRIPTION_ID}

 echo "Check if Resource Group exists, if not create it"
 id=$(az group show --name ${RG_NAME})
 if [ -z "$id" ] ; then
   echo "Creating resource group ${RG_NAME} in ${REGION} region"
   az group create --name ${RG_NAME} --location ${REGION}
 fi

 echo "Check if Shared Image Gallery exists, if not create it"
 id=$(az sig show --resource-group ${RG_NAME} --gallery-name ${GALLERY_NAME})
 if [ -z "$id" ]; then
   echo "Creating Shared Image Gallery ${GALLERY_NAME} in the resource group ${RG_NAME}"
   az sig create --resource-group ${RG_NAME} --gallery-name ${GALLERY_NAME} --location ${REGION}
 fi

 echo "Check if SIG imagedefnition exists, if not create it"
 id=$(az sig image-definition show \
   --resource-group ${RG_NAME} \
   --gallery-name ${GALLERY_NAME} \
   --gallery-image-definition ${IMAGEDEFINITION_NAME})
 if [ -z "$id" ]; then
   echo "Creating image definition ${IMAGEDEFINITION_NAME} generation ${HYPERV_GENERATION} in Shared Image Gallery ${GALLERY_NAME} inside the resource group ${RG_NAME}"
   az sig image-definition create \
     --resource-group ${RG_NAME} \
     --gallery-name ${GALLERY_NAME} \
     --gallery-image-definition ${IMAGEDEFINITION_NAME} \
     --publisher microsoft-aks \
     --offer ${GALLERY_NAME} \
     --sku ${IMAGEDEFINITION_NAME} \
     --os-type ${OS_NAME} \
     --location ${REGION} \
     --hyper-v-generation ${HYPERV_GENERATION}
 fi

echo "##vso[task.setvariable variable=RG_NAME;]$RG_NAME"
echo "##vso[task.setvariable variable=GALLERY_NAME;]$GALLERY_NAME"
echo "##vso[task.setvariable variable=IMAGEDEFINITION_NAME;]$IMAGEDEFINITION_NAME"

echo "##vso[task.setvariable variable=OS_NAME;]$OS_NAME"
