#!/bin/bash
set -e

echo "Installing previous version of azcli in order to mitigate az compute bug"
source parts/linux/cloud-init/artifacts/ubuntu/cse_helpers_ubuntu.sh

SCRIPT_DIR=$(dirname "$0")
source "$SCRIPT_DIR/produce-packer-settings-functions.sh"

if [ -n "${AZCLI_VERSION_OVERRIDE}" ]; then
  echo "Overriding azcli version to ${AZCLI_VERSION_OVERRIDE}"
  enforce_azcli_version "${AZCLI_VERSION_OVERRIDE}"
fi

CDIR=$(dirname "${BASH_SOURCE}")
SETTINGS_JSON="${SETTINGS_JSON:-./packer/settings.json}"
PUBLISHER_BASE_IMAGE_VERSION_JSON="${PUBLISHER_BASE_IMAGE_VERSION_JSON:-./vhdbuilder/publisher_base_image_version.json}"
VHD_BUILD_TIMESTAMP_JSON="${VHD_BUILD_TIMESTAMP_JSON:-./vhdbuilder/vhd_build_timestamp.json}"
SUBSCRIPTION_ID="${SUBSCRIPTION_ID:-$(az account show -o json --query="id" | tr -d '"')}"
GALLERY_SUBSCRIPTION_ID="${GALLERY_SUBSCRIPTION_ID:-${SUBSCRIPTION_ID}}"
CREATE_TIME="$(date +%s)"

# This variable will only be set if a VHD build is triggered from an official branch
VHD_BUILD_TIMESTAMP=""

# Check if the file exists, if it does, the build is triggered from an official branch
if [ -f "${PUBLISHER_BASE_IMAGE_VERSION_JSON}" ]; then
  # Ensure that the file is not empty, this will never happen since automation generates the file after each build but still have this check in place
  if [ -s "${PUBLISHER_BASE_IMAGE_VERSION_JSON}" ]; then
    # For IMG_SKUs that dont exist in the file, this is a no-op, therefore Windows/Mariner wont be affected and their IMG_VERSION will always be 'latest'
    echo "The publisher_base_image_version.json is not empty, therefore, use the publisher base images specified there, if they exist"
    PUBLISHER_BASE_IMAGE_VERSION=$(jq -r --arg key "${IMG_SKU}" 'if has($key) then .[$key] else empty end' "${PUBLISHER_BASE_IMAGE_VERSION_JSON}")
    if [ -n "${PUBLISHER_BASE_IMAGE_VERSION}" ]; then
      echo "Change publisher base image version to ${PUBLISHER_BASE_IMAGE_VERSION} for ${IMG_SKU}"
      IMG_VERSION=${PUBLISHER_BASE_IMAGE_VERSION}
    fi
  fi
fi

# Check if the file exists, if it does, the build is triggered from an official branch
if [ -f "${VHD_BUILD_TIMESTAMP_JSON}" ]; then
  # Ensure that the file is not empty, this will never happen since automation generates the file after each build but still have this check in place
  if [ -s "${VHD_BUILD_TIMESTAMP_JSON}" ]; then
    VHD_BUILD_TIMESTAMP=$(jq -r .build_timestamp < ${VHD_BUILD_TIMESTAMP_JSON})
  fi
fi

# Hard-code RG/gallery location to 'eastus' only for linux builds.
if [ "$MODE" = "linuxVhdMode" ]; then
	# In linux builds, this variable is only used for creating the resource group holding the
	# staging "PackerSigGalleryEastUS" SIG, as well as the gallery itself. It's also used
	# for creating any image definitions that might be missing from the gallery based on the particular
	# SKU being built.
	#
	# For windows, this variable is also used for creating resources to import base images
	AZURE_LOCATION="eastus"
fi

# We use the provided SIG_IMAGE_VERSION if it's instantiated and we're running linuxVhdMode, otherwise we randomly generate one
if [ "${MODE}" = "linuxVhdMode" ] && [ -n "${SIG_IMAGE_VERSION}" ]; then
	CAPTURED_SIG_VERSION=${SIG_IMAGE_VERSION}
else
	CAPTURED_SIG_VERSION="1.${CREATE_TIME}.$RANDOM"
fi

if [ -z "${POOL_NAME}" ]; then
	echo "POOL_NAME is not set, can't compute VNET_RG_NAME for packer templates"
	exit 1
fi

echo "POOL_NAME is set to $POOL_NAME"

if [ "$MODE" = "linuxVhdMode" ] && [ -z "${SKU_NAME}" ]; then
	echo "SKU_NAME must be set for linux VHD builds"
	exit 1
fi

# This variable is used within linux builds to inform which region that packer build itself will be running,
# and subsequently the region in which the 1ES pool the build is running on is in.
# Note that this variable is ONLY used for linux builds, windows builds simply use AZURE_LOCATION.
if  [ -z "${PACKER_BUILD_LOCATION}" ]; then
	echo "PACKER_BUILD_LOCATION is not set, cannot compute VNET_RG_NAME for packer templates"
	exit 1
