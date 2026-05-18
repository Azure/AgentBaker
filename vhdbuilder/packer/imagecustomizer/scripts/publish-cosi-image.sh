#!/bin/bash
set -euo pipefail

# Upload a COSI artifact to PMC Storage through AFD and register it in Nebraska.

SCRIPTS_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
CONFIG=$IMG_CUSTOMIZER_CONFIG
AGENTBAKER_DIR=$(realpath "$SCRIPTS_DIR/../../../../")
OUT_DIR="${AGENTBAKER_DIR}/out"

required_env_vars=(
    "IMG_CUSTOMIZER_CONFIG"
    "AFD_UPLOAD_ENDPOINT"
    "COSI_CONTAINER"
)

for v in "${required_env_vars[@]}"
do
    if [ -z "${!v}" ]; then
        echo "$v was not set!"
        exit 1
    fi
done

PUBLISHING_INFO="${OUT_DIR}/${CONFIG}/cosi-publishing-info.json"
if [ ! -f "$PUBLISHING_INFO" ]; then
    echo "Error: cosi-publishing-info.json not found at $PUBLISHING_INFO" >&2
    echo "Run build-cosi-image.sh first" >&2
    exit 1
fi

ARTIFACT_PATH=$(jq -r '.artifact_path' "$PUBLISHING_INFO")
IMAGE_VERSION=$(jq -r '.image_version' "$PUBLISHING_INFO")
SHA256=$(jq -r '.sha256' "$PUBLISHING_INFO")

if [ ! -f "$ARTIFACT_PATH" ]; then
    echo "Error: COSI artifact not found at $ARTIFACT_PATH" >&2
    exit 1
fi

BLOB_NAME="${CONFIG}-${IMAGE_VERSION}.cosi"
UPLOAD_URL="${AFD_UPLOAD_ENDPOINT}/${COSI_CONTAINER}/${BLOB_NAME}"

echo "Setting azcopy environment variables..."
export AZCOPY_AUTO_LOGIN_TYPE="AZCLI"
export AZCOPY_CONCURRENCY_VALUE="AUTO"

export AZCOPY_LOG_LOCATION="$(pwd)/azcopy-log-files/"
export AZCOPY_JOB_PLAN_LOCATION="$(pwd)/azcopy-job-plan-files/"
mkdir -p "${AZCOPY_LOG_LOCATION}"
mkdir -p "${AZCOPY_JOB_PLAN_LOCATION}"

echo "Uploading ${ARTIFACT_PATH} to ${UPLOAD_URL}"
if ! azcopy copy "$ARTIFACT_PATH" "$UPLOAD_URL" --recursive=true; then
    azExitCode=$?
    shopt -s nullglob
    for f in "${AZCOPY_LOG_LOCATION}"/*.log; do
        echo "Azcopy log file: $f"
        echo "##vso[build.uploadlog]$f"
        if grep -q '"level":"Error"' "$f"; then
            echo "log file $f contains errors"
            echo "##vso[task.logissue type=error]Azcopy log file $f contains errors"
            cat "$f"
        fi
    done
    shopt -u nullglob
    echo "Exiting with azcopy exit code $azExitCode"
    exit "$azExitCode"
fi

echo "COSI artifact uploaded successfully"
echo "  Blob: ${COSI_CONTAINER}/${BLOB_NAME}"
echo "  SHA256: ${SHA256}"
echo "  Version: ${IMAGE_VERSION}"

# If Nebraska registration is configured, run the cosi-extension CLI
if [ -n "${NEBRASKA_PUBLISHER_ENDPOINT:-}" ] && [ -n "${NEBRASKA_APP_ID:-}" ] && [ -n "${AFD_DOWNLOAD_HOSTNAME:-}" ]; then
    echo "Registering COSI artifact in Nebraska..."
    COSI_EXTENSION="${AGENTBAKER_DIR}/bin/cosi-extension"
    if [ ! -x "$COSI_EXTENSION" ]; then
        echo "Error: cosi-extension binary not found at $COSI_EXTENSION" >&2
        echo "Run 'make -f packer.mk build-cosi-extension' first" >&2
        exit 2
    fi

    "$COSI_EXTENSION" register \
        --nebraska-endpoint "$NEBRASKA_PUBLISHER_ENDPOINT" \
        --app-id "$NEBRASKA_APP_ID" \
        --afd-download-hostname "$AFD_DOWNLOAD_HOSTNAME" \
        --container "$COSI_CONTAINER" \
        --publishing-info "$PUBLISHING_INFO"
    echo "Nebraska registration complete"
else
    echo "Skipping Nebraska registration (NEBRASKA_PUBLISHER_ENDPOINT, NEBRASKA_APP_ID, or AFD_DOWNLOAD_HOSTNAME not set)"
fi
