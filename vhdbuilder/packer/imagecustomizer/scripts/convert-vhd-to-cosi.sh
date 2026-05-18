#!/bin/bash
set -euo pipefail

# Converts an ACL VHD from blob storage to COSI format using ImageCustomizer,
# then uploads the COSI file to PMC Storage via AFD endpoint.

required_env_vars=(
    "DESTINATION_STORAGE_CONTAINER"
    "CAPTURED_SIG_VERSION"
    "IMG_CUSTOMIZER_CONTAINER"
    "IMG_CUSTOMIZER_VERSION"
    "AFD_UPLOAD_ENDPOINT"
    "COSI_CONTAINER"
    "IMG_CUSTOMIZER_CONFIG"
)

for v in "${required_env_vars[@]}"
do
    if [ -z "${!v}" ]; then
        echo "$v was not set!"
        exit 1
    fi
done

WORK_DIR="$(pwd)/cosi-convert"
mkdir -p "$WORK_DIR/build" "$WORK_DIR/out"

cleanup() {
    echo "Cleaning up working directory $WORK_DIR"
    rm -rf "$WORK_DIR"
}
trap cleanup EXIT

VHD_BLOB_URL="${DESTINATION_STORAGE_CONTAINER}/${CAPTURED_SIG_VERSION}.vhd"
LOCAL_VHD="$WORK_DIR/${CAPTURED_SIG_VERSION}.vhd"
COSI_BLOB_NAME="${IMG_CUSTOMIZER_CONFIG}-${CAPTURED_SIG_VERSION}.cosi"
LOCAL_COSI="$WORK_DIR/out/${COSI_BLOB_NAME}"

echo "Setting azcopy environment variables"
export AZCOPY_AUTO_LOGIN_TYPE="AZCLI"
export AZCOPY_CONCURRENCY_VALUE="AUTO"
export AZCOPY_LOG_LOCATION="$WORK_DIR/azcopy-log-files/"
export AZCOPY_JOB_PLAN_LOCATION="$WORK_DIR/azcopy-job-plan-files/"
mkdir -p "${AZCOPY_LOG_LOCATION}"
mkdir -p "${AZCOPY_JOB_PLAN_LOCATION}"

echo "Downloading VHD from ${VHD_BLOB_URL}"
if ! azcopy copy "$VHD_BLOB_URL" "$LOCAL_VHD" --recursive=true; then
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
    echo "Failed to download VHD, exiting with code $azExitCode"
    exit $azExitCode
fi
echo "Downloaded VHD to ${LOCAL_VHD}"

echo "Converting VHD to COSI using ImageCustomizer ${IMG_CUSTOMIZER_CONTAINER}:${IMG_CUSTOMIZER_VERSION}"
docker run \
    --rm \
    --interactive \
    --privileged=true \
    -v "$WORK_DIR:/convert" \
    -v /dev:/dev \
    "${IMG_CUSTOMIZER_CONTAINER}:${IMG_CUSTOMIZER_VERSION}" \
    imagecustomizer convert \
        --log-level "debug" \
        --build-dir /convert/build \
        --image-file "/convert/${CAPTURED_SIG_VERSION}.vhd" \
        --output-image-file "/convert/out/${COSI_BLOB_NAME}" \
        --output-image-format cosi

if [ ! -f "$LOCAL_COSI" ]; then
    echo "##vso[task.logissue type=error]COSI file was not created at ${LOCAL_COSI}"
    exit 1
fi

echo "Uploading COSI to ${AFD_UPLOAD_ENDPOINT}/${COSI_CONTAINER}/${COSI_BLOB_NAME}"
if ! azcopy copy "$LOCAL_COSI" "${AFD_UPLOAD_ENDPOINT}/${COSI_CONTAINER}/${COSI_BLOB_NAME}" --recursive=true; then
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
    echo "Failed to upload COSI, exiting with code $azExitCode"
    exit $azExitCode
fi

echo "Successfully converted and uploaded COSI: ${AFD_UPLOAD_ENDPOINT}/${COSI_CONTAINER}/${COSI_BLOB_NAME}"
