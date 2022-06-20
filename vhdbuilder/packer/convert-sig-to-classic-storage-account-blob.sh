#!/bin/bash -e

required_env_vars=(
    "SUBSCRIPTION_ID"
    "RESOURCE_GROUP_NAME"
    "GEN2_CAPTURED_SIG_VERSION"
    "LOCATION"
    "OS_TYPE"
    "SIG_IMAGE_NAME"
)


for v in "${required_env_vars[@]}"
do
    if [ -z "${!v}" ]; then
        echo "$v was not set!"
        exit 1
    fi
done

sig_resource_id="/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${RESOURCE_GROUP_NAME}/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/${SIG_IMAGE_NAME}/versions/${GEN2_CAPTURED_SIG_VERSION}"
disk_resource_id="/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${RESOURCE_GROUP_NAME}/providers/Microsoft.Compute/disks/${GEN2_CAPTURED_SIG_VERSION}"

az resource create --id $disk_resource_id  --is-full-object --location $LOCATION --properties "{\"location\": \"$LOCATION\", \
  \"properties\": { \
    \"osType\": \"$OS_TYPE\", \
    \"creationData\": { \
      \"createOption\": \"FromImage\", \
      \"galleryImageReference\": { \
        \"id\": \"${sig_resource_id}\" \
      } \
    } \
  } \
}"
# shellcheck disable=SC2102
sas=$(az disk grant-access --ids $disk_resource_id --duration-in-seconds 3600 --query [accessSas] -o tsv)

echo "Before coping"
az image show -n 2019-containerd -g aksvhdbuilderrg
az image show -n 2022-containerd -g aksvhdbuilderrg
azcopy-preview copy "${sas}" "${CLASSIC_BLOB}/1.0.${CREATE_TIME}.vhd${CLASSIC_SAS_TOKEN}" --recursive=true

echo "Converted $sig_resource_id to ${CLASSIC_BLOB}/1.0.${CREATE_TIME}.vhd"
echo "After converting and before revoking access"
az image show -n 2019-containerd -g aksvhdbuilderrg
az image show -n 2022-containerd -g aksvhdbuilderrg

azcopy-preview copy "${sas}" "${CLASSIC_BLOB}/${GEN2_CAPTURED_SIG_VERSION}.vhd${CLASSIC_SAS_TOKEN}" --recursive=true

echo "Converted $sig_resource_id to ${CLASSIC_BLOB}/${GEN2_CAPTURED_SIG_VERSION}.vhd"

az disk revoke-access --ids $disk_resource_id 

az resource delete --ids $disk_resource_id
echo "After deleting disk resource"
az image show -n 2019-containerd -g aksvhdbuilderrg
az image show -n 2022-containerd -g aksvhdbuilderrg
echo "Deleted $disk_resource_id"