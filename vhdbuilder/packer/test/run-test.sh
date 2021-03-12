#!/bin/bash
set -eux

LINUX_SCRIPT_PATH="linux-vhd-content-test.sh"
WIN_SCRIPT_PATH="windows-vhd-content-test.ps1"
TEST_RESOURCE_PREFIX="vhd-test"
TEST_VM_ADMIN_USERNAME="azureuser"
TEST_VM_ADMIN_PASSWORD="TestVM@$(date +%s)"

RESOURCE_GROUP_NAME="$TEST_RESOURCE_PREFIX-$(date +%s)"
az group create --name $RESOURCE_GROUP_NAME --location ${AZURE_LOCATION} --tags 'source=AgentBaker'

# defer function to cleanup resource group when VHD debug is not enabled
function cleanup() {
  if [ "$VHD_DEBUG" == "true" ]; then
    echo "VHD debug mode is enabled, please manually delete test vm resource group $RESOURCE_GROUP_NAME after debugging"
  else
    az group delete --name $RESOURCE_GROUP_NAME --yes --no-wait
  fi
}
trap cleanup EXIT

DISK_NAME="${TEST_RESOURCE_PREFIX}-disk"
VM_NAME="${TEST_RESOURCE_PREFIX}-vm"

if [ "$MODE" == "sigMode" ]; then
  echo "SIG existence checking for $MODE"
  id=$(az sig show --resource-group ${AZURE_RESOURCE_GROUP_NAME} --gallery-name ${SIG_GALLERY_NAME}) || id=""
  if [ -z "$id" ]; then
    echo "Shared Image gallery ${SIG_GALLERY_NAME} does not exist in the resource group ${AZURE_RESOURCE_GROUP_NAME} location ${AZURE_LOCATION}"
    exit 1
  fi

  id=$(az sig image-definition show \
    --resource-group ${AZURE_RESOURCE_GROUP_NAME} \
    --gallery-name ${SIG_GALLERY_NAME} \
    --gallery-image-definition ${SIG_IMAGE_NAME}) || id=""
  if [ -z "$id" ]; then
    echo "Image definition ${SIG_IMAGE_NAME} does not exist in gallery ${SIG_GALLERY_NAME} resource group ${AZURE_RESOURCE_GROUP_NAME}"
    exit 1
  fi

  IMG_DEF="/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${AZURE_RESOURCE_GROUP_NAME}/providers/Microsoft.Compute/galleries/${SIG_GALLERY_NAME}/images/${SIG_IMAGE_NAME}/versions/${SIG_IMAGE_VERSION}"

  # In SIG mode, Windows VM requires admin-username and admin-password to be set,
  # otherwise 'root' is used by default but not allowed by the Windows Image. See the error image below:
  # ERROR: This user name 'root' meets the general requirements, but is specifically disallowed for this image. Please try a different value.
  az vm create \
    --resource-group $RESOURCE_GROUP_NAME \
    --name $VM_NAME \
    --image $IMG_DEF \
    --admin-username $TEST_VM_ADMIN_USERNAME \
    --admin-password $TEST_VM_ADMIN_PASSWORD \
    --public-ip-address ""
  echo "VHD test VM username: $TEST_VM_ADMIN_USERNAME, password: $TEST_VM_ADMIN_PASSWORD"

else
  az disk create --resource-group $RESOURCE_GROUP_NAME \
    --name $DISK_NAME \
    --source "${OS_DISK_URI}" \
    --query id
  az vm create --name $VM_NAME \
    --resource-group $RESOURCE_GROUP_NAME \
    --attach-os-disk $DISK_NAME \
    --os-type $OS_TYPE \
    --public-ip-address ""
fi

time az vm wait -g $RESOURCE_GROUP_NAME -n $VM_NAME --created

FULL_PATH=$(realpath $0)
CDIR=$(dirname $FULL_PATH)

