#!/bin/bash

function enforce_azcli_version() {
	AZ_VER_REQUIRED=$1
	AZ_DIST=$(lsb_release -cs)
	echo "Enforcing az cli version to ${AZ_VER_REQUIRED} for ${AZ_DIST}"
	sudo DEBIAN_FRONTEND=noninteractive dpkg --configure -a
	sudo DEBIAN_FRONTEND=noninteractive apt-get install azure-cli=${AZ_VER_REQUIRED}-1~${AZ_DIST} -y --allow-downgrades
	AZ_VER_INSTALLED=$(az version --query "[\"azure-cli\"]" -o tsv)
	if [ "${AZ_VER_INSTALLED}" != "${AZ_VER_REQUIRED}" ]; then
		echo "Failed to install required az cli version ${AZ_VER_REQUIRED}, installed version is ${AZ_VER_INSTALLED}"
		exit 1
	else
		echo "Successfully installed az cli version ${AZ_VER_INSTALLED}"
	fi
}

function produce_ua_token() {
	set +x
	UA_TOKEN="${UA_TOKEN:-}" # used to attach UA when building ESM-enabled Ubuntu SKUs
	if [ "$MODE" = "linuxVhdMode" ] && [ "${OS_SKU,,}" = "ubuntu" ]; then
		if [ "${OS_VERSION}" = "20.04" ] || [ "${ENABLE_FIPS,,}" = "true" ]; then
			echo "OS_VERSION: ${OS_VERSION}, ENABLE_FIPS: ${ENABLE_FIPS,,}, will use token for UA attachment"
			if [ -z "${UA_TOKEN}" ]; then
				echo "UA_TOKEN must be provided when building SKUs which require ESM"
				exit 1
			fi
		else
			echo "UA_TOKEN not used for ubuntu 20.04, and FIPS"
			UA_TOKEN="notused"
		fi
	else
		echo "UA_TOKEN only used for Ubuntu"
		UA_TOKEN="notused"
	fi
}

function ensure_sig_image_name_linux() {
	# Ensure the SIG name
	if [ -z "${SIG_GALLERY_NAME}" ]; then
		SIG_GALLERY_NAME="PackerSigGalleryEastUS"
		echo "No input for SIG_GALLERY_NAME was provided, using auto-generated value: ${SIG_GALLERY_NAME}"
	else
		echo "Using provided SIG_GALLERY_NAME: ${SIG_GALLERY_NAME}"
	fi

	if [ -z "${SIG_IMAGE_NAME}" ]; then
		SIG_IMAGE_NAME=$SKU_NAME
		# shellcheck disable=SC3010
		if [ "${IMG_OFFER,,}" = "cbl-mariner" ]; then
			# we need to add a distinction here since we currently use the same image definition names
			# for both azlinux and cblmariner in prod galleries, though we only have one gallery which Packer
			# is configured to deliver images to...
			# shellcheck disable=SC3010
			if [ "${ENABLE_CGROUPV2,,}" = "true" ]; then
				SIG_IMAGE_NAME="AzureLinux${SIG_IMAGE_NAME}"
			else
				SIG_IMAGE_NAME="CBLMariner${SIG_IMAGE_NAME}"
			fi
		elif [ "${IMG_OFFER,,}" = "azure-linux-3" ]; then
			# for Azure Linux 3.0, only use AzureLinux prefix
			SIG_IMAGE_NAME="AzureLinux${SIG_IMAGE_NAME}"
		elif [ "${OS_SKU,,}" = "azurelinuxosguard" ]; then
			SIG_IMAGE_NAME="AzureLinuxOSGuard${SIG_IMAGE_NAME}"
		elif grep -q "cvm" <<<"$FEATURE_FLAGS"; then
			SIG_IMAGE_NAME+="Specialized"
		fi
		echo "No input for SIG_IMAGE_NAME was provided, defaulting to: ${SIG_IMAGE_NAME}"
	else
		echo "Using provided SIG_IMAGE_NAME: ${SIG_IMAGE_NAME}"
	fi
}

