#!/bin/bash
set -euo pipefail
# Don't echo commands to the console, as this will cause Azure DevOps to do odd things with setvariable.
set +x

# This script builds a windows VHD. It has the following steps:
# 1. Validate the source branch. Releasable VHDs must be created from branches with the right name: windows/vYYYYMMDD
# 2. Set ENV vars for use later in the script
#
#
# The script uses and sets several environment variables:
# Uses:
# * DRY_RUN (will overwrite the value in the pipeline)
# * BRANCH - the git branch name
# * IS_RELEASE_PIPELINE
# * SIG_GALLERY_NAME (will overwrite the value in the pipeline)
# * SIG_IMAGE_NAME_PREFIX (will overwrite the value in the pipeline)
# * SIG_IMAGE_VERSION (will overwrite the value in the pipeline)
# * WINDOWS_SKU
# * USE_RELEASE_DATE
#
# Outputs:
# * MODE - the build mode. Is always set to "windowsVhdMode" as this is the windows pipeline.
# * SKIPVALIDATEREOFFERUPDATE - is always set to True
# * BUILD_DATE

# First we validate the branch. DRY_RUN is only allowed to be false on release branches - which are of the form
# windows/vYYYYMMDD. If we're on the release branch then we also override SIG_FOR_PRODUCTION, because this is a production build.
# for dry runs, we set SIG_FOR_PRODUCTION to false.
# SIG_FOR_PRODUCTION has the side effect of deleting the generated VHD.

echo "Checking SourceBranch: ${BRANCH}"

# Check if IS_RELEASE_PIPELINE is already set in the environment
if [ -z "${IS_RELEASE_PIPELINE:-}" ]; then
	if echo "${BRANCH}" | grep -E '^refs/heads/windows/v[[:digit:]]{8}$' >/dev/null; then
		echo "The branch ${BRANCH} is a release branch. Setting IS_RELEASE_PIPELINE to True."
		export IS_RELEASE_PIPELINE="True"
		echo "##vso[task.setvariable variable=IS_RELEASE_PIPELINE]True"
	else
		echo "The branch ${BRANCH} is not a release branch. Setting IS_RELEASE_PIPELINE to False."
		export IS_RELEASE_PIPELINE="False"
		echo "##vso[task.setvariable variable=IS_RELEASE_PIPELINE]False"
	fi
fi

if [ "${IS_RELEASE_PIPELINE}" = "True" ]; then
	if [ "${DRY_RUN}" = "True" ]; then
		echo "This is a test build triggered from the release pipeline"
	else
		echo "This is a release build triggered from the release pipeline. DRY_RUN=${DRY_RUN}"

		if ! (echo "${BRANCH}" | grep -E '^refs/heads/windows/v[[:digit:]]{8}$' >/dev/null); then
			echo "The branch ${BRANCH} is not release branch. Please use the release branch. Release branch name format: windows/vYYYYMMDD."
			exit 1
		fi
		echo "##vso[task.setvariable variable=SIG_FOR_PRODUCTION]True"
	fi
else
	echo "This is a test build triggered from the test pipeline"
	export DRY_RUN=True
	echo "##vso[task.setvariable variable=DRY_RUN]$DRY_RUN"
	echo "##vso[task.setvariable variable=SIG_FOR_PRODUCTION]False"
fi

export MODE="windowsVhdMode"
echo "Set build mode to $MODE"
echo "##vso[task.setvariable variable=MODE]$MODE"

echo "Original SIG_GALLERY_NAME: ${SIG_GALLERY_NAME:-}"
echo "Original SIG_IMAGE_NAME_PREFIX: ${SIG_IMAGE_NAME_PREFIX:-}"
echo "Original SIG_IMAGE_VERSION: ${SIG_IMAGE_VERSION:-}"

# -n is "not empty"
if [ -n "${SIG_GALLERY_NAME:-}" ] && [ -n "${SIG_IMAGE_NAME_PREFIX:-}" ] && [ -n "${SIG_IMAGE_VERSION:-}" ]; then
	echo "All of Name, Prefix, and Version have been set"
	export SIG_IMAGE_NAME="${SIG_IMAGE_NAME_PREFIX}-${WINDOWS_SKU}"
else
	echo "At least on of the name, prefix or version are empty. Overwriting all values. "
	export SIG_IMAGE_VERSION="$(date +"%y%m%d").$(date +"%H%M%S").$RANDOM"
	export SIG_IMAGE_NAME="windows-${WINDOWS_SKU}"
	export SIG_GALLERY_NAME="PackerSigGalleryEastUS"

	export WS_SKU=$(echo $WINDOWS_SKU | tr '-' '_')
fi

if [ "${USE_RELEASE_DATE:-}" = "False" ]; then
	echo "use current date as build date"
	BUILD_DATE=$(date +"%y%m%d")
else
	echo "use release date as build date"
	echo "${RELEASE_DATE:-}" | grep -E '[[:digit:]]{6}'
	if (($? != 0)); then
		echo "The release date ${RELEASE_DATE} is not valid date. Release date format: YYMMDD."
		exit 1
	fi
	export BUILD_DATE=${RELEASE_DATE}
fi
echo "Default BUILD_DATE is $BUILD_DATE"
if [ -n "${CUSTOM_BUILD_DATE:-}" ]; then
	echo "set BUILD_DATE to ${CUSTOM_BUILD_DATE}"
	export BUILD_DATE=${CUSTOM_BUILD_DATE}
fi

