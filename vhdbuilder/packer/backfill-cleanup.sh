#!/bin/bash -x

if [ "${MODE}" != "windowsVhdMode" ]; then
  exit 0
fi

if [ ${SUBSCRIPTION_ID} = ${PROD_SUBSCRIPTION_ID} ]; then
  echo "Shouldn't do backfill clean up in production subscription."
  exit 1
fi

EXPIRATION_IN_HOURS=168
# convert to seconds so we can compare it against the "tags.now" property in the resource group metadata
(( expirationInSecs = ${EXPIRATION_IN_HOURS} * 60 * 60 ))
# deadline = the "date +%s" representation of the oldest age we're willing to keep
(( deadline=$(date +%s)-${expirationInSecs%.*} ))

# attempt to clean up Windows managed images and SIG image versions created over a week ago
if [ -n "${AZURE_RESOURCE_GROUP_NAME}" ]; then
  echo "Looking for Windows managed images in ${AZURE_RESOURCE_GROUP_NAME} created over ${EXPIRATION_IN_HOURS} hours ago..."

  managed_image_ids=""
  sig_version_ids=""

  # delete outdated Windows managed images and associated SIG versions (.tags.os must be "Windows")
  images=$(az image list -g ${AZURE_RESOURCE_GROUP_NAME} | jq --arg dl $deadline -r '.[] | select(.tags.os == "Windows") | select(.tags.now < $dl).name')
  for image in $images; do
    managed_image_ids+=" /subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${AZURE_RESOURCE_GROUP_NAME}/providers/Microsoft.Compute/images/${image}"
    echo "Will delete Windows managed image ${image} and associated SIG version from resource group ${AZURE_RESOURCE_GROUP_NAME}"
  done

  if [ -n "${managed_image_ids}" ]; then
    echo "Attempting to delete $(echo ${managed_image_ids} | wc -w) Windows managed images..."
    az resource delete --ids ${managed_image_ids} > /dev/null || echo "Windows managed image deletion was not successful, continuing..."
  else
    echo "Did not find any Windows managed images eligible for deletion"
  fi
fi

# attempt to clean up Windows SIG image versions in all galleries except "AKSWindows" created over a week ago
if [ -n "${AZURE_RESOURCE_GROUP_NAME}" ]; then
  gallery_list=$(az sig list -g ${AZURE_RESOURCE_GROUP_NAME} | jq -r '.[] | select(.name != "AKSWindows") | .name')
  for gallery in $gallery_list; do
    case "$gallery" in
    WS2019Gallery*)
        create_date=${gallery:13:6}
        ;;
    WS2019_containerdGallery*)
        create_date=${gallery:24:6}
        ;;
    WS2022_containerdGallery*)
        create_date=${gallery:24:6}
        ;;
    WS2022_containerd_gen2Gallery*)
        create_date=${gallery:29:6}
        ;;
    WSGallery*)
        create_date=${gallery:9:6}
        ;;
    *)
        continue
        ;;
    esac

    due_date=$(date +%y%m%d -d "7 days ago")
    echo "create_date is ${create_date}"
    echo "due_date is ${due_date}"
    # clean the entire SIG resources if it's one week ago
    if [ "$create_date" -lt "$due_date" ]; then
      echo "Finding sig image definitions from gallery ${gallery}"
      image_defs=$(az sig image-definition list -g ${AZURE_RESOURCE_GROUP_NAME} -r ${gallery} | jq -r '.[] | select(.osType == "Windows").name')
      for image_definition in $image_defs; do
        echo "Finding sig image versions associated with ${image_definition} in gallery ${gallery}"
        image_versions=$(az sig image-version list -g ${AZURE_RESOURCE_GROUP_NAME} -r ${gallery} -i ${image_definition} | jq -r '.[].name')
        for image_version in $image_versions; do
          echo "Deleting sig image-version ${image_version} ${image_definition} from gallery ${gallery} rg ${AZURE_RESOURCE_GROUP_NAME}"
          az sig image-version delete -e $image_version -i ${image_definition} -r ${gallery} -g ${AZURE_RESOURCE_GROUP_NAME}
          az sig image-version wait --deleted --timeout 1800 -e $image_version -i ${image_definition} -r ${gallery} -g ${AZURE_RESOURCE_GROUP_NAME}
        done
        image_versions=$(az sig image-version list -g ${AZURE_RESOURCE_GROUP_NAME} -r ${gallery} -i ${image_definition} | jq -r '.[].name')
        echo "image versions are $image_versions"
        if [ -z "${image_versions}" ]; then
          echo "Deleting sig image-definition ${image_definition} from gallery ${gallery} rg ${AZURE_RESOURCE_GROUP_NAME}"
          az sig image-definition delete --gallery-image-definition ${image_definition} -r ${gallery} -g ${AZURE_RESOURCE_GROUP_NAME}
          az sig image-definition wait --deleted --timeout 1800 --gallery-image-definition ${image_definition} -r ${gallery} -g ${AZURE_RESOURCE_GROUP_NAME}
        fi
      done
      image_defs=$(az sig image-definition list -g ${AZURE_RESOURCE_GROUP_NAME} -r ${gallery} | jq -r '.[] | select(.osType == "Windows").name')

      if [ -n "$image_defs" ]; then
        echo "$image_defs"
      fi
      
      echo "Deleting gallery ${gallery}"
      az sig delete --gallery-name ${gallery} --resource-group ${AZURE_RESOURCE_GROUP_NAME}
    fi
  done
