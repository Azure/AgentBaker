#!/bin/bash -e

UPLOAD=${1:-false}

export AZCOPY_AUTO_LOGIN_TYPE="MSI"
export AZCOPY_MSI_RESOURCE_STRING="$AZURE_MSI_RESOURCE_STRING"
export AZCOPY_CONCURRENCY_VALUE="AUTO"

if [ "${UPLOAD}" == "true"]; then
  azcopy copy "./${CAPTURED_SIG_VERSION}.vhd" "${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd" --overwrite=true
else
  azcopy copy "${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd" "./${CAPTURED_SIG_VERSION}.vhd"
fi


