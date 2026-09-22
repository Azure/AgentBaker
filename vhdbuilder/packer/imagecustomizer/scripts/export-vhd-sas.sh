#!/bin/bash
set -euo pipefail

export AZCOPY_AUTO_LOGIN_TYPE="AZCLI"
export AZCOPY_CONCURRENCY_VALUE="AUTO"
vhd_name="${CAPTURED_SIG_VERSION}.vhd"

echo "Copying ${DESTINATION_STORAGE_CONTAINER}/${vhd_name} to immutable container ${VHD_CONTAINER_NAME}"
az storage blob copy start --account-name "$STORAGE_ACCOUNT_NAME" \
    --destination-blob "$vhd_name" --destination-container "$VHD_CONTAINER_NAME" \
    --source-uri "${DESTINATION_STORAGE_CONTAINER}/${vhd_name}" --auth-mode login
for ((attempt = 0; attempt < 120; attempt++)); do
    copy_status="$(az storage blob show --account-name "$STORAGE_ACCOUNT_NAME" \
        --container-name "$VHD_CONTAINER_NAME" --name "$vhd_name" --auth-mode login \
        --query properties.copy.status --output tsv)"
    case "$copy_status" in
        success) break ;;
        pending) sleep 15 ;;
        *)
            echo "##vso[task.logissue type=error]VHD copy to immutable container finished with status ${copy_status}"
            exit 1
            ;;
    esac
done
if [ "$copy_status" != "success" ]; then
    echo "##vso[task.logissue type=error]Timed out waiting for the immutable VHD copy"
    exit 1
fi

expiry="$(date -u -d '+8 hours' '+%Y-%m-%dT%H:%MZ')"
sas_url="$(az storage blob generate-sas --account-name "$STORAGE_ACCOUNT_NAME" \
    --container-name "$VHD_CONTAINER_NAME" --name "$vhd_name" --permissions r \
    --expiry "$expiry" --https-only --as-user --auth-mode login --full-uri --output tsv)"
echo "##vso[task.setvariable variable=VHD_SAS_URL;isOutput=true;issecret=true]$sas_url"
echo "Removing staging copy of ${vhd_name}"
azcopy remove "${DESTINATION_STORAGE_CONTAINER}/${vhd_name}" --recursive=true