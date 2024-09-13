#!/bin/bash -e
set -x
CDIR=$(dirname "${BASH_SOURCE}")
SETTINGS_JSON="${SETTINGS_JSON:-./packer/settings.json}"
PUBLISHER_BASE_IMAGE_VERSION_JSON="${PUBLISHER_BASE_IMAGE_VERSION_JSON:-./vhdbuilder/publisher_base_image_version.json}"
VHD_BUILD_TIMESTAMP_JSON="${VHD_BUILD_TIMESTAMP_JSON:-./vhdbuilder/vhd_build_timestamp.json}"
SUBSCRIPTION_ID="${SUBSCRIPTION_ID:-$(az account show -o json --query="id" | tr -d '"')}"
CREATE_TIME="$(date +%s)"
STORAGE_ACCOUNT_NAME="aksimages${CREATE_TIME}$RANDOM"

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
if [ "$MODE" == "linuxVhdMode" ]; then
	# In linux builds, this variable is only used for creating the resource group holding the
	# staging "PackerSigGalleryEastUS" SIG, as well as the gallery itself. It's also used
	# for creating any image definitions that might be missing from the gallery based on the particular
	# SKU being built.
	#
	# For windows, this variable is also used for creating resources to import base images
	AZURE_LOCATION="eastus"
fi

# We use the provided SIG_IMAGE_VERSION if it's instantiated and we're running linuxVhdMode, otherwise we randomly generate one
if [[ "${MODE}" == "linuxVhdMode" ]] && [[ -n "${SIG_IMAGE_VERSION}" ]]; then
	CAPTURED_SIG_VERSION=${SIG_IMAGE_VERSION}
else
	CAPTURED_SIG_VERSION="1.${CREATE_TIME}.$RANDOM"
fi

if [ -z "${POOL_NAME}" ]; then
	echo "POOL_NAME is not set, can't compute VNET_RG_NAME for packer templates"
	exit 1
fi

echo "POOL_NAME is set to $POOL_NAME"

if [ "$MODE" == "linuxVhdMode" ] && [ -z "${SKU_NAME}" ]; then
	echo "SKU_NAME must be set for linux VHD builds"
	exit 1
fi

# This variable is used within linux builds to inform which region that packer build itself will be running,
# and subsequently the region in which the 1ES pool the build is running on is in.
# Note that this variable is ONLY used for linux builds, windows builds simply use AZURE_LOCATION.
if [ "$MODE" == "linuxVhdMode" ] && [ -z "${PACKER_BUILD_LOCATION}" ]; then
	echo "PACKER_BUILD_LOCATION is not set, cannot compute VNET_RG_NAME for packer templates"
	exit 1
fi

# Currently only used for linux builds. This determines the environment in which the build is running (either prod or test).
# Used to construct the name of the resource group in which the 1ES pool the build is running on lives in, which also happens.
# to be the resource group in which the packer VNET lives in.
if [ "$MODE" == "linuxVhdMode" ] && [ -z "${ENVIRONMENT}" ]; then
	echo "ENVIRONMENT is not set, cannot compute VNET_RG_NAME or VNET_NAME for packer templates"
	exit 1
fi

if [ -z "${VNET_RG_NAME}" ]; then
	if [ "$MODE" == "linuxVhdMode" ]; then
		VNET_RG_NAME="nodesig-${ENVIRONMENT}-${PACKER_BUILD_LOCATION}-agent-pool"
	fi
	if [ "$MODE" == "windowsVhdMode" ]; then
		if [[ "${POOL_NAME}" == *nodesigprod* ]]; then
			VNET_RG_NAME="nodesigprod-agent-pool"
		else
			VNET_RG_NAME="nodesigtest-agent-pool"
		fi
	fi
fi

if [ -z "${VNET_NAME}" ]; then
	if [ "$MODE" == "linuxVhdMode" ]; then
		VNET_NAME="nodesig-pool-vnet-${PACKER_BUILD_LOCATION}"
	fi
	if [ "$MODE" == "windowsVhdMode" ]; then
		VNET_NAME="nodesig-pool-vnet"
	fi
fi

