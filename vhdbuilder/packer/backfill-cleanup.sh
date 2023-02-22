#!/bin/bash -x

EXPIRATION_IN_HOURS=168
# convert to seconds so we can compare it against the "tags.now" property in the resource group metadata
(( expirationInSecs = ${EXPIRATION_IN_HOURS} * 60 * 60 ))
# deadline = the "date +%s" representation of the oldest age we're willing to keep
(( deadline=$(date +%s)-${expirationInSecs%.*} ))

if [[ "${MODE}" != "windowsVhdMode" ]]; then
  exit 0
fi

if [[ "${BACKFILL_RESOURCE_DELETION}" == "False" ]]; then
  exit 0
fi

# attempt to clean up Windows managed images and SIG image versions created over a week ago in SIG_GALLERY_NAME (cannot be the production gallery)
# this can be used in PR check-in pipelines together with a set SIG_GALLERY_NAME from which we'd like to free up resources
if [[ -n "${AZURE_RESOURCE_GROUP_NAME}" && "${SIG_GALLERY_NAME}" != "AKSWindows" ]]; then
  echo "Looking for Windows managed images in ${AZURE_RESOURCE_GROUP_NAME} created over ${EXPIRATION_IN_HOURS} hours ago..."

  managed_image_ids=""
  sig_version_ids=""

  # delete outdated Windows managed images and associated SIG versions (.tags.os must be "Windows")
  images=$(az image list -g ${AZURE_RESOURCE_GROUP_NAME} | jq --arg dl $deadline -r '.[] | select(.tags.os == "Windows") | select(.tags.now < $dl).name')
  for image in $images; do
    managed_image_ids+=" /subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${AZURE_RESOURCE_GROUP_NAME}/providers/Microsoft.Compute/images/${image}"
    sig_version_ids+=" /subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${AZURE_RESOURCE_GROUP_NAME}/providers/Microsoft.Compute/galleries/${SIG_GALLERY_NAME}/images/${image%-*}/versions/${image#*-}"
    echo "Will delete Windows managed image ${image} and associated SIG version from resource group ${AZURE_RESOURCE_GROUP_NAME}"
  done

  if [[ -n "${managed_image_ids}" ]]; then
    echo "Attempting to delete $(echo ${managed_image_ids} | wc -w) Windows managed images..."
    az resource delete --ids ${managed_image_ids} > /dev/null || echo "Windows managed image deletion was not successful, continuing..."
  else
    echo "Did not find any Windows managed images eligible for deletion"
  fi

  if [[ -n "${sig_version_ids}" ]]; then
    echo "Attempting to delete $(echo ${sig_version_ids} | wc -w) Windows SIG image versions associated with old managed images..."
    az resource delete --ids ${sig_version_ids} > /dev/null || echo "Windows SIG image version deletion was not successful, continuing..."
  else
    echo "Did not find any SIG versions associated with old Windows managed images eligible for deletion"
  fi
fi

# attempt to clean up Windows SIG image versions in all galleries except "AKSWindows" created over a week ago
if [[ -n "${AZURE_RESOURCE_GROUP_NAME}" ]]; then
  gallery_list=$(az sig list -g ${AZURE_RESOURCE_GROUP_NAME} | jq -r '.[] | select(.name != "AKSWindows") | .name')
  for gallery in $gallery_list; do
    if [[ "${gallery}" =~ WS2019Gallery* ]]; then
      create_date=${gallery:13:12}
    elif [[ "${gallery_name}" =~ WS2019_containerdGallery* ]]; then
      create_date=${gallery_name:24:12}
    elif [[ "${gallery_name}" =~ WS2022_containerdGallery* ]]; then
      create_date=${gallery_name:24:12}
    elif [[ "${gallery_name}" =~ WS2022_containerd_gen2Gallery* ]]; then
      create_date=${gallery_name:29:12}
    else
      continue
    fi
    due_date=$(date +%y%m%d -d "7 days ago")
    # clean the entire SIG resources if it's one week ago
    if [[ $create_date < $due_date ]]; then
      echo "Deleting gallery ${gallery}"
      az sig delete --gallery-name ${gallery} --resource-group ${AZURE_RESOURCE_GROUP_NAME}
      image_defs=$(az sig image-definition list -g ${AZURE_RESOURCE_GROUP_NAME} -r ${gallery} | jq -r '.[] | select(.osType == "Windows").name')
      if [[ -n "${image_defs}" ]]; then 
        echo "Still having SIG image-definitions"
      fi
    fi
  done
fi

# clean up storage account created over a week ago
if [[ -n "${AZURE_RESOURCE_GROUP_NAME}" ]]; then
  echo "Looking for storage accounts in ${AZURE_RESOURCE_GROUP_NAME} created over ${EXPIRATION_IN_HOURS} hours ago..."
  echo "That is, those created before $(date -d@$deadline) As shown below"
  az storage account list -g ${AZURE_RESOURCE_GROUP_NAME} | jq --arg dl $deadline -r '.[] | select(.tags.now < $dl).name'
  for storage_account in $(az storage account list -g ${AZURE_RESOURCE_GROUP_NAME} | jq --arg dl $deadline -r '.[] | select(.tags.now < $dl).name'); do
      if [[ $storage_account = aksimages* ]]; then
          echo "Will delete storage account ${storage_account}# from resource group ${AZURE_RESOURCE_GROUP_NAME}..."
          az storage account delete --name ${storage_account} -g ${AZURE_RESOURCE_GROUP_NAME} --yes  || echo "unable to delete storage account ${storage_account}, will continue..."
          echo "Deletion completed"
      fi
  done
fi

# clean up old packer and vnet resource groups created over a week ago that didn't get cleaned in the building vhd step in case the pipeline fails
if [[ -n "${AZURE_RESOURCE_GROUP_NAME}" ]]; then
  pkr_groups=$(az group list | jq --arg dl $deadline -r '.[] | select(.name | test("pkr-Resource-Group*")) | select(.tags.now < $dl).name')
  for pkr_group in $pkr_groups; do
      echo "Deleting packer resource group $pkr_group"
      az group delete --name ${pkr_group} --yes 
      echo "Deleted packer resource group $pkr_group"
  done
fi
