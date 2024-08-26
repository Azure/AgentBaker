#!/bin/bash -e

required_env_vars=(
    "AZURE_MSI_RESOURCE_STRING"
    "SUBSCRIPTION_ID"
    "RESOURCE_GROUP_NAME"
    "CAPTURED_SIG_VERSION"
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

if [ -z "$PACKER_BUILD_LOCATION" ]; then
  echo "PACKER_BUILD_LOCATION must be set for linux builds"
  exit 1
fi
LOCATION=$PACKER_BUILD_LOCATION

if [[ -z "$SIG_GALLERY_NAME" ]]; then
  SIG_GALLERY_NAME="PackerSigGalleryEastUS"
fi

echo "SIG_IMAGE_VERSION before checking and assigning is $SIG_IMAGE_VERSION"
# Linux Gen 2: assign $CAPTURED_SIG_VERSION to $SIG_IMAGE_VERSION
if [[ -z "$SIG_IMAGE_VERSION" ]]; then
  SIG_IMAGE_VERSION=${CAPTURED_SIG_VERSION}
fi

SNAPSHOT_UPLOAD_RESOURCE_GROUP_NAME="UPLOAD_RG-$(date +%s)-$RANDOM"
if [ -z "$SNAPSHOT_UPLOAD_RESOURCE_GROUP_NAME" ]; then
  echo "SNAPSHOT_UPLOAD_RESOURCE_GROUP_NAME could not be passed successfully."
  exit 1
fi
az group create --name $SNAPSHOT_UPLOAD_RESOURCE_GROUP_NAME --location ${AZURE_LOCATION} --tags "source=AgentBaker" "branch=${GIT_BRANCH}"

# defer function to cleanup resource group when VHD debug is not enabled
function cleanup() {
  echo "Deleting resource group ${SNAPSHOT_UPLOAD_RESOURCE_GROUP_NAME}"
  az group delete --name $SNAPSHOT_UPLOAD_RESOURCE_GROUP_NAME --yes --no-wait
}
trap cleanup EXIT

az vm create \
    --resource-group $TEST_VM_RESOURCE_GROUP_NAME \
    --name $VM_NAME \
    --image $IMG_DEF \
    --admin-username $TEST_VM_ADMIN_USERNAME \
    --admin-password $TEST_VM_ADMIN_PASSWORD \
    --public-ip-address "" \
    ${TARGET_COMMAND_STRING}
    
echo "VHD test VM username: $TEST_VM_ADMIN_USERNAME, password: $TEST_VM_ADMIN_PASSWORD"


time az vm wait -g $TEST_VM_RESOURCE_GROUP_NAME -n $VM_NAME --created