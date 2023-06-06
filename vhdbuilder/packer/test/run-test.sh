#!/bin/bash
set -eux
LINUX_SCRIPT_PATH="linux-vhd-content-test.sh"
WIN_CONFIGURATION_SCRIPT_PATH="generate-windows-vhd-configuration.ps1"
WIN_SCRIPT_PATH="windows-vhd-content-test.ps1"
TEST_RESOURCE_PREFIX="vhd-test"
TEST_VM_ADMIN_USERNAME="azureuser"

set +x
TEST_VM_ADMIN_PASSWORD="TestVM@$(date +%s)"
set -x

if [ "$OS_TYPE" == "Linux" ]; then
  if [ "$IMG_SKU" == "20_04-lts-cvm" ] || [ "$OS_VERSION" == "V1" ] && [ "$OS_SKU" == "CBLMariner" ]; then
    echo "Skipping tests for CVM 20.04 and Mariner 1.0"
    exit 0
  fi
fi

RESOURCE_GROUP_NAME="$TEST_RESOURCE_PREFIX-$(date +%s)-$RANDOM"
az group create --name $RESOURCE_GROUP_NAME --location ${AZURE_LOCATION} --tags 'source=AgentBaker'

# These will be set later when we're about to run the tests. The BLOB_URI variables are set only upon creating
# the blobs, which means we can use them to determine whether the blobs have been created yet and clean them up.
STDOUT_BLOB_NAME=''
STDERR_BLOB_NAME=''
STDOUT_BLOB_URI=''
STDERR_BLOB_URI=''

# defer function to cleanup resource group when VHD debug is not enabled
function cleanup() {
  if [[ "$VHD_DEBUG" == "True" ]]; then
    echo "VHD debug mode is enabled, please manually delete test vm resource group $RESOURCE_GROUP_NAME after debugging"
  else
    echo "Deleting resource group ${RESOURCE_GROUP_NAME}"
    az group delete --name $RESOURCE_GROUP_NAME --yes --no-wait

    if [[ -n "$STDOUT_BLOB_URI" ]]; then
      echo "Deleting stdout blob ${STDOUT_BLOB_NAME}"
      az storage blob delete \
        --account-name "${OUTPUT_STORAGE_ACCOUNT_NAME}" \
        --container-name "${OUTPUT_STORAGE_CONTAINER_NAME}" \
        --connection-string "${CLASSIC_SA_CONNECTION_STRING}" \
        --name "${STDOUT_BLOB_NAME}" \
        --output json
    fi

    if [[ -n "$STDERR_BLOB_URI" ]]; then
      echo "Deleting stderr blob ${STDERR_BLOB_NAME}"
      az storage blob delete \
        --account-name "${OUTPUT_STORAGE_ACCOUNT_NAME}" \
        --container-name "${OUTPUT_STORAGE_CONTAINER_NAME}" \
        --connection-string "${CLASSIC_SA_CONNECTION_STRING}" \
        --name "${STDERR_BLOB_NAME}" \
        --output json
    fi
  fi
}
trap cleanup EXIT

DISK_NAME="${TEST_RESOURCE_PREFIX}-disk"
VM_NAME="${TEST_RESOURCE_PREFIX}-vm"

if [ "$MODE" == "default" ]; then
  az disk create --resource-group $RESOURCE_GROUP_NAME \
    --name $DISK_NAME \
    --source "${OS_DISK_URI}" \
    --query id
  az vm create --name $VM_NAME \
    --resource-group $RESOURCE_GROUP_NAME \
    --attach-os-disk $DISK_NAME \
    --os-type $OS_TYPE \
    --public-ip-address ""
