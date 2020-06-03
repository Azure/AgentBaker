#!/bin/bash

# Create managed Image from VHD sas url and then publish the managed image into the already created shared image gallery

[[ -z "${RG_NAME}" ]] && (echo "RG_NAME is not set"; exit 1)
[[ -z "${GALLERY_NAME}" ]] && (echo "GALLERY_NAME is not set"; exit 1)
[[ -z "${IMAGEDEFINITION_NAME}" ]] && (echo "IMAGEDEFINITION_NAME is not set"; exit 1)
[[ -z "${IMAGE_VERSION}" ]] && (echo "IMAGE_VERSION is not set"; exit 1)
#TARGET_REGIONS must be set in the following format region=replicacount "westus2=1 eastus=4 uksouth=3"
[[ -z "${TARGET_REGIONS}" ]] && (echo "TARGET_REGIONS is not set"; exit 1)

echo "scaling the image version /resourcegroup/${RG_NAME}/galleries/${GALLERY_NAME}/images/${IMAGEDEFINITION_NAME}/versions/${IMAGE_VERSION} as follows ${TARGET_REGIONS}"

 az sig image-version update \
   --resource-group ${RG_NAME} \
   --gallery-name ${GALLERY_NAME} \
   --gallery-image-definition ${IMAGEDEFINITION_NAME} \
   --gallery-image-version ${IMAGE_VERSION} \
   --target-regions ${TARGET_REGIONS} \
