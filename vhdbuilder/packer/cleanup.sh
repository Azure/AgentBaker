#!/bin/bash -x

EXPIRATION_IN_HOURS=168
# convert to seconds so we can compare it against the "tags.now" property in the resource group metadata
(( expirationInSecs = ${EXPIRATION_IN_HOURS} * 60 * 60 ))
# deadline = the "date +%s" representation of the oldest age we're willing to keep
(( deadline=$(date +%s)-${expirationInSecs%.*} ))

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

#clean up managed image
if [[ -n "$AZURE_RESOURCE_GROUP_NAME" && -n "$IMAGE_NAME" ]]; then
  if [[ "$MODE" != "default" ]]; then
    id=$(az image show -n ${IMAGE_NAME} -g ${AZURE_RESOURCE_GROUP_NAME} | jq .id)
    if [ -n "$id" ]; then
      echo "deleting managed image ${IMAGE_NAME} under resource group ${AZURE_RESOURCE_GROUP_NAME}"
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

#cleanup built sig image if the generated sig is for production only, but not for test purpose.
#If SIG_FOR_PRODUCTION is set to true, the sig has been converted to VHD before this step.
#And since we only need to upload the converted VHD to the classic storage account, there's no need to keep the built sig.
if [[ "${MODE}" == "windowsVhdMode" && "$SIG_FOR_PRODUCTION" == "True" ]]; then
  if [[ -n "${SIG_IMAGE_NAME}" ]]; then
    # Delete sig image version first
    echo "SIG_IMAGE_NAME is ${SIG_IMAGE_NAME}, deleting sig image version first"
    versions=$(az sig image-version list -i ${SIG_IMAGE_NAME} -r ${SIG_GALLERY_NAME} -g ${AZURE_RESOURCE_GROUP_NAME} | jq -r '.[].name')
    for version in $versions; do
        az sig image-version show -e $version -i ${SIG_IMAGE_NAME} -r ${SIG_GALLERY_NAME} -g ${AZURE_RESOURCE_GROUP_NAME} | jq .id
        echo "Deleting sig image-version ${version} ${SIG_IMAGE_NAME} from gallery ${SIG_GALLERY_NAME} rg ${AZURE_RESOURCE_GROUP_NAME}"
        az sig image-version delete -e $version -i ${SIG_IMAGE_NAME} -r ${SIG_GALLERY_NAME} -g ${AZURE_RESOURCE_GROUP_NAME}
        #double confirm
        id=$(az sig image-version show -e $version -i ${SIG_IMAGE_NAME} -r ${SIG_GALLERY_NAME} -g ${AZURE_RESOURCE_GROUP_NAME} | jq .id)
        if [ -n "$id" ]; then
            echo "Deleting sig image-version $version failed"
        else 
            echo "Deletion of sig image-version $version completed"
        fi
    done

    # Delete sig image finally
    echo "SIG_IMAGE_NAME is ${SIG_IMAGE_NAME}, deleting sig image definition next"
    id=$(az sig image-definition show --gallery-image-definition ${SIG_IMAGE_NAME} -r ${SIG_GALLERY_NAME} -g ${AZURE_RESOURCE_GROUP_NAME} | jq .id)
    if [ -n "$id" ]; then
      echo "Deleting sig image-definition ${SIG_IMAGE_NAME} from gallery ${SIG_GALLERY_NAME} rg ${AZURE_RESOURCE_GROUP_NAME}"
      az sig image-definition delete --gallery-image-definition ${SIG_IMAGE_NAME} -r ${SIG_GALLERY_NAME} -g ${AZURE_RESOURCE_GROUP_NAME}
      #double confirm
      id=$(az sig image-definition show --gallery-image-definition ${SIG_IMAGE_NAME} -r ${SIG_GALLERY_NAME} -g ${AZURE_RESOURCE_GROUP_NAME} | jq .id)
      if [ -n "$id" ]; then
          echo "Deleting sig image-definition ${SIG_IMAGE_NAME} failed"
      else 
          echo "Deletion of sig image-definition ${SIG_IMAGE_NAME} completed"
      fi
    fi

    # Delete sig image gallery
    echo "SIG_GALLERY_NAME is ${SIG_GALLERY_NAME}, deleting sig gallery since sig is no longer needed"
    az sig delete --gallery-name ${SIG_GALLERY_NAME} --resource-group ${AZURE_RESOURCE_GROUP_NAME}
  fi
fi

#clean up arm64 OS disk snapshot
if [[ "${MODE}" == "linuxVhdMode" ]] && [[ ${ARCHITECTURE,,} == "arm64" ]] && [ -n "${ARM64_OS_DISK_SNAPSHOT_NAME}" ]; then
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

