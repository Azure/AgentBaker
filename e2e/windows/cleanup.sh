#!/bin/bash

set -euxo pipefail

# delete blobs created 3 days ago
EXPIRATION_IN_HOURS=72
(( expirationInSecs = ${EXPIRATION_IN_HOURS} * 60 * 60 ))
(( deadline=$(date +%s)-${expirationInSecs%.*} ))
dateOfdeadline=$(date -d @${deadline} +"%Y-%m-%dT%H:%M:%S+00:00")

# two containers need to be cleaned up now
CONTAINER_LIST=("cselogs" "csepackages")

for CONTAINER_NAME in "${CONTAINER_LIST[@]}"
do 
    result=$(az storage blob list -c $CONTAINER_NAME --account-name $STORAGE_ACCOUNT_NAME --account-key $MAPPED_ACCOUNT_KEY -o json \
    | jq -r --arg time "$dateOfdeadline" '.[] | select(.properties.creationTime < $time)' \
    | jq -r '.name')

    for item in $result
    do
        az storage blob delete -c $CONTAINER_NAME --account-name $STORAGE_ACCOUNT_NAME --account-key $MAPPED_ACCOUNT_KEY -n $item
        echo "Deleted $item in $CONTAINER_NAME"
    done
done