echo "Modified SIG_IMAGE_VERSION: ${SIG_IMAGE_VERSION}"
echo "Modified SIG_IMAGE_NAME: ${SIG_IMAGE_NAME}"
echo "Modified SIG_GALLERY_NAME: ${SIG_GALLERY_NAME}"
echo "Set build date to $BUILD_DATE"
echo "Use CSE pacakge at URI: ${WINDOWS_CSE_PACKAGE_URI}"

# Finally, we invoke packer to build the VHD.
packer init ./vhdbuilder/packer/packer-plugin.pkr.hcl
packer version
./vhdbuilder/packer/produce-packer-settings.sh
packer build -timestamp-ui -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/windows/windows-vhd-builder-sig.json | tee -a packer-output

export OS_DISK_URI="$(cat packer-output | grep -a "OSDiskUri:" | cut -d " " -f 2)"
export MANAGED_SIG_ID="$(cat packer-output | grep -a "ManagedImageSharedImageGalleryId:" | cut -d " " -f 2)"

echo "Found OS_DISK_URI: ${OS_DISK_URI}"
echo "Found MANAGED_SIG_ID: ${MANAGED_SIG_ID}"

# Break gallery lineage if the build was sourced from a shared gallery.
# Sometimes Azure requires AutomaticOSUpgrade on VMSS created from images with gallery lineage.
# Re-creating the SIG image version from a managed disk severs that association.
sig_source_gallery_name=$(jq -r ".WindowsBaseVersions.\"${WINDOWS_SKU}\".sig_source_gallery_name // empty" <vhdbuilder/packer/windows/windows_settings.json)
if [ -n "${sig_source_gallery_name}" ] && [ -n "${MANAGED_SIG_ID}" ]; then
	echo "Build sourced from gallery '${sig_source_gallery_name}' — breaking gallery lineage on SIG image version"
	LINEAGE_DISK_NAME="lineage-break-${SIG_IMAGE_NAME}-$(date +%s)"
	LOCATION="${AZURE_LOCATION}"

	echo "Creating managed disk from SIG image version..."
	az disk create \
		--resource-group "${AZURE_RESOURCE_GROUP_NAME}" \
		--name "${LINEAGE_DISK_NAME}" \
		--gallery-image-reference "${MANAGED_SIG_ID}" \
		--location "${LOCATION}" \
		--hyper-v-generation "${HYPERV_GENERATION:-V2}" \
		--os-type Windows \
		-o none

	DISK_ID="/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${AZURE_RESOURCE_GROUP_NAME}/providers/Microsoft.Compute/disks/${LINEAGE_DISK_NAME}"
	echo "Created disk: ${DISK_ID}"

	echo "Deleting original SIG image version..."
	az sig image-version delete \
		--resource-group "${AZURE_RESOURCE_GROUP_NAME}" \
		--gallery-name "${SIG_GALLERY_NAME}" \
		--gallery-image-definition "${SIG_IMAGE_NAME}" \
		--gallery-image-version "${SIG_IMAGE_VERSION}"

	echo "Waiting for deletion to propagate..."
	az sig image-version wait \
		--resource-group "${AZURE_RESOURCE_GROUP_NAME}" \
		--gallery-name "${SIG_GALLERY_NAME}" \
		--gallery-image-definition "${SIG_IMAGE_NAME}" \
		--gallery-image-version "${SIG_IMAGE_VERSION}" \
		--deleted 2>/dev/null || sleep 30

	echo "Re-creating SIG image version from managed disk (no gallery lineage)..."
	az sig image-version create \
		--resource-group "${AZURE_RESOURCE_GROUP_NAME}" \
		--gallery-name "${SIG_GALLERY_NAME}" \
		--gallery-image-definition "${SIG_IMAGE_NAME}" \
		--gallery-image-version "${SIG_IMAGE_VERSION}" \
		--os-snapshot "${DISK_ID}" \
		--location "${LOCATION}" \
		--replica-count 1 \
		-o none

	NEW_SIG_ID="/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${AZURE_RESOURCE_GROUP_NAME}/providers/Microsoft.Compute/galleries/${SIG_GALLERY_NAME}/images/${SIG_IMAGE_NAME}/versions/${SIG_IMAGE_VERSION}"
	echo "New SIG image version (lineage-free): ${NEW_SIG_ID}"
	export MANAGED_SIG_ID="${NEW_SIG_ID}"

	echo "Cleaning up temporary disk..."
	az disk delete --resource-group "${AZURE_RESOURCE_GROUP_NAME}" --name "${LINEAGE_DISK_NAME}" --yes -o none

	echo "Gallery lineage successfully broken"
else
	echo "Build was not sourced from a gallery — no lineage breaking needed"
fi

# if bash is echoing the commands, then ADO processes both the echo of the command to set the variable and the command itself.
# This causes super odd behavior in ADO.
set +x
echo "##vso[task.setvariable variable=SIG_GALLERY_NAME]$SIG_GALLERY_NAME"
echo "##vso[task.setvariable variable=SIG_IMAGE_NAME]$SIG_IMAGE_NAME"
echo "##vso[task.setvariable variable=SIG_IMAGE_VERSION]$SIG_IMAGE_VERSION"
echo "##vso[task.setvariable variable=SKIPVALIDATEREOFFERUPDATE]True"
echo "##vso[task.setvariable variable=BUILD_DATE]$BUILD_DATE"
echo "##vso[task.setvariable variable=DRY_RUN]${DRY_RUN}"
echo "##vso[task.setvariable variable=WINDOWS_CSE_PACKAGE_URI]${WINDOWS_CSE_PACKAGE_URI}"
echo "##vso[task.setvariable variable=OS_DISK_URI]${OS_DISK_URI}"
echo "##vso[task.setvariable variable=MANAGED_SIG_ID]${MANAGED_SIG_ID}"