if [ "$OS_TYPE" == "Linux" ]; then
  if [[ -z "${ENABLE_FIPS// }" ]]; then
    ENABLE_FIPS="false"
  fi

  SCRIPT_PATH="$CDIR/$LINUX_SCRIPT_PATH"
  for i in $(seq 1 3); do
    ret=$(az vm run-command invoke --command-id RunShellScript \
      --name $VM_NAME \
      --resource-group $RESOURCE_GROUP_NAME \
      --scripts @$SCRIPT_PATH \
      --parameters ${CONTAINER_RUNTIME} ${OS_VERSION} ${ENABLE_FIPS}) && break
    echo "${i}: retrying az vm run-command"
  done
  # The error message for a Linux VM run-command is as follows:
  #  "value": [
  #    {
  #      "code": "ProvisioningState/succeeded",
  #      "displayStatus": "Provisioning succeeded",
  #      "level": "Info",
  #      "message": "Enable succeeded: \n[stdout]\n\n[stderr]\ntestImagesPulled:Error: Image mcr.microsoft.com/azure-policy/policy-kubernetes-addon-prod:prod_20201015.1 has NOT been pulled
  # \n",
  #      "time": null
  #    }
  #  ]
  #  We have extract the message field from the json, and get the errors outputted to stderr + remove \n
  errMsg=$(echo -e $(echo $ret | jq ".value[] | .message" | grep -oP '(?<=stderr]).*(?=\\n")'))
  echo $errMsg
  if [[ $errMsg != '' ]]; then
    exit 1
  fi
else
  SCRIPT_PATH="$CDIR/$WIN_SCRIPT_PATH"
  ret=$(az vm run-command invoke --command-id RunPowerShellScript \
    --name $VM_NAME \
    --resource-group $RESOURCE_GROUP_NAME \
    --scripts @$SCRIPT_PATH \
    --output json \
    --parameters "containerRuntime=${CONTAINER_RUNTIME}" "WindowsSKU=${WINDOWS_SKU}")
  # An example of failed run-command output:
  # {
  #   "value": [
  #     {
  #       "code": "ComponentStatus/StdOut/succeeded",
  #       "displayStatus": "Provisioning succeeded",
  #       "level": "Info",
  #       "message": "c:\akse-cache\containerd\containerd-0.0.87-public.zip is cached as expected
  # c:\akse-cache\win-vnet-cni\azure-vnet-cni-singletenancy-windows-amd64-v1.1.2.zip is cached as expected
  # ... ...
  # "
  #       "time": null
  #     },
  #     {
  #       "code": "ComponentStatus/StdErr/succeeded",
  #       "displayStatus": "Provisioning succeeded",
  #       "level": "Info",
  #       "message": "Test-FilesToCacheOnVHD : File c:\akse-cache\win-k8s\v1.15.10-azs-1int.zip does not exist
  # At C:\Packages\Plugins\Microsoft.CPlat.Core.RunCommandWindows\1.1.5\Downloads\script0.ps1:146 char:1
  # + Test-FilesToCacheOnVHD
  # + ~~~~~~~~~~~~~~~~~~~~~~
  #     + CategoryInfo          : NotSpecified: (:) [Write-Error], WriteErrorException
  #     + FullyQualifiedErrorId : Microsoft.PowerShell.Commands.WriteErrorException,Test-FilesToCacheOnVHD
  #  ",
  #       "time": null
  #     }
  #   ]
  # }
  # we have to use `-E` to disable interpretation of backslash escape sequences, for jq cannot process string
  # with a range of control characters not escaped as shown in the error below:
  #   Invalid string: control characters from U+0000 through U+001F must be escaped
  errMsg=$(echo -E $ret | jq '.value[]  | select(.code == "ComponentStatus/StdErr/succeeded") | .message')
  # a successful errMsg should be '""' after parsed by `jq`
  if [[ $errMsg != \"\" ]]; then
    exit 1
  fi
fi

echo "Tests Run Successfully"