fi

# attemp to clean up old test Windows SIG image versions over 3 months ago
if [ -n "${AZURE_RESOURCE_GROUP_NAME}" ]; then
  gallery_list=$(az sig list -g ${AZURE_RESOURCE_GROUP_NAME} | jq -r '.[].name')

  due_date=$(date +%F -d "90 days ago")
  for gallery in $gallery_list; do
    image_defs=$(az sig image-definition list -g ${AZURE_RESOURCE_GROUP_NAME} -r ${gallery} | jq -r '.[] | select(.osType == "Windows").name')
    for image_definition in $image_defs; do
      image_versions=$(az sig image-version list -g ${AZURE_RESOURCE_GROUP_NAME} -r ${gallery} -i ${image_definition} | jq --arg ValueForDueDate "$due_date" -r '.[] | select(.publishingProfile.publishedDate < $ValueForDueDate).name')
      for image_version in $image_versions; do
        echo "Deleting sig image-version ${image_version} ${image_definition} from gallery ${gallery} rg ${AZURE_RESOURCE_GROUP_NAME}"
        az sig image-version delete -e $image_version -i ${image_definition} -r ${gallery} -g ${AZURE_RESOURCE_GROUP_NAME}
      done
    done
  done
fi

# clean up storage account created over a week ago
if [ -n "${AZURE_RESOURCE_GROUP_NAME}" ]; then
  echo "Looking for storage accounts in ${AZURE_RESOURCE_GROUP_NAME} created over ${EXPIRATION_IN_HOURS} hours ago..."
  echo "That is, those created before $(date -d@$deadline) As shown below"
  az storage account list -g ${AZURE_RESOURCE_GROUP_NAME} | jq --arg dl $deadline -r '.[] | select(.tags.now < $dl).name'
  for storage_account in $(az storage account list -g ${AZURE_RESOURCE_GROUP_NAME} | jq --arg dl $deadline -r '.[] | select(.tags.now < $dl).name'); do
      # shellcheck disable=SC3010
      if [[ $storage_account = aksimages* ]]; then
          echo "Will delete storage account ${storage_account}# from resource group ${AZURE_RESOURCE_GROUP_NAME}..."
          az storage account delete --name ${storage_account} -g ${AZURE_RESOURCE_GROUP_NAME} --yes  || echo "unable to delete storage account ${storage_account}, will continue..."
          echo "Deletion completed"
      fi
  done
fi

# clean up old packer and vnet resource groups created over a week ago that didn't get cleaned in the building vhd step in case the pipeline fails
if [ -n "${AZURE_RESOURCE_GROUP_NAME}" ]; then
  pkr_groups=$(az group list | jq --arg dl $deadline -r '.[] | select(.name | test("pkr-Resource-Group*")) | select(.tags.now < $dl).name')
  for pkr_group in $pkr_groups; do
      echo "Deleting packer resource group $pkr_group"
      az group delete --name ${pkr_group} --yes 
      echo "Deleted packer resource group $pkr_group"
  done
