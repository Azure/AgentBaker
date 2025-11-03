#!/bin/bash -e

echo "Checking azcopy version..."
azcopy --version


export AZCOPY_AUTO_LOGIN_TYPE="MSI"
export AZCOPY_MSI_RESOURCE_STRING="$AZURE_MSI_RESOURCE_STRING"
export AZCOPY_CONCURRENCY_VALUE="AUTO"

azcopy copy "${CLASSIC_BLOB}/${CAPTURED_SIG_VERSION}.vhd" "./${CAPTURED_SIG_VERSION}.vhd"
