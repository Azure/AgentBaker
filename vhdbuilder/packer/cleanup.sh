#!/bin/bash
set -x

source ./parts/linux/cloud-init/artifacts/cse_benchmark_functions.sh

EXPIRATION_IN_HOURS=168 # 7 days
# convert to seconds so we can compare it against the "tags.now" property in the resource group metadata
(( expirationInSecs = ${EXPIRATION_IN_HOURS} * 60 * 60 ))
# deadline = the "date +%s" representation of the oldest age we're willing to keep
(( deadline=$(date +%s)-${expirationInSecs%.*} ))

if [ -z "$SIG_GALLERY_NAME" ]; then
  SIG_GALLERY_NAME="PackerSigGalleryEastUS"
fi

if [ -n "$PKR_RG_NAME" ]; then
  #clean up the packer generated resource group
  id=$(az group show --name ${PKR_RG_NAME} | jq .id)
  if [ -n "$id" ]; then
    echo "Deleting packer resource group ${PKR_RG_NAME}"
    az group delete --name ${PKR_RG_NAME} --yes
  else
    echo "Packer resource group already successfully deleted"
  fi
fi
capture_benchmark "${SCRIPT_NAME}_delete_packer_rg"

#clean up the test vm resource group
id=$(az group show --name ${TEST_VM_RESOURCE_GROUP_NAME} | jq .id)
if [ -n "$id" ]; then
  echo "Deleting test vm resource group ${TEST_VM_RESOURCE_GROUP_NAME}"
  az group delete --name ${TEST_VM_RESOURCE_GROUP_NAME} --yes
fi
capture_benchmark "${SCRIPT_NAME}_delete_test_vm_rg"

#clean up managed image
if [ -n "$AZURE_RESOURCE_GROUP_NAME" ] && [ -n "$IMAGE_NAME" ]; then
  # shellcheck disable=SC3010
  if [[ ${ARCHITECTURE,,} != "arm64" ]]; then
    id=$(az image show -n ${IMAGE_NAME} -g ${AZURE_RESOURCE_GROUP_NAME} | jq .id)
    if [ -n "$id" ]; then
      echo "Deleting managed image ${IMAGE_NAME} under resource group ${AZURE_RESOURCE_GROUP_NAME}"
      az image delete -n ${IMAGE_NAME} -g ${AZURE_RESOURCE_GROUP_NAME}
    fi
  else
    echo "Not attempting managed image deletion due to ARM64 architecture."
  fi
fi
capture_benchmark "${SCRIPT_NAME}_delete_managed_image"

#cleanup imported sig image version
if [ -n "${IMPORTED_IMAGE_NAME}" ]; then
  id=$(az sig image-version show -e 1.0.0 -i ${IMPORTED_IMAGE_NAME} -r ${SIG_GALLERY_NAME} -g ${AZURE_RESOURCE_GROUP_NAME} | jq .id)
  if [ -n "$id" ]; then
    echo "Deleting sig image-version 1.0.0 ${IMPORTED_IMAGE_NAME} from gallery ${SIG_GALLERY_NAME} rg ${AZURE_RESOURCE_GROUP_NAME}"
    az sig image-version delete -e 1.0.0 -i ${IMPORTED_IMAGE_NAME} -r ${SIG_GALLERY_NAME} -g ${AZURE_RESOURCE_GROUP_NAME}
  fi
fi

#cleanup imported sig image definition
if [ -n "${IMPORTED_IMAGE_NAME}" ]; then
  id=$(az sig image-definition show --gallery-image-definition ${IMPORTED_IMAGE_NAME} -r ${SIG_GALLERY_NAME} -g ${AZURE_RESOURCE_GROUP_NAME} | jq .id)
  if [ -n "$id" ]; then
    echo "Deleting sig image-definition ${IMPORTED_IMAGE_NAME} from gallery ${SIG_GALLERY_NAME} rg ${AZURE_RESOURCE_GROUP_NAME}"
    az sig image-definition delete --gallery-image-definition ${IMPORTED_IMAGE_NAME} -r ${SIG_GALLERY_NAME} -g ${AZURE_RESOURCE_GROUP_NAME}
  fi
fi

#cleanup imported image
if [ -n "${IMPORTED_IMAGE_NAME}" ]; then
  id=$(az image show -n ${IMPORTED_IMAGE_NAME} -g ${AZURE_RESOURCE_GROUP_NAME} | jq .id)
  if [ -n "$id" ]; then
    echo "Deleting managed image ${IMPORTED_IMAGE_NAME} from rg ${AZURE_RESOURCE_GROUP_NAME}"
    az image delete -n ${IMPORTED_IMAGE_NAME} -g ${AZURE_RESOURCE_GROUP_NAME}
  fi