fi

if grep -q "cvm" <<< "$FEATURE_FLAGS" && [ -n "${CVM_PACKER_BUILD_LOCATION}" ]; then
	PACKER_BUILD_LOCATION="${CVM_PACKER_BUILD_LOCATION}"
	echo "CVM: PACKER_BUILD_LOCATION is set to ${PACKER_BUILD_LOCATION}"
fi

# GB200 specific build location handling (if needed in future)
if grep -q "GB200" <<< "$FEATURE_FLAGS"; then
	echo "GB200: Using standard ARM64 build location ${PACKER_BUILD_LOCATION}"
	# Additional GB200-specific configuration can be added here
fi

# Currently only used for linux builds. This determines the environment in which the build is running (either prod or test).
# Used to construct the name of the resource group in which the 1ES pool the build is running on lives in, which also happens.
# to be the resource group in which the packer VNET lives in.
if [ -z "${ENVIRONMENT}" ]; then
	echo "ENVIRONMENT is not set, cannot compute VNET_RG_NAME or VNET_NAME for packer templates"
	exit 1
fi

if [ -z "${VNET_RG_NAME}" ]; then
  if [ "${ENVIRONMENT,,}" = "prod" ]; then
    # TODO(cameissner): build out updated pool resources in prod so we don't have to pivot like this
    VNET_RG_NAME="nodesig-${ENVIRONMENT}-${PACKER_BUILD_LOCATION}-agent-pool"
  else
    VNET_RG_NAME="nodesig-${ENVIRONMENT}-${PACKER_BUILD_LOCATION}-packer-vnet-rg"
  fi
fi

if [ -z "${VNET_NAME}" ]; then
  if [ "${ENVIRONMENT,,}" = "prod" ]; then
    # TODO(cameissner): build out updated pool resources in prod so we don't have to pivot like this
    VNET_NAME="nodesig-pool-vnet-${PACKER_BUILD_LOCATION}"
  else
    VNET_NAME="nodesig-packer-vnet-${PACKER_BUILD_LOCATION}"
  fi
fi

if [ -z "${SUBNET_NAME}" ]; then
	SUBNET_NAME="packer"
fi

echo "VNET_RG_NAME set to: ${VNET_RG_NAME}"

echo "CAPTURED_SIG_VERSION set to: ${CAPTURED_SIG_VERSION}"

echo "Subscription ID: ${SUBSCRIPTION_ID}"
echo "Gallery Subscription ID: ${GALLERY_SUBSCRIPTION_ID}"

rg_id=$(az group show --name $AZURE_RESOURCE_GROUP_NAME) || rg_id=""
if [ -z "$rg_id" ]; then
	echo "Creating resource group $AZURE_RESOURCE_GROUP_NAME, location ${AZURE_LOCATION}"
	az group create --name $AZURE_RESOURCE_GROUP_NAME --location ${AZURE_LOCATION}
fi

# If SIG_GALLERY_NAME/SIG_IMAGE_NAME hasnt been provided in linuxVhdMode, use defaults
# NOTE: SIG_IMAGE_NAME is the name of the image definition that Packer will use when delivering the
# output image version to the staging gallery. This is NOT the name of the image definitions used in prod.
if [ "${MODE}" = "linuxVhdMode" ]; then
  ensure_sig_image_name_linux

fi