if [ -z "${SUBNET_NAME}" ]; then
	SUBNET_NAME="packer"
fi

echo "VNET_RG_NAME set to: ${VNET_RG_NAME}"

echo "CAPTURED_SIG_VERSION set to: ${CAPTURED_SIG_VERSION}"

echo "Subscription ID: ${SUBSCRIPTION_ID}"

rg_id=$(az group show --name $AZURE_RESOURCE_GROUP_NAME) || rg_id=""
if [ -z "$rg_id" ]; then
	echo "Creating resource group $AZURE_RESOURCE_GROUP_NAME, location ${AZURE_LOCATION}"
	az group create --name $AZURE_RESOURCE_GROUP_NAME --location ${AZURE_LOCATION}
fi

if [ "$MODE" != "linuxVhdMode" ]; then
	avail=$(az storage account check-name -n ${STORAGE_ACCOUNT_NAME} -o json | jq -r .nameAvailable)
	if $avail ; then
		echo "creating new storage account ${STORAGE_ACCOUNT_NAME}"
		az storage account create -n $STORAGE_ACCOUNT_NAME -g $AZURE_RESOURCE_GROUP_NAME --sku "Standard_RAGRS" --tags "now=${CREATE_TIME}" --location ${AZURE_LOCATION}
		echo "creating new container system"
		az storage container create --name system --account-name=$STORAGE_ACCOUNT_NAME --auth-mode login
	else
		echo "storage account ${STORAGE_ACCOUNT_NAME} already exists."
	fi
fi

echo "storage name: ${STORAGE_ACCOUNT_NAME}"

# If SIG_GALLERY_NAME/SIG_IMAGE_NAME hasnt been provided in linuxVhdMode, use defaults
# NOTE: SIG_IMAGE_NAME is the name of the image definition that Packer will use when delivering the
# output image version to the staging gallery. This is NOT the name of the image definitions used in prod.
if [[ "${MODE}" == "linuxVhdMode" ]]; then
	# Ensure the SIG name
	if [[ -z "${SIG_GALLERY_NAME}" ]]; then
		SIG_GALLERY_NAME="PackerSigGalleryEastUS"
		echo "No input for SIG_GALLERY_NAME was provided, using auto-generated value: ${SIG_GALLERY_NAME}"
	else
		echo "Using provided SIG_GALLERY_NAME: ${SIG_GALLERY_NAME}"
	fi

	if [[ -z "${SIG_IMAGE_NAME}" ]]; then
		SIG_IMAGE_NAME=$SKU_NAME
		if [[ "${IMG_OFFER,,}" == "cbl-mariner" ]]; then
			# we need to add a distinction here since we currently use the same image definition names
			# for both azlinux and cblmariner in prod galleries, though we only have one gallery which Packer
			# is configured to deliver images to...
			if [ "${ENABLE_CGROUPV2,,}" == "true" ]; then
				SIG_IMAGE_NAME="AzureLinux${SIG_IMAGE_NAME}"
			else
				SIG_IMAGE_NAME="CBLMariner${SIG_IMAGE_NAME}"
			fi
		elif [[ "${IMG_OFFER,,}" == "azure-linux-3" ]]; then
			# for Azure Linux 3.0, only use AzureLinux prefix
			SIG_IMAGE_NAME="AzureLinux${SIG_IMAGE_NAME}"
		fi
		echo "No input for SIG_IMAGE_NAME was provided, defaulting to: ${SIG_IMAGE_NAME}"
	else
		echo "Using provided SIG_IMAGE_NAME: ${SIG_IMAGE_NAME}"
	fi
fi

