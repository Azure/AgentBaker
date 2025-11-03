#!/bin/bash -e

required_env_vars=(
  "VHD_FILE"
)

for v in "${required_env_vars[@]}"
do
    if [ -z "${!v}" ]; then
        echo "$v was not set!"
        exit 1
    fi
done

if [ ! -f "$VHD_FILE" ]; then
  echo "Error: file ${VHD_FILE} not found"
  exit 1
fi

HASH_FILE=""
SIG_FILE=""




