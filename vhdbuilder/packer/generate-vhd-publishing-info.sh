#!/bin/bash -e

required_env_vars=(
    "WINDOWS_SKU"
)

for v in "${required_env_vars[@]}"
do
    if [ -z "${!v}" ]; then
        echo "$v was not set!"
        exit 1
    fi
done

curl -L -o  vhd-publishing-info.json https://testxx3e.blob.core.windows.net/0223newimages/${WINDOWS_SKU}-vhd-publishing-info.json

# Do not log sas token
sed 's/?.*\",/?***\",/g' < vhd-publishing-info.json
