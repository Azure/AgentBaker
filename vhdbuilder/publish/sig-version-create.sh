#!/bin/bash

# Create managed Image from VHD sas url and then publish the managed image into the already created shared image gallery

[[ -z "${REGION}" ]] && echo "REGION is not set" && exit 1
[[ -z "${RG_NAME}" ]] && echo "RG_NAME is not set" && exit 1
[[ -z "${GALLERY_NAME}" ]] && echo "GALLERY_NAME is not set" && exit 1
[[ -z "${IMAGEDEFINITION_NAME}" ]] && echo "IMAGEDEFINITION_NAME is not set" && exit 1
[[ -z "${IMAGE_VERSION}" ]] && echo "IMAGE_VERSION is not set" && exit 1
#TARGET_REGIONS must be set in the following format region=replicacount "westus2=1 eastus=4 uksouth=3"
[[ -z "${TARGET_CREATE_REGIONS}" ]] && echo "TARGET_CREATE_REGIONS is not set" && exit 1
[[ -z "${MANAGED_IMAGE_RG_NAME}" ]] && echo "MANAGED_IMAGE_RG_NAME is not set" && exit 1
[[ -z "${VHD_SOURCE}" ]] && echo "VHD_SOURCE is not set" && exit 1
[[ -z "${OS_NAME}" ]] && echo "OS_NAME is not set" && exit 1
[[ -z "${HYPERV_GENERATION}" ]] && echo "HYPERV_GENERATION is not set" && exit 1

MANAGED_IMAGE_NAME="MI_${REGION}_${GALLERY_NAME}_${IMAGEDEFINITION_NAME}_${IMAGE_VERSION}"

echo "Creating managed image ${MANAGED_IMAGE_NAME} from VHD, inside resource group ${MANAGED_IMAGE_RG_NAME}"
create_managed_image_command="az image create --resource-group ${MANAGED_IMAGE_RG_NAME} --name ${MANAGED_IMAGE_NAME} --os-type ${OS_NAME} --hyper-v-generation ${HYPERV_GENERATION} --source ${VHD_SOURCE}"
eval $create_managed_image_command

echo "Get managed image URI for the managed image ${MANAGED_IMAGE_NAME}"
sleep 1m
MANAGED_IMAGE_URI=$(az image show --resource-group ${MANAGED_IMAGE_RG_NAME} --name ${MANAGED_IMAGE_NAME} -o json | jq -r ".id")

echo "publishing managed image to /resourcegroup/${RG_NAME}/galleries/${GALLERY_NAME}/images/${IMAGEDEFINITION_NAME}/versions/${IMAGE_VERSION} with 5 replcia count in ${TARGET_CREATE_REGIONS}"
 
 az sig image-version create \
   --resource-group ${RG_NAME} \
   --gallery-name ${GALLERY_NAME} \
   --gallery-image-definition ${IMAGEDEFINITION_NAME} \
   --gallery-image-version ${IMAGE_VERSION} \
   --managed-image "${MANAGED_IMAGE_URI}" \
   --target-regions ${TARGET_CREATE_REGIONS} \
   --storage-account-type Premium_LRS \
   --replica-count 5


echo "##vso[task.setvariable variable=MANAGED_IMAGE_NAME;]$MANAGED_IMAGE_NAME"