else 
  if [ "$MODE" == "sigMode" ]; then
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
  fi

  if [ -z "${MANAGED_SIG_ID}" ]; then
    echo "Managed Sig Id from packer-output is empty, unable to proceed..."
    exit 1
  else
    echo "Managed Sig Id from packer-output is ${MANAGED_SIG_ID}"
    IMG_DEF=${MANAGED_SIG_ID}
  fi

  # In SIG mode, Windows VM requires admin-username and admin-password to be set,
  # otherwise 'root' is used by default but not allowed by the Windows Image. See the error image below:
  # ERROR: This user name 'root' meets the general requirements, but is specifically disallowed for this image. Please try a different value.
  TARGET_COMMAND_STRING=""
  if [[ "${ARCHITECTURE,,}" == "arm64" ]]; then
    TARGET_COMMAND_STRING+="--size Standard_D2pds_v5"
  elif [[ "${FEATURE_FLAGS,,}" == "kata" ]]; then
    TARGET_COMMAND_STRING="--size Standard_D4ds_v5"
  fi

  if [[ "${OS_TYPE}" == "Linux" && "${ENABLE_TRUSTED_LAUNCH}" == "True" ]]; then
    if [[ -n "$TARGET_COMMAND_STRING" ]]; then
      # To take care of Mariner Kata TL images
      TARGET_COMMAND_STRING+=" "
    fi
    TARGET_COMMAND_STRING+="--security-type TrustedLaunch --enable-secure-boot true --enable-vtpm true"
  fi

  az vm create \
      --resource-group $RESOURCE_GROUP_NAME \
      --name $VM_NAME \
      --image $IMG_DEF \
      --admin-username $TEST_VM_ADMIN_USERNAME \
      --admin-password $TEST_VM_ADMIN_PASSWORD \
      --public-ip-address "" \
      ${TARGET_COMMAND_STRING}
      
  echo "VHD test VM username: $TEST_VM_ADMIN_USERNAME, password: $TEST_VM_ADMIN_PASSWORD"
fi

time az vm wait -g $RESOURCE_GROUP_NAME -n $VM_NAME --created

FULL_PATH=$(realpath $0)
CDIR=$(dirname $FULL_PATH)

