#!/bin/bash -e

required_env_vars=(
  "AZURE_MSI_RESOURCE_STRING"
  "CAPTURED_SIG_VERSION"
  "CLASSIC_BLOB"
)

for v in "${required_env_vars[@]}"
do
    if [ -z "${!v}" ]; then
        echo "$v was not set!"
        exit 1
    fi
done

UPLOAD=${1:-false}

export AZCOPY_AUTO_LOGIN_TYPE="MSI"
export AZCOPY_MSI_RESOURCE_STRING="$AZURE_MSI_RESOURCE_STRING"
export AZCOPY_CONCURRENCY_VALUE="AUTO"

if [ "${UPLOAD}" == "true"]; then
  azcopy copy "./${CAPTURED_SIG_VERSION}.vhd" "${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd" --overwrite=true
else
  azcopy copy "${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd" "./${CAPTURED_SIG_VERSION}.vhd"
fi


