#!/bin/bash -x

if [[ -z "$SIG_GALLERY_NAME" ]]; then
  SIG_GALLERY_NAME="PackerSigGalleryEastUS"
fi

if [[ -n "$PKR_RG_NAME" ]]; then
  #clean up the packer generated resource group
  id=$(az group show --name ${PKR_RG_NAME} | jq .id)
  if [ -n "$id" ]; then
    echo "Deleting packer resource group ${PKR_RG_NAME}"
    az group delete --name ${PKR_RG_NAME} --yes
  fi
fi

#clean up the vnet resource group for Windows
if [ -n "${VNET_RESOURCE_GROUP_NAME}" ]; then
  id=$(az group show --name ${VNET_RESOURCE_GROUP_NAME} | jq .id)
  if [ -n "$id" ]; then
    echo "Deleting packer resource group ${VNET_RESOURCE_GROUP_NAME}"
    az group delete --name ${VNET_RESOURCE_GROUP_NAME} --yes --no-wait
  fi
fi

#clean up managed image
if [[ -n "$AZURE_RESOURCE_GROUP_NAME" && -n "$IMAGE_NAME" ]]; then
  if [[ "$MODE" != "default" ]]; then
    id=$(az image show -n ${IMAGE_NAME} -g ${AZURE_RESOURCE_GROUP_NAME} | jq .id)
    if [ -n "$id" ]; then
      az image delete -n ${IMAGE_NAME} -g ${AZURE_RESOURCE_GROUP_NAME}
    fi
  fi
fi

#cleanup imported sig image version
if [[ -n "${IMPORTED_IMAGE_NAME}" ]]; then
  id=$(az sig image-version show -e 1.0.0 -i ${IMPORTED_IMAGE_NAME} -r ${SIG_GALLERY_NAME} -g ${AZURE_RESOURCE_GROUP_NAME} | jq .id)
  if [ -n "$id" ]; then
    echo "Deleting sig image-version 1.0.0 ${IMPORTED_IMAGE_NAME} from gallery ${SIG_GALLERY_NAME} rg ${AZURE_RESOURCE_GROUP_NAME}"
    az sig image-version delete -e 1.0.0 -i ${IMPORTED_IMAGE_NAME} -r ${SIG_GALLERY_NAME} -g ${AZURE_RESOURCE_GROUP_NAME}
  fi
fi

#cleanup imported sig image definition
if [[ -n "${IMPORTED_IMAGE_NAME}" ]]; then
  id=$(az sig image-definition show --gallery-image-definition ${IMPORTED_IMAGE_NAME} -r ${SIG_GALLERY_NAME} -g ${AZURE_RESOURCE_GROUP_NAME} | jq .id)
  if [ -n "$id" ]; then
    echo "Deleting sig image-definition ${IMPORTED_IMAGE_NAME} from gallery ${SIG_GALLERY_NAME} rg ${AZURE_RESOURCE_GROUP_NAME}"
    az sig image-definition delete --gallery-image-definition ${IMPORTED_IMAGE_NAME} -r ${SIG_GALLERY_NAME} -g ${AZURE_RESOURCE_GROUP_NAME}
  fi
fi

#cleanup imported image
if [[ -n "${IMPORTED_IMAGE_NAME}" ]]; then
  id=$(az image show -n ${IMPORTED_IMAGE_NAME} -g ${AZURE_RESOURCE_GROUP_NAME} | jq .id)
  if [ -n "$id" ]; then
    echo "Deleting managed image ${IMPORTED_IMAGE_NAME} from rg ${AZURE_RESOURCE_GROUP_NAME}"
    az image delete -n ${IMPORTED_IMAGE_NAME} -g ${AZURE_RESOURCE_GROUP_NAME}
  fi
fi

#clean up arm64 OS disk snapshot
if [ ${ARCHITECTURE,,} == "arm64" ] && [ -n "${ARM64_OS_DISK_SNAPSHOT_NAME}" ]; then
  id=$(az snapshot show -n ${ARM64_OS_DISK_SNAPSHOT_NAME} -g ${AZURE_RESOURCE_GROUP_NAME} | jq .id)
  if [ -n "$id" ]; then
    az snapshot delete -n ${ARM64_OS_DISK_SNAPSHOT_NAME} -g ${AZURE_RESOURCE_GROUP_NAME}
  fi
fi

#clean up the temporary storage account
if [[ -n "${SA_NAME}" ]]; then
  id=$(az storage account show -n ${SA_NAME} -g ${AZURE_RESOURCE_GROUP_NAME} | jq .id)
  if [ -n "$id" ]; then
    az storage account delete -n ${SA_NAME} -g ${AZURE_RESOURCE_GROUP_NAME} --yes
  fi
fi

#clean up storage account created over a week ago
if [[ -n "${AZURE_RESOURCE_GROUP_NAME}" && "${DRY_RUN}" == "False" ]]; then
  EXPIRATION_IN_HOURS=168
  # convert to seconds so we can compare it against the "tags.now" property in the resource group metadata
  (( expirationInSecs = ${EXPIRATION_IN_HOURS} * 60 * 60 ))
  # deadline = the "date +%s" representation of the oldest age we're willing to keep
  (( deadline=$(date +%s)-${expirationInSecs%.*} ))
  echo "Current time is $(date)"
  echo "Looking for storage accounts in ${AZURE_RESOURCE_GROUP_NAME} created over ${EXPIRATION_IN_HOURS} hours ago..."
  echo "That is, those created before $(date -d@$deadline) As shown below"
  az storage account list -g ${AZURE_RESOURCE_GROUP_NAME} | jq --arg dl $deadline '.[] | select(.tags.now < $dl).name' | tr -d '\"' || ""
  for storage_account in $(az storage account list -g ${AZURE_RESOURCE_GROUP_NAME} | jq --arg dl $deadline '.[] | select(.tags.now < $dl).name' | tr -d '\"' || ""); do
      if [[ $storage_account = aksimages* ]]; then
          echo "Will delete storage account ${storage_account}# from resource group ${AZURE_RESOURCE_GROUP_NAME}..."
          az storage account delete --name ${storage_account} -g ${AZURE_RESOURCE_GROUP_NAME} --yes  || echo "unable to delete storage account ${storage_account}, will continue..."
          echo "Deletion completed"
      fi
  done
fi