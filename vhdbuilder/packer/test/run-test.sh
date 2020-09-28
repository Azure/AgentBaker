#!/bin/bash
set -eux

WIN_SCRIPT_PATH="vhd-content-test.ps1"
TEST_RESOURCE_PREFIX="vhd-test"

RESOURCE_GROUP_NAME="$TEST_RESOURCE_PREFIX-$(date +%s)"
az group create --name $RESOURCE_GROUP_NAME --location ${AZURE_LOCATION} --tags 'source=AgentBaker'

# defer function to cleanup resource group
function cleanup {
    az group delete --name $RESOURCE_GROUP_NAME --yes --no-wait
}
trap cleanup EXIT

DISK_NAME="${TEST_RESOURCE_PREFIX}-disk"
VM_NAME="${TEST_RESOURCE_PREFIX}-vm"
az disk create --resource-group $RESOURCE_GROUP_NAME \
    --name $DISK_NAME \
    --source "${OS_DISK_URI}"  \
    --query id
az vm create --name $VM_NAME \
    --resource-group $RESOURCE_GROUP_NAME \
    --attach-os-disk $DISK_NAME \
    --os-type $OS_TYPE  \
    --public-ip-address ""
time az vm wait -g $RESOURCE_GROUP_NAME -n $VM_NAME --created

FULL_PATH=$(realpath $0)
CDIR=$(dirname $FULL_PATH)
if [ "$OS_TYPE" == "Windows" ]; then
    SCRIPT_PATH="$CDIR/$WIN_SCRIPT_PATH"
    ret=$(az vm run-command invoke --command-id RunPowerShellScript \
        --name $VM_NAME \
        --resource-group $RESOURCE_GROUP_NAME  \
        --scripts  @$SCRIPT_PATH \
        --output json \
        --parameters "containerRuntime=${CONTAINER_RUNTIME}" "WindowsSKU=${WINDOWS_SKU}")
fi
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
