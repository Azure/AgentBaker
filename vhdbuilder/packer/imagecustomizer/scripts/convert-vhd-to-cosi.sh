#!/bin/bash
set -euo pipefail

# Converts an ACL VHD to COSI and stages it for the separate "Upload COSI to PMC"
# task (which runs under PMC's service connection).

# Nebraska requires strict SemVer, so strip per-component leading zeros (e.g.
# 202608.06.0 -> 202608.6.0) and append -fips for FIPS builds.
generate_cosi_package_version() {
    local raw_version="$1"
    local enable_fips="$2"

    # Drop any existing -fips so it is never duplicated; ENABLE_FIPS re-adds it.
    local core="$raw_version"
    case "${raw_version,,}" in
        *-fips) core="${raw_version%-*}" ;;
    esac

    # Require exactly three numeric components (major.minor.patch).
    local major minor patch extra
    IFS='.' read -r major minor patch extra <<<"$core"
    if [ -n "$extra" ]; then
        echo "generate_cosi_package_version: '${core}' has more than three components" >&2
        return 1
    fi
    local component
    for component in "$major" "$minor" "$patch"; do
        case "$component" in
            ''|*[!0-9]*)
                echo "generate_cosi_package_version: '${core}' must have exactly three numeric components" >&2
                return 1
                ;;
        esac
    done

    # 10# forces base-10 so values like 06 are not parsed as octal.
    local version="$((10#$major)).$((10#$minor)).$((10#$patch))"
    if [ "${enable_fips,,}" = "true" ]; then
        version="${version}-fips"
    fi
    printf '%s\n' "$version"
}

# Source-guard: functions above are unit-tested; the flow below skips when sourced.
${__SOURCED__:+return}

required_env_vars=(
    "DESTINATION_STORAGE_CONTAINER"
    "CAPTURED_SIG_VERSION"
    "IMG_CUSTOMIZER_CONTAINER"
    "AFD_DOWNLOAD_HOSTNAME"
    "COSI_CONTAINER"
)

for v in "${required_env_vars[@]}"
do
    if [ -z "${!v:-}" ]; then
        echo "$v was not set!"
        exit 1
    fi
done

# Optional GHCR fallback: when the MCR ImageCustomizer image
# (IMG_CUSTOMIZER_CONTAINER, including its tag) is unavailable, optionally fall
# back to pulling the published GitHub Container Registry image
# (IMG_CUSTOMIZER_CONTAINER_FALLBACK, also including its tag). Gated by the
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

COSI_NAME="${CAPTURED_SIG_VERSION}.cosi"
COSI_DOWNLOAD_URL="https://${AFD_DOWNLOAD_HOSTNAME}/${COSI_CONTAINER}/${COSI_NAME}"

echo "Setting azcopy environment variables"
export AZCOPY_AUTO_LOGIN_TYPE="AZCLI"
export AZCOPY_CONCURRENCY_VALUE="AUTO"
export AZCOPY_LOG_LOCATION="$WORK_DIR/azcopy-log-files/"
export AZCOPY_JOB_PLAN_LOCATION="$WORK_DIR/azcopy-job-plan-files/"
mkdir -p "${AZCOPY_LOG_LOCATION}"
mkdir -p "${AZCOPY_JOB_PLAN_LOCATION}"

echo "Downloading VHD from ${VHD_BLOB_URL}"
if azcopy copy "$VHD_BLOB_URL" "$LOCAL_VHD" --recursive=true; then
    echo "Downloaded VHD to ${LOCAL_VHD}"
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
    echo "Failed to download VHD, exiting with code $azExitCode"
    exit "$azExitCode"
fi

# Run the arch-matched ImageCustomizer image so its objcopy can read the VHD's
# UKI kernel cmdline. ARCH_LOWER is reused for image_architecture below. This
# convert job always runs on an agent whose host arch matches ARCHITECTURE (arm64
# ACL converts run on a native arm64 pool via a separate job); no emulation.
ARCH_LOWER="${ARCHITECTURE:-}"
if [ "${ARCH_LOWER,,}" = "arm64" ]; then
    IMG_CUSTOMIZER_PLATFORM="linux/arm64"
else
    IMG_CUSTOMIZER_PLATFORM="linux/amd64"
fi

IMG_CUSTOMIZER_REF="${IMG_CUSTOMIZER_CONTAINER}"