if [[ "${MODE}" == "windowsVhdMode" ]] && [[ ${ARCHITECTURE,,} == "arm64" ]]; then
	# only append 'Arm64' in windows builds, for linux we either take what was provided
	# or base the name off the the value of SKU_NAME (see above)
  SIG_IMAGE_NAME=${SIG_IMAGE_NAME//./}Arm64
fi

echo "Using finalized SIG_IMAGE_NAME: ${SIG_IMAGE_NAME}, SIG_GALLERY_NAME: ${SIG_GALLERY_NAME}"

# If we're building a Linux VHD or we're building a windows VHD in windowsVhdMode, ensure SIG resources
if [[ "$MODE" == "linuxVhdMode" || "$MODE" == "windowsVhdMode" ]]; then
	echo "SIG existence checking for $MODE"

	is_need_create=true
	state=$(az sig show --resource-group ${AZURE_RESOURCE_GROUP_NAME} --gallery-name ${SIG_GALLERY_NAME} | jq -r '.provisioningState') || state=""

	# {
	#   "description": null,
	#   "id": "/subscriptions/xxx/resourceGroups/xxx/providers/Microsoft.Compute/galleries/WSGallery240719",
	#   "identifier": {
	#     "uniqueName": "xxx-WSGALLERY240719"
	#   },
	#   "location": "eastus",
	#   "name": "WSGallery240719",
	#   "provisioningState": "Failed",
	#   "resourceGroup": "xxx",
	#   "sharingProfile": null,
	#   "sharingStatus": null,
	#   "softDeletePolicy": null,
	#   "tags": {},
	#   "type": "Microsoft.Compute/galleries"
	# }
	if [ -n "$state" ]; then
		echo "Gallery ${SIG_GALLERY_NAME} exists in the resource group ${AZURE_RESOURCE_GROUP_NAME} location ${AZURE_LOCATION}"

		if [[ $state == "Failed" ]]; then
			echo "Gallery ${SIG_GALLERY_NAME} is in a failed state, deleting and recreating"

			image_defs=$(az sig image-definition list -g ${AZURE_RESOURCE_GROUP_NAME} -r ${SIG_GALLERY_NAME} | jq -r '.[] | select(.osType == "Windows").name')
			for image_definition in $image_defs; do
				echo "Finding sig image versions associated with ${image_definition} in gallery ${SIG_GALLERY_NAME}"
				image_versions=$(az sig image-version list -g ${AZURE_RESOURCE_GROUP_NAME} -r ${SIG_GALLERY_NAME} -i ${image_definition} | jq -r '.[].name')
				for image_version in $image_versions; do
					echo "Deleting sig image-version ${image_version} ${image_definition} from gallery ${SIG_GALLERY_NAME} rg ${AZURE_RESOURCE_GROUP_NAME}"
					az sig image-version delete -e $image_version -i ${image_definition} -r ${SIG_GALLERY_NAME} -g ${AZURE_RESOURCE_GROUP_NAME} --no-wait false
				done
				image_versions=$(az sig image-version list -g ${AZURE_RESOURCE_GROUP_NAME} -r ${SIG_GALLERY_NAME} -i ${image_definition} | jq -r '.[].name')
				echo "image versions are $image_versions"
				if [[ -z "${image_versions}" ]]; then
					echo "Deleting sig image-definition ${image_definition} from gallery ${SIG_GALLERY_NAME} rg ${AZURE_RESOURCE_GROUP_NAME}"
					az sig image-definition delete --gallery-image-definition ${image_definition} -r ${SIG_GALLERY_NAME} -g ${AZURE_RESOURCE_GROUP_NAME} --no-wait false
				fi
			done
			image_defs=$(az sig image-definition list -g ${AZURE_RESOURCE_GROUP_NAME} -r ${SIG_GALLERY_NAME} | jq -r '.[] | select(.osType == "Windows").name')

			if [[ -n $image_defs ]]; then
				echo $image_defs
			fi

			echo "Deleting gallery ${gallery}"
			az sig delete --resource-group ${AZURE_RESOURCE_GROUP_NAME} --gallery-name ${SIG_GALLERY_NAME} --no-wait false

			is_need_create=true
		else
			echo "Gallery ${SIG_GALLERY_NAME} is in a $state state"
			is_need_create=false
		fi
	fi

	if $is_need_create ; then
		echo "Creating gallery ${SIG_GALLERY_NAME} in the resource group ${AZURE_RESOURCE_GROUP_NAME} location ${AZURE_LOCATION}"
		az sig create --resource-group ${AZURE_RESOURCE_GROUP_NAME} --gallery-name ${SIG_GALLERY_NAME} --location ${AZURE_LOCATION}
	fi

	id=$(az sig image-definition show \
		--resource-group ${AZURE_RESOURCE_GROUP_NAME} \
		--gallery-name ${SIG_GALLERY_NAME} \
		--gallery-image-definition ${SIG_IMAGE_NAME}) || id=""
	if [ -z "$id" ]; then
		echo "Creating image definition ${SIG_IMAGE_NAME} in gallery ${SIG_GALLERY_NAME} resource group ${AZURE_RESOURCE_GROUP_NAME}"
		TARGET_COMMAND_STRING=""
		if [[ ${ARCHITECTURE,,} == "arm64" ]]; then
			TARGET_COMMAND_STRING+="--architecture Arm64"
		elif [[ ${IMG_SKU} == "20_04-lts-cvm" ]]; then
			TARGET_COMMAND_STRING+="--features SecurityType=ConfidentialVMSupported"
		elif [[ ${ENABLE_TRUSTED_LAUNCH} == "True" ]]; then
			TARGET_COMMAND_STRING+="--features SecurityType=TrustedLaunch"
		fi

		az sig image-definition create \
			--resource-group ${AZURE_RESOURCE_GROUP_NAME} \
			--gallery-name ${SIG_GALLERY_NAME} \
			--gallery-image-definition ${SIG_IMAGE_NAME} \
			--publisher microsoft-aks \
			--offer ${SIG_GALLERY_NAME} \
			--sku ${SIG_IMAGE_NAME} \
			--os-type ${OS_TYPE} \
			--hyper-v-generation ${HYPERV_GENERATION} \
			--location ${AZURE_LOCATION} \
			${TARGET_COMMAND_STRING}
	else
		echo "Image definition ${SIG_IMAGE_NAME} existing in gallery ${SIG_GALLERY_NAME} resource group ${AZURE_RESOURCE_GROUP_NAME}"
	fi
else
	echo "Skipping SIG check for $MODE, os-type: ${OS_TYPE}"
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
if [ "$OS_TYPE" == "Windows" ]; then
	imported_windows_image_name=""
	source $CDIR/windows-image.env

	echo "Set the base image sku and version from windows-image.env"
	case "${WINDOWS_SKU}" in
	"2019")
		WINDOWS_IMAGE_SKU=$WINDOWS_2019_BASE_IMAGE_SKU
		WINDOWS_IMAGE_VERSION=$WINDOWS_2019_BASE_IMAGE_VERSION
		imported_windows_image_name="windows-2019-imported-${CREATE_TIME}-${RANDOM}"

		echo "Set OS disk size"
		if [ -n "${WINDOWS_2019_OS_DISK_SIZE_GB}" ]; then
			echo "Setting os_disk_size_gb to the value in windows-image.env for 2019 Docker: ${WINDOWS_2019_OS_DISK_SIZE_GB}"
			os_disk_size_gb=${WINDOWS_2019_OS_DISK_SIZE_GB}
		fi
		;;
	"2019-containerd")
		WINDOWS_IMAGE_SKU=$WINDOWS_2019_BASE_IMAGE_SKU
		WINDOWS_IMAGE_VERSION=$WINDOWS_2019_BASE_IMAGE_VERSION
		imported_windows_image_name="windows-2019-containerd-imported-${CREATE_TIME}-${RANDOM}"

		echo "Set OS disk size"
		if [ -n "${WINDOWS_2019_CONTAINERD_OS_DISK_SIZE_GB}" ]; then
			echo "Setting os_disk_size_gb to the value in windows-image.env for 2019 Containerd: ${WINDOWS_2019_CONTAINERD_OS_DISK_SIZE_GB}"
			os_disk_size_gb=${WINDOWS_2019_CONTAINERD_OS_DISK_SIZE_GB}
		fi
		;;
	"2022-containerd" | "2022-containerd-gen2")
		WINDOWS_IMAGE_SKU=$WINDOWS_2022_BASE_IMAGE_SKU
		WINDOWS_IMAGE_VERSION=$WINDOWS_2022_BASE_IMAGE_VERSION
		imported_windows_image_name="windows-2022-containerd-imported-${CREATE_TIME}-${RANDOM}"

		echo "Set OS disk size"
		if [ -n "${WINDOWS_2022_CONTAINERD_OS_DISK_SIZE_GB}" ]; then
			echo "Setting os_disk_size_gb to the value in windows-image.env for 2022 Containerd: ${WINDOWS_2022_CONTAINERD_OS_DISK_SIZE_GB}"
			os_disk_size_gb=${WINDOWS_2022_CONTAINERD_OS_DISK_SIZE_GB}
		fi
		# Default: read from the official MCR image
		if [[ $HYPERV_GENERATION == "V2" ]]; then
			WINDOWS_IMAGE_SKU=$WINDOWS_2022_GEN2_BASE_IMAGE_SKU
			WINDOWS_IMAGE_VERSION=$WINDOWS_2022_GEN2_BASE_IMAGE_VERSION
		fi
		;;
	"23H2" | "23H2-gen2")
		WINDOWS_IMAGE_SKU=$WINDOWS_23H2_BASE_IMAGE_SKU
		WINDOWS_IMAGE_VERSION=$WINDOWS_23H2_BASE_IMAGE_VERSION
		imported_windows_image_name="windows-23H2-imported-${CREATE_TIME}-${RANDOM}"

		echo "Set OS disk size"
		if [ -n "${WINDOWS_23H2_OS_DISK_SIZE_GB}" ]; then
			echo "Setting os_disk_size_gb to the value in windows-image.env for 23H2: ${WINDOWS_23H2_OS_DISK_SIZE_GB}"
			os_disk_size_gb=${WINDOWS_23H2_OS_DISK_SIZE_GB}
		fi
		# Default: read from the official MCR image
		if [[ $HYPERV_GENERATION == "V2" ]]; then
			WINDOWS_IMAGE_SKU=$WINDOWS_23H2_GEN2_BASE_IMAGE_SKU
			WINDOWS_IMAGE_VERSION=$WINDOWS_23H2_GEN2_BASE_IMAGE_VERSION
		fi
		;;
	*)
		echo "unsupported windows sku: ${WINDOWS_SKU}"
		exit 1
		;;
	esac

	# Create the sig image from the official images defined in windows-image.env by default
	windows_sigmode_source_subscription_id=""
	windows_sigmode_source_resource_group_name=""
	windows_sigmode_source_gallery_name=""
	windows_sigmode_source_image_name=""
	windows_sigmode_source_image_version=""

	# default: build VHD images from a marketplace base image
	IMPORTED_IMAGE_NAME=$imported_windows_image_name
	IMPORTED_IMAGE_URL="https://${STORAGE_ACCOUNT_NAME}.blob.core.windows.net/system/$IMPORTED_IMAGE_NAME.vhd"

	# build from a pre-supplied VHD blob a.k.a. external raw VHD
	if [ -n "${WINDOWS_BASE_IMAGE_URL}" ]; then
		echo "WINDOWS_BASE_IMAGE_URL is set in pipeline variables"

		WINDOWS_IMAGE_URL=${IMPORTED_IMAGE_URL}

		echo "Copy Windows base image to ${WINDOWS_IMAGE_URL}"
		export AZCOPY_AUTO_LOGIN_TYPE="MSI"
		export AZCOPY_MSI_RESOURCE_STRING="${AZURE_MSI_RESOURCE_STRING}"
		azcopy-preview copy "${WINDOWS_BASE_IMAGE_URL}" "${WINDOWS_IMAGE_URL}"
		# https://www.packer.io/plugins/builders/azure/arm#image_url
		# WINDOWS_IMAGE_URL to a custom VHD to use for your base image. If this value is set, image_publisher, image_offer, image_sku, or image_version should not be set.
		WINDOWS_IMAGE_PUBLISHER=""
		WINDOWS_IMAGE_OFFER=""
		WINDOWS_IMAGE_SKU=""
		WINDOWS_IMAGE_VERSION=""

		# Need to use a sig image to create the build VM
		# create a new managed image $IMPORTED_IMAGE_NAME from os disk source $IMPORTED_IMAGE_URL
		echo "Creating new image for imported vhd ${IMPORTED_IMAGE_URL}"
		az image create \
			--resource-group $AZURE_RESOURCE_GROUP_NAME \
			--name $IMPORTED_IMAGE_NAME \
			--source ${IMPORTED_IMAGE_URL} \
			--location $AZURE_LOCATION \
			--hyper-v-generation $HYPERV_GENERATION \
			--os-type ${OS_TYPE}

		# create a gallery image definition $IMPORTED_IMAGE_NAME
		echo "Creating new image-definition for imported image ${IMPORTED_IMAGE_NAME}"
		# Need to specifiy hyper-v-generation to support Gen 2
		az sig image-definition create \
			--resource-group $AZURE_RESOURCE_GROUP_NAME \
			--gallery-name $SIG_GALLERY_NAME \
			--gallery-image-definition $IMPORTED_IMAGE_NAME \
			--location $AZURE_LOCATION \
			--hyper-v-generation $HYPERV_GENERATION \
			--os-type ${OS_TYPE} \
			--publisher microsoft-aks \
			--sku ${WINDOWS_SKU} \
			--offer $IMPORTED_IMAGE_NAME \
			--os-state generalized \
			--description "Imported image for AKS Packer build"

		# create a image version defaulting to 1.0.0 for $IMPORTED_IMAGE_NAME
		echo "Creating new image-version for imported image ${IMPORTED_IMAGE_NAME}"
		az sig image-version create \
			--location $AZURE_LOCATION \
			--resource-group $AZURE_RESOURCE_GROUP_NAME \
			--gallery-name $SIG_GALLERY_NAME \
			--gallery-image-definition $IMPORTED_IMAGE_NAME \
			--gallery-image-version 1.0.0 \
			--managed-image $IMPORTED_IMAGE_NAME

		# Use imported sig image to create the build VM
		WINDOWS_IMAGE_URL=""
		windows_sigmode_source_subscription_id=$SUBSCRIPTION_ID
		windows_sigmode_source_resource_group_name=$AZURE_RESOURCE_GROUP_NAME
		windows_sigmode_source_gallery_name=$SIG_GALLERY_NAME
		windows_sigmode_source_image_name=$IMPORTED_IMAGE_NAME
		windows_sigmode_source_image_version="1.0.0"
	fi

	# Set nanoserver image url if the pipeline variable is set
	if [ -n "${WINDOWS_NANO_IMAGE_URL}" ]; then
		echo "WINDOWS_NANO_IMAGE_URL is set in pipeline variables"
		windows_nanoserver_image_url="${WINDOWS_NANO_IMAGE_URL}"
	fi

	# Set servercore image url if the pipeline variable is set
	if [ -n "${WINDOWS_CORE_IMAGE_URL}" ]; then
		echo "WINDOWS_CORE_IMAGE_URL is set in pipeline variables"
		windows_servercore_image_url="${WINDOWS_CORE_IMAGE_URL}"
	fi

	# Set windows private packages url if the pipeline variable is set
	if [ -n "${WINDOWS_PRIVATE_PACKAGES_URL}" ]; then
		echo "WINDOWS_PRIVATE_PACKAGES_URL is set in pipeline variables"
		windows_private_packages_url="${WINDOWS_PRIVATE_PACKAGES_URL}"
	fi
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
if [ "$MODE" == "windowsVhdMode" ] || [ "${ENVIRONMENT,,}" == "prod" ]; then
	PACKER_BUILD_LOCATION=$AZURE_LOCATION
fi

# windows_image_version refers to the version from azure gallery
# aks_windows_image_version refers to the version built by AKS Windows SIG
cat <<EOF > vhdbuilder/packer/settings.json
{ 
  "subscription_id":  "${SUBSCRIPTION_ID}",
  "resource_group_name": "${AZURE_RESOURCE_GROUP_NAME}",
  "location": "${PACKER_BUILD_LOCATION}",
  "storage_account_name": "${STORAGE_ACCOUNT_NAME}",
  "vm_size": "${AZURE_VM_SIZE}",
  "create_time": "${CREATE_TIME}",
  "img_version": "${IMG_VERSION}",
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
  "aks_windows_image_version": "${AKS_WINDOWS_IMAGE_VERSION}"
}
EOF

cat vhdbuilder/packer/settings.json