#delete the SIG version that was created during a dry-run of linuxVhdMode
if [[ "${MODE}" == "linuxVhdMode" && "${DRY_RUN,,}" == "true" ]]; then
  echo "running dry-run in mode ${MODE}, attempting to delete output SIG version: ${AZURE_RESOURCE_GROUP_NAME}/${SIG_GALLERY_NAME}/${SIG_IMAGE_NAME}/${CAPTURED_SIG_VERSION}"
  id=$(az sig image-definition show -g ${AZURE_RESOURCE_GROUP_NAME} -r ${SIG_GALLERY_NAME} -i ${SIG_IMAGE_NAME} | jq '.id')
  if [ -n "$id" ]; then
    az sig image-version delete -g ${AZURE_RESOURCE_GROUP_NAME} -r ${SIG_GALLERY_NAME} -i ${SIG_IMAGE_NAME} -e ${CAPTURED_SIG_VERSION}
  else
    echo "specified image-definition ${AZURE_RESOURCE_GROUP_NAME}/${SIG_GALLERY_NAME}/${SIG_IMAGE_NAME} does not exist, will not delete SIG image version"
  fi
fi

# attempt to clean up managed images and associated SIG versions created over a week ago
if [[ "${MODE}" == "linuxVhdMode" && -n "${AZURE_RESOURCE_GROUP_NAME}" && "${DRY_RUN,,}" == "false" ]]; then
  set +x # to avoid blowing up logs
  echo "Looking for managed images in ${AZURE_RESOURCE_GROUP_NAME} created over ${EXPIRATION_IN_HOURS} hours ago..."

  managed_image_ids=""
  sig_version_ids=""
  # we limit deletions to 25 managed images to make sure the build doesn't take too long and can finish successfully
  for image in $(az image list -g ${AZURE_RESOURCE_GROUP_NAME} | jq --arg dl $deadline '.[] | select(.name | test("Ubuntu*|CBLMariner*|V1*|V2*|1804*|2004*|2204*")) | select(.tags.now < $dl).name' | head -n 25 | tr -d '\"' || ""); do
    managed_image_ids="${managed_image_ids} /subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${AZURE_RESOURCE_GROUP_NAME}/providers/Microsoft.Compute/images/${image}"
    sig_version_ids="${sig_version_ids} /subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${AZURE_RESOURCE_GROUP_NAME}/providers/Microsoft.Compute/galleries/${SIG_GALLERY_NAME}/images/${image%-*}/versions/${image#*-}"
    echo "Will delete managed image ${image} and associated SIG version from resource group ${AZURE_RESOURCE_GROUP_NAME}"
  done

  if [[ -n "${managed_image_ids}" ]]; then
    echo "Attempting to delete $(echo ${managed_image_ids} | wc -w) managed images..."
    az resource delete --ids ${managed_image_ids} > /dev/null || echo "managed image deletion was not successful, continuing..."
  else
    echo "Did not find any managed images eligible for deletion"
  fi

  if [[ -n "${sig_version_ids}" ]]; then
    echo "Attempting to delete $(echo ${sig_version_ids} | wc -w) SIG image versions associated with old managed images..."
    az resource delete --ids ${sig_version_ids} > /dev/null || echo "SIG image version deletion was not successful, continuing..."
  else
    echo "Did not find any SIG versions associated with old managed images eligible for deletion"
  fi

  old_sig_version_ids=""
  # we limit deletion to 15 SIG image versions per image definition
  for image_definition in $(az sig image-definition list -g ${AZURE_RESOURCE_GROUP_NAME} -r ${SIG_GALLERY_NAME} | jq '.[] | select(.name | test("Ubuntu*|CBLMariner*|V1*|V2*|1804*|2004*|2204*")).name' | tr -d '\"' || ""); do
    for image_version in $(az sig image-version list -g ${AZURE_RESOURCE_GROUP_NAME} -r ${SIG_GALLERY_NAME} -i ${image_definition} | jq --arg dl $deadline '.[] | select(.tags.now < $dl).name' | head -n 15 | tr -d '\"' || ""); do
      old_sig_version_ids="${old_sig_version_ids} /subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${AZURE_RESOURCE_GROUP_NAME}/providers/Microsoft.Compute/galleries/${SIG_GALLERY_NAME}/images/${image_definition}/versions/${image_version}"
    done
  done

  if [[ -n "${old_sig_version_ids}" ]]; then
    echo "Attempting to delete $(echo ${old_sig_version_ids} | wc -w) SIG image versions older than ${EXPIRATION_IN_HOURS} hours..."
    az resource delete --ids ${old_sig_version_ids} > /dev/null || echo "SIG image version deletion was not successful, continuing..."
  else
    echo "Did not find any old SIG versions eligible for deletion"
  fi
  
  set -x
fi

# clean up storage accounts created over a week ago
if [[ -n "${AZURE_RESOURCE_GROUP_NAME}" && "${DRY_RUN}" == "False" ]]; then
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