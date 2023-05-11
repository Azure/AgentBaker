#!/bin/bash
set -o pipefail

build-packer() {
  if [[ "ARM64" == "$ARCHITECTURE" ]]; then

    echo "${MODE}: Building with Hyper-v generation 2 ARM64 VM"

    if [[ "Ubuntu" == "$OS_SKU" ]]; then
      echo "Using packer template file vhd-image-builder-arm64-gen2.json"
      packer build -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-arm64-gen2.json
    elif [[ "CBLMariner" == "$OS_SKU" ]]; then
      echo "Using packer template file vhd-image-builder-mariner-arm64.json"
      packer build -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-mariner-arm64.json
    fi

    echo "${MODE}: Convert os disk snapshot to SIG"
    ./vhdbuilder/packer/convert-osdisk-snapshot-to-sig.sh

  elif [[ "X86_64" == "$ARCHITECTURE" ]]; then

    if [[ "V2" == "$HYPERV_GENERATION" ]]; then
     echo "${MODE}: Building with Hyper-v generation 2 x86_64 VM"
    elif [[ "V1" == "$HYPERV_GENERATION" ]]; then
      echo "${MODE}: Building with Hyper-v generation 1 X86_64 VM"
    fi

    if [[ "Ubuntu" == "$OS_SKU" ]]; then
      echo "Using packer template file: vhd-image-builder-base.json"
      packer build -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-base.json
    elif [[ "CBLMariner" == "$OS_SKU" ]]; then
      echo "Using packer template file vhd-image-builder-mariner.json"
      packer build -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-mariner.json
    fi

  fi
}

build-packer-windows() {
  if [[ "windowsVhdMode" == "$MODE" ]]; then
    if [[ "${SIG_FOR_PRODUCTION}" == "True" ]]; then
      if [[ "V1" == "${HYPERV_GENERATION}" ]]; then
        echo "${MODE}: Building with Hyper-v generation 1 VM and save to Classic Storage Account"
      else
        echo "${MODE}: Building with Hyper-v generation 2 VM and save to Classic Storage Account"
      fi
    else
      if [[ "V1" == "${HYPERV_GENERATION}" ]]; then
        echo "${MODE}: Building with Hyper-v generation 1 VM and save to Shared Image Gallery"
      else
        echo "${MODE}: Building with Hyper-v generation 2 VM and save to Shared Image Gallery"
      fi
    fi
    packer build -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/windows-vhd-builder-sig.json
  fi
}

az-login() {
  if [[ "Windows" == "${OS_TYPE}" ]]; then
    echo "Logging into Azure with service principal..."
    az login --service-principal -u "${CLIENT_ID}" -p "${CLIENT_SECRET}" --tenant "${TENANT_ID}"
  else
    echo "Logging into Azure with agent VM MSI..."
    az login --identity
  fi
  az account set -s "${SUBSCRIPTION_ID}"
}

init-packer() {
  ./vhdbuilder/packer/init-variables.sh
}

run-packer() {
  az-login
  packer version && init-packer | tee packer-output && build-packer | tee -a packer-output
}

run-packer-windows() {
  az-login
  packer version && init-packer | tee packer-output && build-packer-windows | tee -a packer-output
}

cleanup() {
  az-login
  ./vhdbuilder/packer/cleanup.sh
}

backfill-cleanup() {
  az-login
  ./vhdbuilder/packer/backfill-cleanup.sh
}

generate-sas() {
  az-login
  ./vhdbuilder/packer/generate-vhd-publishing-info.sh
}

convert-sig-to-classic-storage-account-blob() {
  az-login
  ./vhdbuilder/packer/convert-sig-to-classic-storage-account-blob.sh
}

windows-vhd-publishing-info() {
  az-login
  ./vhdbuilder/packer/generate-windows-vhd-publishing-info.sh
}

test-building-vhd() {
  az-login
  ./vhdbuilder/packer/test/run-test.sh
}


# This allows for calling ./packer.sh functions from the command line
# Ex: $ ./packer.sh run-packer
# Check if the function exists
if declare -f "$1" > /dev/null
then
  # call arguments verbatim
  "$@"
else
  # Show a helpful error
  echo "'$1' is not a known function name" >&2
  exit 1
fi