if [ "$OS_TYPE" == "Linux" ]; then
  if [[ -z "${ENABLE_FIPS// }" ]]; then
    ENABLE_FIPS="false"
  fi


  # Replace dots with dashes and make sure we only have the file name of the test script.
  # This will be used to name azure resources related to the test.
  LINUX_SCRIPT_FILE_NAME_NO_DOTS=$(basename "${LINUX_SCRIPT_PATH//./-}")

  # Create blob storage for the test stdout and stderr. This allows us to get all output, not just
  # the first 4KB each of stdout/stderr.
  STDOUT_BLOB_NAME="${RESOURCE_GROUP_NAME}-${VM_NAME}-${LINUX_SCRIPT_FILE_NAME_NO_DOTS}-stdout.txt"
  STDERR_BLOB_NAME="${RESOURCE_GROUP_NAME}-${VM_NAME}-${LINUX_SCRIPT_FILE_NAME_NO_DOTS}-stderr.txt"
  SAS_EXPIRY=$(date -u -d "60 minutes" '+%Y-%m-%dT%H:%MZ')
  STDOUT_BLOB_URI=$(az storage blob generate-sas \
    --account-name "${OUTPUT_STORAGE_ACCOUNT_NAME}" \
    --container-name "${OUTPUT_STORAGE_CONTAINER_NAME}" \
    --connection-string "${CLASSIC_SA_CONNECTION_STRING}" \
    --name "${STDOUT_BLOB_NAME}" \
    --permissions acrw \
    --expiry "${SAS_EXPIRY}"\
    --full-uri --output tsv)
  STDERR_BLOB_URI=$(az storage blob generate-sas \
    --account-name "${OUTPUT_STORAGE_ACCOUNT_NAME}" \
    --container-name "${OUTPUT_STORAGE_CONTAINER_NAME}" \
    --connection-string "${CLASSIC_SA_CONNECTION_STRING}" \
    --name "${STDERR_BLOB_NAME}" \
    --permissions acrw \
    --expiry "${SAS_EXPIRY}" \
    --full-uri --output tsv)

  # Start the test script on the VM and wait for it to complete.
  # In testing, I've found that creating the script with --no-wait and then waiting for it
  # is nore reliable than waiting on the initial create command.
  COMMAND_NAME="${LINUX_SCRIPT_FILE_NAME_NO_DOTS}-command"
  SCRIPT_PATH="$CDIR/$LINUX_SCRIPT_PATH"
  az vm run-command create \
    --resource-group "${RESOURCE_GROUP_NAME}" \
    --vm-name "${VM_NAME}" \
    --name "${COMMAND_NAME}" \
    --script @"${SCRIPT_PATH}" \
    --parameters "CONTAINER_RUNTIME=${CONTAINER_RUNTIME}" "OS_VERSION=${OS_VERSION}" "ENABLE_FIPS=${ENABLE_FIPS}" "OS_SKU=${OS_SKU}" \
    --output json \
    --output-blob-uri "${STDOUT_BLOB_URI}" \
    --error-blob-uri "${STDERR_BLOB_URI}" \
    --no-wait
  az vm run-command wait \
    --resource-group "${RESOURCE_GROUP_NAME}" \
    --vm-name "${VM_NAME}" \
    --name "${COMMAND_NAME}" \
    --instance-view \
    --custom 'instanceView.endTime != null' \
    --output json

  # Get the data associated with the command, collecting the exit code
  # and execution state. Dump the whole thing.
  command_data=$(az vm run-command show \
    --resource-group "${RESOURCE_GROUP_NAME}" \
    --vm-name "${VM_NAME}" \
    --name "${COMMAND_NAME}" \
    --instance-view \
    --output json)
  command_exit_code=$(echo "${command_data}" | jq '.instanceView.exitCode')
  command_execution_state=$(echo "${command_data}" | jq '.instanceView.executionState')
  echo "TEST EXIT CODE: ${command_exit_code}"
  echo "TEST EXECUTION STATE: ${command_execution_state}"

  # Get our stdout from the blob storage.
  az storage blob download \
    --account-name "${OUTPUT_STORAGE_ACCOUNT_NAME}" \
    --container-name "${OUTPUT_STORAGE_CONTAINER_NAME}" \
    --connection-string "${CLASSIC_SA_CONNECTION_STRING}" \
    --name "${STDOUT_BLOB_NAME}" \
    --file "./${STDOUT_BLOB_NAME}"
  echo "TEST STDOUT:"
  echo "============"
  cat "./${STDOUT_BLOB_NAME}"
  echo "============"

  # Get our stderr from the blob storage, collecting it in a variable
  # for later inspection.
  az storage blob download \
    --account-name "${OUTPUT_STORAGE_ACCOUNT_NAME}" \
    --container-name "${OUTPUT_STORAGE_CONTAINER_NAME}" \
    --connection-string "${CLASSIC_SA_CONNECTION_STRING}" \
    --name "${STDERR_BLOB_NAME}" \
    --file "./${STDERR_BLOB_NAME}"
  echo "TEST STDERR:"
  echo "============"
  cat "./${STDERR_BLOB_NAME}"
  echo "============"

  # A failure occurs if any of the following three happens:
  #   1. The command execution state is not "Succeeded".
  #   2. The command exit code is not 0.
  #   3. The stderr is not empty.
  if [ "${command_execution_state}" != '"Succeeded"' ] || [ "${command_exit_code}" != "0" ] || [ -s "./${STDERR_BLOB_NAME}" ]; then
    echo "TEST FAILED: See about output for details."
    exit 1
  fi
else
  SCRIPT_PATH="$CDIR/../$WIN_CONFIGURATION_SCRIPT_PATH"
  echo "Run $SCRIPT_PATH"
  az vm run-command invoke --command-id RunPowerShellScript \
    --name $VM_NAME \
    --resource-group $RESOURCE_GROUP_NAME \
    --scripts @$SCRIPT_PATH \
    --output json

  SCRIPT_PATH="$CDIR/$WIN_SCRIPT_PATH"
  echo "Run $SCRIPT_PATH"
  ret=$(az vm run-command invoke --command-id RunPowerShellScript \
    --name $VM_NAME \
    --resource-group $RESOURCE_GROUP_NAME \
    --scripts @$SCRIPT_PATH \
    --output json \
    --parameters "containerRuntime=${CONTAINER_RUNTIME}" "windowsSKU=${WINDOWS_SKU}")
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