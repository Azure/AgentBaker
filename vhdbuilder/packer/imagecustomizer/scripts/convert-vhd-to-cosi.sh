#!/bin/bash
set -euo pipefail

# Converts an ACL VHD from blob storage to COSI format using ImageCustomizer,
# then uploads the COSI file back to blob storage.

required_env_vars=(
    "DESTINATION_STORAGE_CONTAINER"
    "CAPTURED_SIG_VERSION"
    "IMG_CUSTOMIZER_CONTAINER"
    "IMG_CUSTOMIZER_VERSION"
)

for v in "${required_env_vars[@]}"
do
    if [ -z "${!v}" ]; then
        echo "$v was not set!"
        exit 1
    fi
done

# Optional GHCR fallback: when the MCR ImageCustomizer image for the requested
# version is unavailable, optionally fall back to pulling the published GitHub
# Container Registry image (IMG_CUSTOMIZER_CONTAINER_FALLBACK). Gated by the
# first script argument and defaults to "false" so the fallback is opt-in.
ALLOW_GHCR_FALLBACK="${1:-false}"

WORK_DIR="$(pwd)/cosi-convert"
mkdir -p "$WORK_DIR/build" "$WORK_DIR/out"

cleanup() {
    echo "Cleaning up working directory $WORK_DIR"
    rm -rf "$WORK_DIR"
}
trap cleanup EXIT

VHD_BLOB_URL="${DESTINATION_STORAGE_CONTAINER}/${CAPTURED_SIG_VERSION}.vhd"
LOCAL_VHD="$WORK_DIR/${CAPTURED_SIG_VERSION}.vhd"
LOCAL_COSI="$WORK_DIR/out/${CAPTURED_SIG_VERSION}.cosi"

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

IMG_CUSTOMIZER_REF="${IMG_CUSTOMIZER_CONTAINER}:${IMG_CUSTOMIZER_VERSION}"

echo "Pulling ImageCustomizer image ${IMG_CUSTOMIZER_REF}"
if ! docker pull "${IMG_CUSTOMIZER_REF}"; then
    if [ "${ALLOW_GHCR_FALLBACK,,}" != "true" ]; then
        echo "##vso[task.logissue type=error]Failed to pull ImageCustomizer image ${IMG_CUSTOMIZER_REF} and GHCR fallback is disabled"
        exit 1
    fi

    if [ -z "${IMG_CUSTOMIZER_CONTAINER_FALLBACK:-}" ]; then
        echo "##vso[task.logissue type=error]GHCR fallback is enabled but IMG_CUSTOMIZER_CONTAINER_FALLBACK is not set"
        exit 1
    fi

    # GHCR only publishes fully-qualified semver tags (e.g. 1.5.0), whereas MCR
    # exposes moving minor tags (e.g. 1.5). Normalize a major.minor version to
    # major.minor.0 so the fallback tag resolves correctly.
    FALLBACK_VERSION="${IMG_CUSTOMIZER_VERSION}"
    if [[ "${FALLBACK_VERSION}" =~ ^[0-9]+\.[0-9]+$ ]]; then
        FALLBACK_VERSION="${FALLBACK_VERSION}.0"
    fi

    IMG_CUSTOMIZER_REF="${IMG_CUSTOMIZER_CONTAINER_FALLBACK}:${FALLBACK_VERSION}"
    echo "MCR image unavailable; falling back to ${IMG_CUSTOMIZER_REF}"
    if ! docker pull "${IMG_CUSTOMIZER_REF}"; then
        echo "##vso[task.logissue type=error]Failed to pull ImageCustomizer fallback image ${IMG_CUSTOMIZER_REF}"
        exit 1
    fi
fi

echo "Converting VHD to COSI using ImageCustomizer ${IMG_CUSTOMIZER_REF}"
docker run \
    --rm \
    --interactive \
    --privileged=true \
    -v "$WORK_DIR:/convert" \
    -v /dev:/dev \
    "${IMG_CUSTOMIZER_REF}" \
    convert \
        --log-level "debug" \
        --build-dir /convert/build \
        --image-file "/convert/${CAPTURED_SIG_VERSION}.vhd" \
        --output-image-file "/convert/out/${CAPTURED_SIG_VERSION}.cosi" \
        --output-image-format cosi

if [ ! -f "$LOCAL_COSI" ]; then
    echo "##vso[task.logissue type=error]COSI file was not created at ${LOCAL_COSI}"
    exit 1
fi

echo "Uploading COSI to ${DESTINATION_STORAGE_CONTAINER}/${CAPTURED_SIG_VERSION}.cosi"
if ! azcopy copy "$LOCAL_COSI" "${DESTINATION_STORAGE_CONTAINER}/${CAPTURED_SIG_VERSION}.cosi" --recursive=true; then
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

echo "Successfully converted and uploaded COSI: ${DESTINATION_STORAGE_CONTAINER}/${CAPTURED_SIG_VERSION}.cosi"

# Generate cosi-publishing-info.json for downstream consumption
COSI_SHA256=$(sha256sum "$LOCAL_COSI" | awk '{print $1}')
COSI_SIZE=$(stat -c%s "$LOCAL_COSI")

if [ -z "$IMAGE_VERSION" ]; then
    IMAGE_VERSION=$(date +%Y%m.%d.0)
    echo "IMAGE_VERSION was not set, defaulting to ${IMAGE_VERSION}"
fi

if [ "${ARCHITECTURE,,}" = "arm64" ]; then
    IMAGE_ARCH="Arm64"
else
    IMAGE_ARCH="x64"
fi

# OS_NAME is always Linux for ACL COSI artifacts
OS_NAME="Linux"

COSI_NAME="${CAPTURED_SIG_VERSION}.cosi"
cosi_url="${STORAGE_ACCT_BLOB_URL}/${COSI_NAME}"

cat <<EOF > cosi-publishing-info.json
{
    "cosi_url": "$cosi_url",
    "sha256": "${COSI_SHA256}",
    "size_bytes": ${COSI_SIZE},
    "os_name": "$OS_NAME",
    "sku_name": "$SKU_NAME",
    "offer_name": "$OFFER_NAME",
    "hyperv_generation": "${HYPERV_GENERATION}",
    "image_architecture": "${IMAGE_ARCH}",
    "image_version": "${IMAGE_VERSION}"
}
EOF

echo "Generated cosi-publishing-info.json:"
cat cosi-publishing-info.json
