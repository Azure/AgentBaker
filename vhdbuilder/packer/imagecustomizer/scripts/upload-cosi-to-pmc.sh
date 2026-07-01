#!/bin/bash
set -euo pipefail

# Uploads the COSI staged by convert-vhd-to-cosi.sh to PMC's AFD upload endpoint
# under PMC's service connection, via a direct Blob REST PUT authenticated with an
# AAD storage-scoped token. AFD forwards the Bearer token to the blob origin, so
# azcopy is not used (it cannot infer the service type for a custom *.azurefd.net host).

required_env_vars=(
    "CAPTURED_SIG_VERSION"
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
COSI_NAME="${CAPTURED_SIG_VERSION}.cosi"
STAGED_COSI="${COSI_WORK_DIR}/${COSI_NAME}"
COSI_UPLOAD_URL="${AFD_UPLOAD_ENDPOINT%/}/${COSI_CONTAINER}/${COSI_NAME}"

if [ ! -f "$STAGED_COSI" ]; then
    echo "##vso[task.logissue type=error]Staged COSI not found at ${STAGED_COSI}; the convert-vhd-to-cosi step must run first"
    exit 1
fi

RESP_FILE="${COSI_WORK_DIR}/cosi-afd-upload-response.txt"

echo "Uploading COSI to ${COSI_UPLOAD_URL}"
STORAGE_TOKEN="$(az account get-access-token --resource https://storage.azure.com --query accessToken -o tsv)"
HTTP_CODE="$(curl -sS -o "${RESP_FILE}" -w '%{http_code}' \
    -X PUT \
    -H "Authorization: Bearer ${STORAGE_TOKEN}" \
    -H "x-ms-blob-type: BlockBlob" \
    -H "x-ms-version: 2020-10-02" \
    -H "Content-Type: application/octet-stream" \
    -T "${STAGED_COSI}" \
    "${COSI_UPLOAD_URL}")"

if [ "${HTTP_CODE}" = "201" ]; then
    echo "Successfully uploaded COSI to ${COSI_UPLOAD_URL} (HTTP ${HTTP_CODE})"
else
    echo "##vso[task.logissue type=error]COSI upload to ${COSI_UPLOAD_URL} failed with HTTP ${HTTP_CODE}"
    cat "${RESP_FILE}" || true
    exit 1
fi