fi

# clean up resource group "windows-" which is created over a week ago
if [[ -n "${SUBSCRIPTION_ID}" ]]; then
  echo "Looking for resource groups in ${SUBSCRIPTION_ID} created over one week ago..."

 # The timestamp in the resource group name does not represent the creation date of the resource group, but the date when the image version was created.
 # So we will need to rely on the "tags.createdtime" property in the resource group metadata to determine the creation date.
  due_date=$(date +%Y-%m-%d -d "7 days ago")
  rg_groups=$(az group list | jq --arg dl $due_date -r '.[] | select(.name | startswith("windows-")) | select(.tags.createdtime != null and .tags.createdtime < $dl).name')
  for rg_group in $rg_groups; do
      echo "Deleting resource group $rg_group"
      az group delete --name ${rg_group} --yes 
      echo "Deleted resource group $rg_group"
  done
fi

#clean up the image versions generated from successful builds
if [[ -n "${AZURE_RESOURCE_GROUP_NAME}" ]]; then
  echo "Looking for image versions in ${AZURE_RESOURCE_GROUP_NAME}, only keep the last 10..."
  image_defs=("windows-2019-containerd" "windows-2022-containerd" "windows-2022-containerd-gen2" "windows-23h2" "windows-23h2-gen2" "windows-2025" "windows-2025-gen2" "windows-activebranch" "rs_sparc")
  for image_def in ${image_defs[@]}; do
    to_keep=$(az sig image-version list -g ${AZURE_RESOURCE_GROUP_NAME} -r ${SIG_GALLERY_NAME} -i $image_def \
      | jq -r '.[] | {name: .name, publishDate: .publishingProfile.publishedDate}' |  jq -s 'sort_by(.publishDate) | reverse |.[:10] | .[].name')

    echo "Keeping the following image versions for ${image_def}: $to_keep"

    image_versions=$(az sig image-version list -g ${AZURE_RESOURCE_GROUP_NAME} -r ${SIG_GALLERY_NAME} -i $image_def | jq --arg dl $due_date -r '.[] | select(.publishingProfile.publishedDate < $dl).name')

    for image_version in $image_versions; do
    #keep the last three image versions if there has been no builds for a week
      if [[ ! ${to_keep} =~  ${image_version} ]];  then
        echo "Deleting sig image-version ${image_version} ${image_def} from gallery $SIG_GALLERY_NAME rg ${AZURE_RESOURCE_GROUP_NAME}"
        az sig image-version delete -e $image_version -i ${image_def} -r ${SIG_GALLERY_NAME} -g ${AZURE_RESOURCE_GROUP_NAME}
        az sig image-version wait --deleted --timeout 1800 -e $image_version -i ${image_def} -r ${SIG_GALLERY_NAME} -g ${AZURE_RESOURCE_GROUP_NAME}
        echo "Deleted sig image-version ${image_version} ${image_def} from gallery $SIG_GALLERY_NAME rg ${AZURE_RESOURCE_GROUP_NAME}"
      fi
    done
  done

  echo "Removing old image created from image gallery for older than 10 days"
  (( expirationInSecs = 10 * 60 * 60 ))
  # deadline = the "date +%s" representation of the oldest age we're willing to keep
  (( deadline=$(date +%s)-${expirationInSecs%.*} ))
  images=(az image list -g  wcct-agentbaker-test | jq --arg dl $deadline -r '.[] | select(.name | test("^rs_sparc_ctr_serverdatacentercore")) | select(.tags.TimeStamp < $dl).name')
    for image in $images; do
    managed_image_ids+=" /subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${AZURE_RESOURCE_GROUP_NAME}/providers/Microsoft.Compute/images/${image}"
    echo "Will delete 1ES published managed image ${image} from resource group ${AZURE_RESOURCE_GROUP_NAME}"
  done

  if [ -n "${managed_image_ids}" ]; then
    echo "Attempting to delete $(echo ${managed_image_ids} | wc -w) Windows managed images..."
    az resource delete --ids ${managed_image_ids} > /dev/null || echo "Windows managed image deletion was not successful, continuing..."
  else
    echo "Did not find any published managed images eligible for deletion"
  fi
fi