#!/usr/bin/env bash
set -e

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
# windows/vYYYYMMDD.

echo "Checking SourceBranch: ${BRANCH}"
if [[ -n "${IS_RELEASE_PIPELINE}" ]]; then
  if [[ "${DRY_RUN}" = "True" ]]; then
    echo "This is a test build triggered from the release pipeline"
  else
    echo "This is a release build triggered from the release pipeline. DRY_RUN=${DRY_RUN}"

    echo "${BRANCH}" | grep -E '^refs/heads/windows/v[[:digit:]]{8}$'
    if (( $? != 0 )); then
      echo "The branch ${BRANCH} is not release branch. Please use the release branch. Release branch name format: windows/vYYYYMMDD."
      exit 1
    fi
  fi
else
  echo "This is a test build triggered from the test pipeline"
  echo "##vso[task.setvariable variable=DRY_RUN]True";
fi

# This next segment sets build variables - most importantly the VHD name and version we're building.
# Merge gen1, gen2, and sig modes into one mode for Windows VHD builds - use sig only.
# 1. If sig is for test purpose only, SIG_GALLERY_NAME, SIG_IMAGE_NAME_PREFIX, and SIG_IMAGE_VERSION are set.
#     Task variable SIG_FOR_PRODUCTION is set to False and passed to the following steps.
# 2. If sig is for production, we will hard-code the task variables SIG_GALLERY_NAME, SIG_IMAGE_NAME, and SIG_IMAGE_VERSION.
#     $RANDOM is appended to avoid duplicate gallery name running concurrent builds.
#     Task variable SIG_FOR_PRODUCTION is set to True and passed to the following steps.
#     Built sig will be deleted because it has been converted to VHD, and thus not needed.

m="windowsVhdMode"
echo "Set build mode to $m"
echo "##vso[task.setvariable variable=MODE]$m"

echo "Original SIG_GALLERY_NAME: ${SIG_GALLERY_NAME}"
echo "Original SIG_IMAGE_NAME_PREFIX: ${SIG_IMAGE_NAME_PREFIX}"
echo "Original SIG_IMAGE_VERSION: ${SIG_IMAGE_VERSION}"

# -n is "not empty"
if [[ -n ${SIG_GALLERY_NAME} && -n ${SIG_IMAGE_NAME_PREFIX} && -n ${SIG_IMAGE_VERSION} ]]; then
    echo "All of Name, Prefix, and Version have been set"
    SIG_IMAGE_NAME="${SIG_IMAGE_NAME_PREFIX}-${WINDOWS_SKU}"
    echo "##vso[task.setvariable variable=SIG_FOR_PRODUCTION]False"
else
    echo "At least on of the name, prefix or version are empty. Overwriting all values. "
    SIG_IMAGE_VERSION="$(date +"%y%m%d").$(date +"%H%M%S").$RANDOM"
    SIG_IMAGE_NAME="aks-windows-${WINDOWS_SKU}"
    SIG_GALLERY_NAME="WSGallery$(date +"%y%m%d")"
    SIG_GALLERY_NAME="PackerSigGalleryEastUS"

    WS_SKU=$(echo $WINDOWS_SKU | tr '-' '_')

    # This enables the VHD to be uploaded to a classic storage account if DRY_RUN is false.
    echo "##vso[task.setvariable variable=SIG_FOR_PRODUCTION]True"
fi

if [[ "${USE_RELEASE_DATE}" = "False" ]]; then
  echo "use current date as build date";  BUILD_DATE=$(date +"%y%m%d")
else
  echo "use release date as build date"
  echo "${RELEASE_DATE}" | grep -E '[[:digit:]]{6}'
  if (( $? != 0 )); then
    echo "The release date ${RELEASE_DATE} is not valid date. Release date format: YYMMDD."
    exit 1
  fi
  BUILD_DATE=${RELEASE_DATE}
fi
echo "Default BUILD_DATE is $BUILD_DATE"
if [[ -n "${CUSTOM_BUILD_DATE}" ]]; then
  echo "set BUILD_DATE to ${CUSTOM_BUILD_DATE}"
  BUILD_DATE=${CUSTOM_BUILD_DATE}
fi

echo "Modified SIG_IMAGE_VERSION: ${SIG_IMAGE_VERSION}"
echo "Modified SIG_IMAGE_NAME: ${SIG_IMAGE_NAME}"
echo "Modified SIG_GALLERY_NAME: ${SIG_GALLERY_NAME}"
echo "Set build date to $BUILD_DATE"

echo "##vso[task.setvariable variable=SIG_GALLERY_NAME]$SIG_GALLERY_NAME"
echo "##vso[task.setvariable variable=SIG_IMAGE_NAME]$SIG_IMAGE_NAME"
echo "##vso[task.setvariable variable=SIG_IMAGE_VERSION]$SIG_IMAGE_VERSION"
echo "##vso[task.setvariable variable=SKIPVALIDATEREOFFERUPDATE]True"
echo "##vso[task.setvariable variable=BUILD_DATE]$BUILD_DATE"
echo "##vso[task.setvariable variable=DRY_RUN]${DRY_RUN}"

# Finally, we invoke packer to build the VHD.
make -f packer.mk az-login
packer init ./vhdbuilder/packer/packer-plugin.pkr.hcl | tee -a packer-output
packer version | tee -a packer-output
./vhdbuilder/packer/produce-packer-settings.sh | tee -a packer-output
packer build -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/windows/windows-vhd-builder-sig.json
