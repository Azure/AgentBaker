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

ARM64_OS_DISK_SNAPSHOT_NAME="$(grep "arm64_os_disk_snapshot_name" ./vhdbuilder/packer/settings.json | awk -F':' '{print $2}' | awk -F'"' '{print $2}')"
CAPTURED_SIG_VERSION="$(grep "captured_sig_version" ./vhdbuilder/packer/settings.json | awk -F':' '{print $2}' | awk -F'"' '{print $2}')"
SIG_IMAGE_NAME="$(grep "sig_image_name" ./vhdbuilder/packer/settings.json | awk -F':' '{print $2}' | awk -F'"' '{print $2}')" && \
echo "ManagedImageSharedImageGalleryId: /subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${AZURE_RESOURCE_GROUP_NAME}/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/${SIG_IMAGE_NAME}/versions/${CAPTURED_SIG_VERSION}"

disk_snapshot_id="/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${AZURE_RESOURCE_GROUP_NAME}/providers/Microsoft.Compute/snapshots/${ARM64_OS_DISK_SNAPSHOT_NAME}"
az snapshot update --resource-group ${AZURE_RESOURCE_GROUP_NAME} -n ${ARM64_OS_DISK_SNAPSHOT_NAME} --architecture Arm64
az sig image-version create --location $AZURE_LOCATION --resource-group ${AZURE_RESOURCE_GROUP_NAME} --gallery-name PackerSigGalleryEastUS \
     --gallery-image-definition ${SIG_IMAGE_NAME} --gallery-image-version ${CAPTURED_SIG_VERSION} \
     --os-snapshot ${disk_snapshot_id}