# shellcheck disable=SC3010
if [[ "${MODE}" == "windowsVhdMode" ]] && [[ ${ARCHITECTURE,,} == "arm64" ]]; then
	# only append 'Arm64' in windows builds, for linux we either take what was provided
	# or base the name off the the value of SKU_NAME (see above)
  SIG_IMAGE_NAME=${SIG_IMAGE_NAME//./}Arm64
fi

echo "Using finalized SIG_IMAGE_NAME: ${SIG_IMAGE_NAME}, SIG_GALLERY_NAME: ${SIG_GALLERY_NAME}"

if [ "${SUBSCRIPTION_ID}" != "${GALLERY_SUBSCRIPTION_ID}" ]; then
    az account set -s "${GALLERY_SUBSCRIPTION_ID}"
fi

# If we're building a Linux VHD or we're building a windows VHD in windowsVhdMode, ensure SIG resources
if [ "$MODE" = "linuxVhdMode" ] || [ "$MODE" = "windowsVhdMode" ]; then
  ensure_sig_vhd_exists
else
	echo "Skipping SIG check for $MODE, os-type: ${OS_TYPE}"
fi

if [ "${SUBSCRIPTION_ID}" != "${GALLERY_SUBSCRIPTION_ID}" ]; then
    az account set -s "${SUBSCRIPTION_ID}"
fi

# considerations to also add the windows support here instead of an extra script to initialize windows variables:
# 1. we can demonstrate the whole user defined parameters all at once
# 2. help us keep in mind that changes of these variables will influence both windows and linux VHD building

# windows image sku and windows image version are recorded in code instead of pipeline variables
# because a pr gives a better chance to take a review of the version changes.
WINDOWS_IMAGE_PUBLISHER="MicrosoftWindowsServer"
WINDOWS_IMAGE_OFFER="WindowsServer"
WINDOWS_IMAGE_SKU=""
WINDOWS_IMAGE_VERSION=""
WINDOWS_IMAGE_URL=""
windows_servercore_image_url=""
windows_nanoserver_image_url=""
windows_private_packages_url=""

# msi_resource_strings is an array that will be used to build VHD build vm
# test pipelines may not set it
msi_resource_strings=()
if [ -n "${AZURE_MSI_RESOURCE_STRING}" ]; then
	msi_resource_strings+=(${AZURE_MSI_RESOURCE_STRING})
fi

# shellcheck disable=SC2236
if [ "$OS_TYPE" = "Windows" ]; then
  prepare_windows_vhd
fi

private_packages_url=""
# Set linux private packages url if the pipeline variable is set
if [ -n "${PRIVATE_PACKAGES_URL}" ]; then
	echo "PRIVATE_PACKAGES_URL is set in pipeline variables: ${PRIVATE_PACKAGES_URL}"
	private_packages_url="${PRIVATE_PACKAGES_URL}"
fi

# set PACKER_BUILD_LOCATION to the value of AZURE_LOCATION for windows
# since windows doesn't currently distinguish between the 2.
# also do this in cases where we're running a linux build in AME (for now)
# TODO(cameissner): remove conditionals for prod once new pool config has been deployed to AME.
if [ "$MODE" = "windowsVhdMode" ] || [ "${ENVIRONMENT,,}" = "prod" ]; then
	PACKER_BUILD_LOCATION=$AZURE_LOCATION
fi

produce_ua_token

# windows_image_version refers to the version from azure gallery
cat <<EOF > vhdbuilder/packer/settings.json
{
  "subscription_id": "${SUBSCRIPTION_ID}",
  "gallery_subscription_id": "${GALLERY_SUBSCRIPTION_ID}",
  "resource_group_name": "${AZURE_RESOURCE_GROUP_NAME}",
  "location": "${PACKER_BUILD_LOCATION}",
  "storage_account_name": "${STORAGE_ACCOUNT_NAME}",
  "vm_size": "${AZURE_VM_SIZE}",
  "create_time": "${CREATE_TIME}",
  "img_version": "${IMG_VERSION}",
  "SKIP_EXTENSION_CHECK": "${SKIP_EXTENSION_CHECK}",
  "INSTALL_OPEN_SSH_SERVER": "${INSTALL_OPEN_SSH_SERVER}",
  "vhd_build_timestamp": "${VHD_BUILD_TIMESTAMP}",
  "windows_image_publisher": "${WINDOWS_IMAGE_PUBLISHER}",
  "windows_image_offer": "${WINDOWS_IMAGE_OFFER}",
  "windows_image_sku": "${WINDOWS_IMAGE_SKU}",
  "windows_image_version": "${WINDOWS_IMAGE_VERSION}",
  "windows_image_url": "${WINDOWS_IMAGE_URL}",
  "imported_image_name": "${IMPORTED_IMAGE_NAME}",
  "sig_image_name":  "${SIG_IMAGE_NAME}",
  "sig_gallery_name": "${SIG_GALLERY_NAME}",
  "captured_sig_version": "${CAPTURED_SIG_VERSION}",
  "os_disk_size_gb": "${os_disk_size_gb}",
  "nano_image_url": "${windows_nanoserver_image_url}",
  "core_image_url": "${windows_servercore_image_url}",
  "windows_private_packages_url": "${windows_private_packages_url}",
  "windows_sigmode_source_subscription_id": "${windows_sigmode_source_subscription_id}",
  "windows_sigmode_source_resource_group_name": "${windows_sigmode_source_resource_group_name}",
  "windows_sigmode_source_gallery_name": "${windows_sigmode_source_gallery_name}",
  "windows_sigmode_source_image_name": "${windows_sigmode_source_image_name}",
  "windows_sigmode_source_image_version": "${windows_sigmode_source_image_version}",
  "vnet_name": "${VNET_NAME}",
  "subnet_name": "${SUBNET_NAME}",
  "vnet_resource_group_name": "${VNET_RG_NAME}",
  "msi_resource_strings": "${msi_resource_strings}",
  "private_packages_url": "${private_packages_url}",
  "ua_token": "${UA_TOKEN}",
  "build_date": "${BUILD_DATE}"
}
EOF

# so we don't accidently log UA_TOKEN, though ADO will automatically mask it if it appears in stdout
# since it's coming from a variable group
echo "packer settings:"
jq 'del(.ua_token)' < vhdbuilder/packer/settings.json
