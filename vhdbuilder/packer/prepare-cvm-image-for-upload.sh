#!/bin/bash -e

if [ -z "$PACKER_BUILD_LOCATION" ]; then
  echo "PACKER_BUILD_LOCATION must be set."
  exit 1
fi

if [[ -z "$SIG_GALLERY_NAME" ]]; then
  SIG_GALLERY_NAME="PackerSigGalleryEastUS"
fi

if [[ -z "$SIG_IMAGE_VERSION" ]]; then
  SIG_IMAGE_VERSION=${CAPTURED_SIG_VERSION}
fi

CVM_IMAGE_RG="CVM_IMAGE_RG-$(date +%s)-$RANDOM"
if [ -z "$CVM_IMAGE_RG" ]; then
  echo "CVM_IMAGE_RG could not be passed successfully."
  exit 1
fi
az group create --name $CVM_IMAGE_RG --location ${PACKER_BUILD_LOCATION} --tags "source=AgentBaker" "branch=${GIT_BRANCH}"

# defer function to cleanup resource group
function cleanup() {
  echo "Deleting resource group ${CVM_IMAGE_RG}"
  az group delete --name $CVM_IMAGE_RG --yes --no-wait
}
trap cleanup EXIT

TEST_VM_ADMIN_USERNAME="azureuser"
set +x
TEST_VM_ADMIN_PASSWORD="cvmPrepVM@$(date +%s)"
set -x

DISK_NAME="cvmPrepDisk"
VM_NAME="cvmPrepVM"

if [[ "${OS_TYPE}" == "Linux" && "${IMG_SKU}" == "20_04-lts-cvm" ]]; then
  TARGET_COMMAND_STRING="--size Standard_EC16ads_v5"
  TARGET_COMMAND_STRING+=" --security-type ConfidentialVM --enable-secure-boot true --enable-vtpm true --os-disk-security-encryption-type VMGuestStateOnly --specialized"
fi

az vm create \
  --resource-group $CVM_IMAGE_RG \
  --name $VM_NAME \
  --image $MANAGED_SIG_ID \
  --admin-username $TEST_VM_ADMIN_USERNAME \
  --admin-password $TEST_VM_ADMIN_PASSWORD \
  --public-ip-address "" \
  ${TARGET_COMMAND_STRING}
    
az vm wait -g $CVM_IMAGE_RG -n $VM_NAME --created

ret=$(az vm run-command invoke --command-id RunShellScript \
  --name $VM_NAME \
  --resource-group $TEST_VM_RESOURCE_GROUP_NAME \
  --scripts "sudo waagent -force -deprovision+user" \
    "sudo rm -f ~/.bash_history")

errMsg=$(echo -e $(echo $ret | jq ".value[] | .message" | grep -oP '(?<=stderr]).*(?=\\n")'))
echo $errMsg
if [[ $errMsg != '' ]]; then
  echo -e "\nFailed to generalize CVM image.\n"
  exit 1
fi

az vm deallocate --resource-group $CVM_IMAGE_RG --name $VM_NAME

az vm generalize --resource-group $CVM_IMAGE_RG --name $VM_NAME

az sig image-version create \
  --resource-group $AZURE_RESOURCE_GROUP_NAME \
  --gallery-name $SIG_GALLERY_NAME \
  --gallery-image-definition $SKU_NAME \
  --gallery-image-version $SIG_IMAGE_VERSION \
  --virtual-machine /subscriptions/$SUBSCRIPTION_ID/resourceGroups/$CVM_IMAGE_RG/providers/Microsoft.Compute/virtualMachines/$VM_NAME