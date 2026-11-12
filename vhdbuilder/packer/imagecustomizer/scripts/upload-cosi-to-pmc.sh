#!/bin/bash
set -euo pipefail

# Uploads the COSI staged by convert-vhd-to-cosi.sh to PMC's AFD upload endpoint
# under PMC's service connection. AzCopy handles chunked block-blob uploads
# and authenticates via the AzureCLI@2 task's Microsoft Entra login.

required_env_vars=(
    "CAPTURED_SIG_VERSION"
    "SKU_NAME"
    "AFD_UPLOAD_ENDPOINT"
    "COSI_CONTAINER"
)

for v in "${required_env_vars[@]}"
do
    if [ -z "${!v:-}" ]; then
        echo "$v was not set!"
        exit 1
    fi
done

COSI_WORK_DIR="$(pwd)"
COSI_NAME="${SKU_NAME}-${CAPTURED_SIG_VERSION}.cosi"
STAGED_COSI="${COSI_WORK_DIR}/${COSI_NAME}"

if [ ! -f "$STAGED_COSI" ]; then
    echo "##vso[task.logissue type=error]Staged COSI not found at ${STAGED_COSI}; the convert-vhd-to-cosi step must run first"
    exit 1
fi

# Only trust the configured HTTPS host, not a wildcard or a list of suffixes.
# This script runs under Bash; the regex capture supplies the trusted hostname.
# shellcheck disable=SC3010
if [[ ! "${AFD_UPLOAD_ENDPOINT}" =~ ^https://([[:alnum:]][[:alnum:].-]*)/?$ ]]; then
    echo "##vso[task.logissue type=error]AFD_UPLOAD_ENDPOINT must be an HTTPS hostname without a path, query, or wildcard"
    exit 1
fi
AFD_UPLOAD_HOST="${BASH_REMATCH[1]}"

export AZCOPY_AUTO_LOGIN_TYPE=AZCLI
export AZCOPY_CONCURRENCY_VALUE=8
if ! AZCOPY_TENANT_ID="$(az account show --query tenantId --output tsv)" || [ -z "$AZCOPY_TENANT_ID" ]; then
    echo "##vso[task.logissue type=error]Unable to read the Azure CLI tenant for the azcopy login"
    exit 1
fi
export AZCOPY_TENANT_ID

COSI_UPLOAD_URL="${AFD_UPLOAD_ENDPOINT%/}/${COSI_CONTAINER}/${COSI_NAME}"
echo "Uploading ${STAGED_COSI} to ${COSI_UPLOAD_URL}"
azcopy copy \
    "${STAGED_COSI}" \
    "${COSI_UPLOAD_URL}" \
    --from-to LocalBlob \
    --blob-type BlockBlob \
    --block-size-mb 16 \
    --overwrite true \
    --trusted-microsoft-suffixes "${AFD_UPLOAD_HOST}"
echo "Successfully uploaded ${COSI_UPLOAD_URL}"