echo "Pulling ImageCustomizer image ${IMG_CUSTOMIZER_REF} for platform ${IMG_CUSTOMIZER_PLATFORM}"
if ! docker pull --platform "${IMG_CUSTOMIZER_PLATFORM}" "${IMG_CUSTOMIZER_REF}"; then
    if [ "${ALLOW_GHCR_FALLBACK,,}" != "true" ]; then
        echo "##vso[task.logissue type=error]Failed to pull ImageCustomizer image ${IMG_CUSTOMIZER_REF} and GHCR fallback is disabled"
        exit 1
    fi

    if [ -z "${IMG_CUSTOMIZER_CONTAINER_FALLBACK:-}" ]; then
        echo "##vso[task.logissue type=error]GHCR fallback is enabled but IMG_CUSTOMIZER_CONTAINER_FALLBACK is not set"
        exit 1
    fi

    IMG_CUSTOMIZER_REF="${IMG_CUSTOMIZER_CONTAINER_FALLBACK}"
    echo "MCR image unavailable; falling back to ${IMG_CUSTOMIZER_REF}"
    if ! docker pull --platform "${IMG_CUSTOMIZER_PLATFORM}" "${IMG_CUSTOMIZER_REF}"; then
        echo "##vso[task.logissue type=error]Failed to pull ImageCustomizer fallback image ${IMG_CUSTOMIZER_REF}"
        exit 1
    fi
fi

# No -v /dev:/dev: IC 1.5's entrypoint mounts its own devtmpfs on /dev, so bind-
# mounting the host's fails "devtmpfs already mounted on /dev" (exit 32) under
# QEMU. -v /dev:/dev is only for IC v1.4 and below; v1.5+ uses --privileged alone.
echo "Converting VHD to COSI using ImageCustomizer ${IMG_CUSTOMIZER_REF} (${IMG_CUSTOMIZER_PLATFORM})"
docker run \
    --platform "${IMG_CUSTOMIZER_PLATFORM}" \
    --rm \
    --interactive \
    --privileged=true \
    -v "$WORK_DIR:/convert" \
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

# Move out of WORK_DIR (removed by the cleanup trap) so the upload task can find it.
STAGED_COSI="$(pwd)/${COSI_NAME}"
mv "$LOCAL_COSI" "$STAGED_COSI"
echo "Staged COSI for upload at ${STAGED_COSI}"

# cosi-publishing-info.json for aks-rp 'cosi register' (needs both sha256 and sha1).
COSI_SHA256=$(sha256sum "$STAGED_COSI" | awk '{print $1}')
COSI_SHA1=$(sha1sum "$STAGED_COSI" | awk '{print $1}')
COSI_SIZE=$(stat -c%s "$STAGED_COSI")

if [ -z "${IMAGE_VERSION:-}" ]; then
    IMAGE_VERSION=$(date +%Y%m.%d.0)
    echo "IMAGE_VERSION was not set, defaulting to ${IMAGE_VERSION}"
fi

# Normalize only the Nebraska-facing version; IMAGE_VERSION itself is unchanged.
if ! COSI_IMAGE_VERSION="$(generate_cosi_package_version "${IMAGE_VERSION}" "${ENABLE_FIPS:-false}")"; then
    echo "##vso[task.logissue type=error]Failed to derive a strict-SemVer COSI package version from IMAGE_VERSION='${IMAGE_VERSION}' (ENABLE_FIPS='${ENABLE_FIPS:-false}')"
    exit 1
fi
echo "Normalized COSI package version: ${COSI_IMAGE_VERSION} (from IMAGE_VERSION=${IMAGE_VERSION}, ENABLE_FIPS=${ENABLE_FIPS:-false})"

# ARCH_LOWER is derived once near the top of the script and reused here.
if [ "${ARCH_LOWER,,}" = "arm64" ]; then
    IMAGE_ARCH="Arm64"
else
    IMAGE_ARCH="x64"
fi

# OS_NAME is always Linux for ACL COSI artifacts
OS_NAME="Linux"

cat <<EOF > cosi-publishing-info.json
{
    "cosi_url": "${COSI_DOWNLOAD_URL}",
    "sha256": "${COSI_SHA256}",
    "sha1": "${COSI_SHA1}",
    "size_bytes": ${COSI_SIZE},
    "os_name": "$OS_NAME",
    "sku_name": "${SKU_NAME:-}",
    "offer_name": "${OFFER_NAME:-}",
    "hyperv_generation": "${HYPERV_GENERATION:-}",
    "image_architecture": "${IMAGE_ARCH}",
    "image_version": "${COSI_IMAGE_VERSION}"
}
EOF

echo "Generated cosi-publishing-info.json:"
cat cosi-publishing-info.json
