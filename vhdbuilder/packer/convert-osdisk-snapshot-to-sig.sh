#!/bin/bash -e

required_env_vars=(
    "SUBSCRIPTION_ID"
    "AZURE_RESOURCE_GROUP_NAME"
    "AZURE_LOCATION"
)

for v in "${required_env_vars[@]}"
do
    if [ -z "${!v}" ]; then
        echo "$v was not set!"
        exit 1
    fi
done

OS_DISK_SNAPSHOT_NAME="$(grep "os_disk_snapshot_name" ./vhdbuilder/packer/settings.json | awk -F':' '{print $2}' | awk -F'"' '{print $2}')"
CAPTURED_SIG_VERSION="$(grep "captured_sig_version" ./vhdbuilder/packer/settings.json | awk -F':' '{print $2}' | awk -F'"' '{print $2}')"
SIG_IMAGE_NAME="$(grep "sig_image_name" ./vhdbuilder/packer/settings.json | awk -F':' '{print $2}' | awk -F'"' '{print $2}')" && \
echo "ManagedImageSharedImageGalleryId: /subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${AZURE_RESOURCE_GROUP_NAME}/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/${SIG_IMAGE_NAME}/versions/${CAPTURED_SIG_VERSION}"

ARCH_STRING=""
if [[ "${ARCHITECTURE,,}" == "arm64" ]]; then
  ARCH_STRING+="--architecture Arm64"
else 
  ARCH_STRING=""
fi

time_now=$(echo ${CAPTURED_SIG_VERSION} | cut -d '.' -f2)
disk_snapshot_id="/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${AZURE_RESOURCE_GROUP_NAME}/providers/Microsoft.Compute/snapshots/${OS_DISK_SNAPSHOT_NAME}"
az snapshot update --resource-group ${AZURE_RESOURCE_GROUP_NAME} -n ${OS_DISK_SNAPSHOT_NAME} ${ARCH_STRING}
az sig image-version create --location $AZURE_LOCATION --resource-group ${AZURE_RESOURCE_GROUP_NAME} --gallery-name PackerSigGalleryEastUS \
     --gallery-image-definition ${SIG_IMAGE_NAME} --gallery-image-version ${CAPTURED_SIG_VERSION} \
     --os-snapshot ${disk_snapshot_id} --tags now=${time_now} --replication-mode Shallow
