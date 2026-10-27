#!/bin/bash
set -euo pipefail

if [[ "${OS_SKU:-}" != "AzureContainerLinux" ]]; then
    echo "ACL overlay customization must not run for other OS SKUs" >&2
    exit 1
fi
for name in DESTINATION_STORAGE_CONTAINER CAPTURED_SIG_VERSION IMG_CUSTOMIZER_CONTAINER ARCHITECTURE; do
    if [[ -z "${!name:-}" ]]; then
        echo "$name must be set" >&2
        exit 1
    fi
done

case "${ARCHITECTURE,,}:$(uname -m)" in
    arm64:aarch64|arm64:arm64) platform=linux/arm64 ;;
    x86_64:x86_64) platform=linux/amd64 ;;
    *) echo "ImageCustomizer requires a worker matching $ARCHITECTURE" >&2; exit 1 ;;
esac

work_dir="$(mktemp -d "$(pwd)/acl-overlays.XXXXXX")"
trap 'rm -rf -- "$work_dir"' EXIT
mkdir -p "$work_dir/build" "$work_dir/out"
export AZCOPY_AUTO_LOGIN_TYPE=AZCLI AZCOPY_CONCURRENCY_VALUE=AUTO
export AZCOPY_LOG_LOCATION="$work_dir/azcopy-logs"
export AZCOPY_JOB_PLAN_LOCATION="$work_dir/azcopy-plans"

vhd_url="${DESTINATION_STORAGE_CONTAINER}/${CAPTURED_SIG_VERSION}.vhd"
azcopy copy "$vhd_url" "$work_dir/input.vhd"
customizer="$IMG_CUSTOMIZER_CONTAINER"
if ! docker pull --platform "$platform" "$customizer"; then
    customizer="${IMG_CUSTOMIZER_CONTAINER_FALLBACK:?ImageCustomizer pull failed and no fallback is configured}"
    docker pull --platform "$platform" "$customizer"
fi

config_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../acl" && pwd)"
docker run --platform "$platform" --rm --privileged=true \
    -v "$work_dir:/customize" \
    -v "$work_dir/build:/build" \
    -v "$config_dir:/config:ro" \
    "$customizer" customize \
        --log-level debug \
        --build-dir /build \
        --config-file /config/acl.yml \
        --image-file /customize/input.vhd \
        --output-image-file /customize/out/image.vhd \
        --output-image-format vhd-fixed

test -s "$work_dir/out/image.vhd"
azcopy copy "$work_dir/out/image.vhd" "$vhd_url" --blob-type=PageBlob --overwrite=true
