#!/bin/bash
set -euo pipefail

# Uploads the COSI staged by convert-vhd-to-cosi.sh to PMC's AFD upload endpoint
# under PMC's service connection. Delegates the transfer to the cmd/cosi-upload Go
# tool, which uses the Azure Blob SDK's chunked block-blob upload (Put Block + Put
# Block List) so it is not subject to the single Put Blob size limit, and
# authenticates via the AzureCLI@2 task's Microsoft Entra login.

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

if [ ! -f "$STAGED_COSI" ]; then
    echo "##vso[task.logissue type=error]Staged COSI not found at ${STAGED_COSI}; the convert-vhd-to-cosi step must run first"
    exit 1
fi

UPLOADER="${COSI_WORK_DIR}/bin/cosi-upload"
if [ ! -x "$UPLOADER" ]; then
    echo "##vso[task.logissue type=error]cosi-upload binary not found at ${UPLOADER}; build it with 'go build -o bin/cosi-upload ./cmd/cosi-upload'"
    exit 1
fi

"$UPLOADER" \
    --endpoint "${AFD_UPLOAD_ENDPOINT%/}" \
    --container "${COSI_CONTAINER}" \
    --blob "${COSI_NAME}" \
    --file "${STAGED_COSI}"
