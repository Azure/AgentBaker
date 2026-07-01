#!/bin/bash
set -euo pipefail

# Uploads the COSI staged by convert-vhd-to-cosi.sh to PMC's AFD upload endpoint,
# under PMC's service connection. No az-login dep (azcopy's data-plane token needs
# no active subscription, and 'az account set' would fail under PMC's identity).

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

echo "Setting azcopy environment variables"
export AZCOPY_AUTO_LOGIN_TYPE="AZCLI"
export AZCOPY_CONCURRENCY_VALUE="AUTO"
export AZCOPY_LOG_LOCATION="${COSI_WORK_DIR}/azcopy-cosi-upload-log-files/"
export AZCOPY_JOB_PLAN_LOCATION="${COSI_WORK_DIR}/azcopy-cosi-upload-job-plan-files/"
mkdir -p "${AZCOPY_LOG_LOCATION}"
mkdir -p "${AZCOPY_JOB_PLAN_LOCATION}"

echo "Uploading COSI to ${COSI_UPLOAD_URL}"
if azcopy copy "$STAGED_COSI" "$COSI_UPLOAD_URL" --from-to=LocalBlob; then
    echo "Successfully uploaded COSI to ${COSI_UPLOAD_URL}"
else
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
    exit "$azExitCode"
fi