function download_windows_json_artifact() {
	filename=$(basename "$WINDOWS_CONTAINERIMAGE_JSON_URL")
	echo "Downloading $filename from wcct storage account using AzCopy with Managed Identity Auth"

	# The JSON blob is formatted where each build image name is mapped to its corresponding image URL.
	# For details on the expected format and how to manually retrieve the JSON blob,
	# see: [WINDOWS-CONTAINERIMAGE-JSON.MD](vhdbuilder/packer/WINDOWS-CONTAINERIMAGE-JSON.MD)
	if azcopy copy "${WINDOWS_CONTAINERIMAGE_JSON_URL}" "${BUILD_ARTIFACTSTAGINGDIRECTORY}/"; then
		echo "Successfully downloaded the latest artifact: $filename"
	else
		# loop through azcopy log files
		for f in "${AZCOPY_LOG_LOCATION}"/*.log; do
			echo "Azcopy log file: $f"
			# upload the log file as an attachment to vso
			set +x
			echo "##vso[build.uploadlog]$f"
			set -x
			# check if the log file contains any errors
			if grep -q '"level":"Error"' "$f"; then
				echo "log file $f contains errors"
				set +x
				echo "##vso[task.logissue type=error]Azcopy log file $f contains errors"
				set -x
				# print the log file
				cat "$f"
			fi
		done
	fi

	# Parse the json artifact to get the image urls
	echo "Filename: $filename"
	artifact_path="${BUILD_ARTIFACTSTAGINGDIRECTORY}/$filename"
	sudo chmod 600 "$artifact_path"
}

function extract_windows_image_urls() {
	echo "Reading image URLs from $artifact_path"

	# Extract image URLs from the artifact JSON using a case statement for WINDOWS_SKU
	case "${WINDOWS_SKU}" in
	"2019-containerd")
		WINDOWS_BASE_IMAGE_URL=$(jq -r '.images[] | select(.name == "WINDOWS_2019_BASE_IMAGE_URL") | .value' "$artifact_path")
		windows_nanoserver_image_url=$(jq -r '.images[] | select(.name == "WINDOWS_2019_NANO_IMAGE_URL") | .value' "$artifact_path")
		windows_servercore_image_url=$(jq -r '.images[] | select(.name == "WINDOWS_2019_CORE_IMAGE_URL") | .value' "$artifact_path")
		;;
	"2022-containerd")
		WINDOWS_BASE_IMAGE_URL=$(jq -r '.images[] | select(.name == "WINDOWS_2022_BASE_IMAGE_URL") | .value' "$artifact_path")
		windows_nanoserver_image_url=$(jq -r '.images[] | select(.name == "WINDOWS_2022_NANO_IMAGE_URL") | .value' "$artifact_path")
		windows_servercore_image_url=$(jq -r '.images[] | select(.name == "WINDOWS_2022_CORE_IMAGE_URL") | .value' "$artifact_path")
		;;
	"2022-containerd-gen2")
		WINDOWS_BASE_IMAGE_URL=$(jq -r '.images[] | select(.name == "WINDOWS_2022_GEN2_BASE_IMAGE_URL") | .value' "$artifact_path")
		windows_nanoserver_image_url=$(jq -r '.images[] | select(.name == "WINDOWS_2022_NANO_IMAGE_URL") | .value' "$artifact_path")
		windows_servercore_image_url=$(jq -r '.images[] | select(.name == "WINDOWS_2022_CORE_IMAGE_URL") | .value' "$artifact_path")
		;;
	"2025")
		WINDOWS_BASE_IMAGE_URL=$(jq -r '.images[] | select(.name == "WINDOWS_2025_BASE_IMAGE_URL") | .value' "$artifact_path")
		windows_nanoserver_image_url="$(jq -r '.images[] | select(.name == "WINDOWS_2025_NANO_IMAGE_URL") | .value' "$artifact_path"),$(jq -r '.images[] | select(.name == "WINDOWS_2022_NANO_IMAGE_URL") | .value' "$artifact_path")"
		windows_servercore_image_url="$(jq -r '.images[] | select(.name == "WINDOWS_2025_CORE_IMAGE_URL") | .value' "$artifact_path"),$(jq -r '.images[] | select(.name == "WINDOWS_2022_CORE_IMAGE_URL") | .value' "$artifact_path")"
		;;
	"2025-gen2")
		WINDOWS_BASE_IMAGE_URL=$(jq -r '.images[] | select(.name == "WINDOWS_2025_GEN2_BASE_IMAGE_URL") | .value' "$artifact_path")
		windows_nanoserver_image_url="$(jq -r '.images[] | select(.name == "WINDOWS_2025_NANO_IMAGE_URL") | .value' "$artifact_path"),$(jq -r '.images[] | select(.name == "WINDOWS_2022_NANO_IMAGE_URL") | .value' "$artifact_path")"
		windows_servercore_image_url="$(jq -r '.images[] | select(.name == "WINDOWS_2025_CORE_IMAGE_URL") | .value' "$artifact_path"),$(jq -r '.images[] | select(.name == "WINDOWS_2022_CORE_IMAGE_URL") | .value' "$artifact_path")"
		;;
	"23H2")
		WINDOWS_BASE_IMAGE_URL=$(jq -r '.images[] | select(.name == "WINDOWS_23H2_BASE_IMAGE_URL") | .value' "$artifact_path")
		windows_nanoserver_image_url=$(jq -r '.images[] | select(.name == "WINDOWS_2022_NANO_IMAGE_URL") | .value' "$artifact_path")
		windows_servercore_image_url=$(jq -r '.images[] | select(.name == "WINDOWS_2022_CORE_IMAGE_URL") | .value' "$artifact_path")
		;;
	"23H2-gen2")
		WINDOWS_BASE_IMAGE_URL=$(jq -r '.images[] | select(.name == "WINDOWS_23H2_GEN2_BASE_IMAGE_URL") | .value' "$artifact_path")
		windows_nanoserver_image_url=$(jq -r '.images[] | select(.name == "WINDOWS_2022_NANO_IMAGE_URL") | .value' "$artifact_path")
		windows_servercore_image_url=$(jq -r '.images[] | select(.name == "WINDOWS_2022_CORE_IMAGE_URL") | .value' "$artifact_path")
		;;
	*)
		echo "Unsupported WINDOWS_SKU: ${WINDOWS_SKU}"
		;;
	esac
}

function create_windows_storage_account() {

	avail=$(az storage account check-name -n "${STORAGE_ACCOUNT_NAME}" -o json | jq -r .nameAvailable)
	if $avail; then
		echo "creating new storage account ${STORAGE_ACCOUNT_NAME}"
		az storage account create \
			-n "$STORAGE_ACCOUNT_NAME" \
			-g "$AZURE_RESOURCE_GROUP_NAME" \
			--sku "Standard_RAGRS" \
			--tags "now=${CREATE_TIME}" \
			--allow-shared-key-access false \
			--location ""${AZURE_LOCATION}""
		echo "creating new container system"
		az storage container create --name system "--account-name=${STORAGE_ACCOUNT_NAME}" --auth-mode login
	else
		echo "storage account ${STORAGE_ACCOUNT_NAME} already exists."
	fi

}

function copy_windows_base_image_to_storage_account() {
	echo "Copy Windows base image to ${WINDOWS_IMAGE_URL}"

	export AZCOPY_LOG_LOCATION="$(pwd)/azcopy-log-files/"
	export AZCOPY_JOB_PLAN_LOCATION="$(pwd)/azcopy-job-plan-files/"
	mkdir -p "${AZCOPY_LOG_LOCATION}"
	mkdir -p "${AZCOPY_JOB_PLAN_LOCATION}"

	if ! azcopy copy "${WINDOWS_BASE_IMAGE_URL}" "${WINDOWS_IMAGE_URL}"; then
		# loop through azcopy log files
		set +x
		shopt -s nullglob
		for f in "${AZCOPY_LOG_LOCATION}"/*.log; do
			echo "Azcopy log file: $f"
			# upload the log file as an attachment to vso
			set +x
			echo "##vso[build.uploadlog]$f"

			# print the log file
			echo "----- START LOG $f -----"
			cat "$f"
			echo "----- END LOG $f -----"

			# check if the log file contains any errors
			if grep -q '"level":"Error"' "$f"; then
				echo "log file $f contains errors"
				echo "##vso[task.logissue type=error]Azcopy log file $f contains errors"
			fi
		done
		shopt -u nullglob
		set -x
		exit 1
	fi
}

function create_new_base_image() {
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
}

function prepare_windows_vhd() {
	echo "Set the base image sku and version from windows_settings.json"

	WINDOWS_IMAGE_SKU=$(jq -r ".WindowsBaseVersions.\"${WINDOWS_SKU}\".base_image_sku" <$CDIR/windows/windows_settings.json)
	WINDOWS_IMAGE_VERSION=$(jq -r ".WindowsBaseVersions.\"${WINDOWS_SKU}\".base_image_version" <$CDIR/windows/windows_settings.json)
	WINDOWS_IMAGE_NAME=$(jq -r ".WindowsBaseVersions.\"${WINDOWS_SKU}\".windows_image_name" <$CDIR/windows/windows_settings.json)
	OS_DISK_SIZE=$(jq -r ".WindowsBaseVersions.\"${WINDOWS_SKU}\".os_disk_size" <$CDIR/windows/windows_settings.json)
	if [ "null" != "${OS_DISK_SIZE}" ]; then
		echo "Setting os_disk_size_gb to the value in windows-settings.json for ${WINDOWS_SKU}: ${OS_DISK_SIZE}"
		os_disk_size_gb=${OS_DISK_SIZE}
	else
		os_disk_size_gb="30"
	fi

	imported_windows_image_name="${WINDOWS_IMAGE_NAME}-imported-${CREATE_TIME}-${RANDOM}"

	echo "Got base image data: "
	echo "  WINDOWS_IMAGE_SKU: ${WINDOWS_IMAGE_SKU}"
	echo "  WINDOWS_IMAGE_VERSION: ${WINDOWS_IMAGE_VERSION}"
	echo "  WINDOWS_IMAGE_NAME: ${WINDOWS_IMAGE_NAME}"
	echo "  OS_DISK_SIZE: ${OS_DISK_SIZE}"
	echo "  imported_windows_image_name: ${imported_windows_image_name}"

	if [ "${WINDOWS_IMAGE_SKU}" = "null" ]; then
		echo "unsupported windows sku: ${WINDOWS_SKU}"
		exit 1
	fi

	# Create the sig image from the official images defined in windows-settings.json by default
	windows_sigmode_source_subscription_id=""
	windows_sigmode_source_resource_group_name=""
	windows_sigmode_source_gallery_name=""
	windows_sigmode_source_image_name=""
	windows_sigmode_source_image_version=""

	# default: build VHD images from a marketplace base image
	export AZCOPY_AUTO_LOGIN_TYPE="MSI" # use Managed Identity for AzCopy authentication
	export AZCOPY_MSI_RESOURCE_STRING="${AZURE_MSI_RESOURCE_STRING}"
	export AZCOPY_LOG_LOCATION="$(pwd)/azcopy-log-files/"
	export AZCOPY_JOB_PLAN_LOCATION="$(pwd)/azcopy-job-plan-files/"
	mkdir -p "${AZCOPY_LOG_LOCATION}"
	mkdir -p "${AZCOPY_JOB_PLAN_LOCATION}"

	echo "VALID IMAGE URL: ${WINDOWS_CONTAINERIMAGE_JSON_URL}"
	if [ -n "${WINDOWS_CONTAINERIMAGE_JSON_URL}" ]; then
		download_windows_json_artifact
		extract_windows_image_urls
	else
		# If USE_CONTAINER_URLS_FROM_JSON is not true, fall back to default URLs
		echo "Falling back to default Windows image URLs"
	fi

	# Check if base, nano, and servercore urls are set
	if [ -z "${windows_nanoserver_image_url}" ] || [ -z "${windows_servercore_image_url}" ] || [ -z "${WINDOWS_BASE_IMAGE_URL}" ]; then
		echo "Error: One of the Windows image URLs are not set."
	else
		# If all URLs are set, print them
		echo "Using Windows base image URL: ${WINDOWS_BASE_IMAGE_URL}"
		echo "Using Windows Nano Server image URL: ${windows_nanoserver_image_url}"
		echo "Using Windows Server Core image URL: ${windows_servercore_image_url}"
	fi

	# build from a pre-supplied VHD blob a.k.a. external raw VHD
	if [ -n "${WINDOWS_BASE_IMAGE_URL}" ]; then
		echo "WINDOWS_BASE_IMAGE_URL is set in pipeline variable to ${WINDOWS_BASE_IMAGE_URL}"

		STORAGE_ACCOUNT_NAME="aksimages${CREATE_TIME}$RANDOM"
		echo "storage name: ${STORAGE_ACCOUNT_NAME}"

		IMPORTED_IMAGE_NAME=$imported_windows_image_name
		IMPORTED_IMAGE_URL="https://${STORAGE_ACCOUNT_NAME}.blob.core.windows.net/system/${IMPORTED_IMAGE_NAME}.vhd"

		create_windows_storage_account

		WINDOWS_IMAGE_URL=${IMPORTED_IMAGE_URL}

		copy_windows_base_image_to_storage_account

		create_new_base_image
	fi

	# Set nanoserver image url if the pipeline variable is set and the parameter is not already set
	if [ -n "${WINDOWS_NANO_IMAGE_URL}" ] && [ -z "${windows_nanoserver_image_url}" ]; then
		echo "WINDOWS_NANO_IMAGE_URL is set in pipeline variables"
		windows_nanoserver_image_url="${WINDOWS_NANO_IMAGE_URL}"
	fi

	# Set servercore image url if the pipeline variable is set and the parameter is not already set
	if [ -n "${WINDOWS_CORE_IMAGE_URL}" ] && [ -z "${windows_servercore_image_url}" ]; then
		echo "WINDOWS_CORE_IMAGE_URL is set in pipeline variables"
		windows_servercore_image_url="${WINDOWS_CORE_IMAGE_URL}"
	fi

	# Set windows private packages url if the pipeline variable is set
	if [ -n "${WINDOWS_PRIVATE_PACKAGES_URL}" ]; then
		echo "WINDOWS_PRIVATE_PACKAGES_URL is set in pipeline variables"
		windows_private_packages_url="${WINDOWS_PRIVATE_PACKAGES_URL}"
	fi
}

function ensure_sig_vhd_exists() {
	echo "SIG existence checking for $MODE"

	is_need_create=true
	state=$(az sig show --resource-group "${AZURE_RESOURCE_GROUP_NAME}" --gallery-name "${SIG_GALLERY_NAME}" | jq -r '.provisioningState') || state=""

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

		if [ $state = "Failed" ]; then
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
				if [ -z "${image_versions}" ]; then
					echo "Deleting sig image-definition ${image_definition} from gallery ${SIG_GALLERY_NAME} rg ${AZURE_RESOURCE_GROUP_NAME}"
					az sig image-definition delete --gallery-image-definition ${image_definition} -r ${SIG_GALLERY_NAME} -g ${AZURE_RESOURCE_GROUP_NAME} --no-wait false
				fi
			done
			image_defs=$(az sig image-definition list -g ${AZURE_RESOURCE_GROUP_NAME} -r ${SIG_GALLERY_NAME} | jq -r '.[] | select(.osType == "Windows").name')

			if [ -n "$image_defs" ]; then
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

	if $is_need_create; then
		echo "Creating gallery ${SIG_GALLERY_NAME} in the resource group ${AZURE_RESOURCE_GROUP_NAME} location ${AZURE_LOCATION}"
		az sig create --resource-group ${AZURE_RESOURCE_GROUP_NAME} --gallery-name ${SIG_GALLERY_NAME} --location ${AZURE_LOCATION}
	fi

	id=$(az sig image-definition show \
		--resource-group ${AZURE_RESOURCE_GROUP_NAME} \
		--gallery-name ${SIG_GALLERY_NAME} \
		--gallery-image-definition ${SIG_IMAGE_NAME}) || id=""
	if [ -z "$id" ]; then
		echo "Creating image definition ${SIG_IMAGE_NAME} in gallery ${SIG_GALLERY_NAME} resource group ${AZURE_RESOURCE_GROUP_NAME}"
		# The following conditionals do not require NVMe tagging on disk controller type
		# shellcheck disable=SC3010
		if [[ ${ARCHITECTURE,,} == "arm64" ]] || grep -q "cvm" <<<"$FEATURE_FLAGS" || [[ ${HYPERV_GENERATION} == "V1" ]]; then
			TARGET_COMMAND_STRING=""
			if [ "${ARCHITECTURE,,}" = "arm64" ]; then
				TARGET_COMMAND_STRING+="--architecture Arm64 --features DiskControllerTypes=SCSI,NVMe"
			elif grep -q "cvm" <<<"$FEATURE_FLAGS"; then
				TARGET_COMMAND_STRING+="--os-state Specialized --features SecurityType=ConfidentialVM"
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
			# TL can only be enabled on Gen2 VMs, therefore if TL enabled = true, mark features for both TL and NVMe
			if [ "${ENABLE_TRUSTED_LAUNCH}" = "True" ]; then
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
					--features "DiskControllerTypes=SCSI,NVMe SecurityType=TrustedLaunch"
			else
				# For vanilla Gen2, mark only NVMe
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
					--features DiskControllerTypes=SCSI,NVMe
			fi
		fi
	else
		echo "Image definition ${SIG_IMAGE_NAME} existing in gallery ${SIG_GALLERY_NAME} resource group ${AZURE_RESOURCE_GROUP_NAME}"
	fi
}