fi
capture_benchmark "${SCRIPT_NAME}_cleanup_imported_image"

#cleanup built sig image if the generated sig is for production only, but not for test purpose.
#If SIG_FOR_PRODUCTION is set to true, the sig has been converted to VHD before this step.
#And since we only need to upload the converted VHD to the classic storage account, there's no need to keep the built sig.
if [ "${MODE}" = "windowsVhdMode" ] && [ "$SIG_FOR_PRODUCTION" = "True" ] && [ "$DRY_RUN" = "False" ]; then
  if [ -n "${SIG_IMAGE_NAME}" ]; then
    # Delete sig image version first
    echo "SIG_IMAGE_NAME is ${SIG_IMAGE_NAME}, deleting sig image version first"
    versions=$(az sig image-version list -i ${SIG_IMAGE_NAME} -r ${SIG_GALLERY_NAME} -g ${AZURE_RESOURCE_GROUP_NAME} | jq -r '.[].name')
    for version in $versions; do
        az sig image-version show -e $version -i ${SIG_IMAGE_NAME} -r ${SIG_GALLERY_NAME} -g ${AZURE_RESOURCE_GROUP_NAME} | jq .id
        echo "Deleting sig image-version ${version} ${SIG_IMAGE_NAME} from gallery ${SIG_GALLERY_NAME} rg ${AZURE_RESOURCE_GROUP_NAME}"
        az sig image-version delete -e $version -i ${SIG_IMAGE_NAME} -r ${SIG_GALLERY_NAME} -g ${AZURE_RESOURCE_GROUP_NAME}
        az sig image-version wait --deleted --timeout 300 -e $version -i ${SIG_IMAGE_NAME} -r ${SIG_GALLERY_NAME} -g ${AZURE_RESOURCE_GROUP_NAME}
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
      az sig image-definition wait --deleted --timeout 300 --gallery-image-definition ${SIG_IMAGE_NAME} -r ${SIG_GALLERY_NAME} -g ${AZURE_RESOURCE_GROUP_NAME}
      #double confirm
      id=$(az sig image-definition show --gallery-image-definition ${SIG_IMAGE_NAME} -r ${SIG_GALLERY_NAME} -g ${AZURE_RESOURCE_GROUP_NAME} | jq .id)
      if [ -n "$id" ]; then
          echo "Deleting sig image-definition ${SIG_IMAGE_NAME} failed"
      else
          echo "Deletion of sig image-definition ${SIG_IMAGE_NAME} completed"
      fi
    fi
  fi
fi

#clean up the temporary storage account
if [ -n "${SA_NAME}" ]; then
  id=$(az storage account show -n ${SA_NAME} -g ${AZURE_RESOURCE_GROUP_NAME} | jq .id)
  if [ -n "$id" ]; then
    az storage account delete -n ${SA_NAME} -g ${AZURE_RESOURCE_GROUP_NAME} --yes
  fi
fi
capture_benchmark "${SCRIPT_NAME}_cleanup_temp_storage"

#delete the SIG version that was created during a dry-run of linuxVhdMode
# shellcheck disable=SC3010
if [[ "${MODE}" == "linuxVhdMode" && "${DRY_RUN,,}" == "true" ]]; then
  echo "running dry-run in mode ${MODE}, attempting to delete output SIG version: ${AZURE_RESOURCE_GROUP_NAME}/${SIG_GALLERY_NAME}/${SIG_IMAGE_NAME}/${CAPTURED_SIG_VERSION}"
  id=$(az sig image-definition show -g ${AZURE_RESOURCE_GROUP_NAME} -r ${SIG_GALLERY_NAME} -i ${SIG_IMAGE_NAME} | jq '.id')
  if [ -n "$id" ]; then
    az sig image-version delete -g ${AZURE_RESOURCE_GROUP_NAME} -r ${SIG_GALLERY_NAME} -i ${SIG_IMAGE_NAME} -e ${CAPTURED_SIG_VERSION}
  else
    echo "specified image-definition ${AZURE_RESOURCE_GROUP_NAME}/${SIG_GALLERY_NAME}/${SIG_IMAGE_NAME} does not exist, will not delete SIG image version"
  fi
fi

echo -e "Packer cleanup successfully completed\n\n\n"
capture_benchmark "${SCRIPT_NAME}_overall" true
process_benchmarks
