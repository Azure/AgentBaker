#!/bin/bash -e

# Generate cosi-publishing-info.json with metadata about the COSI artifact
# in immutable storage. Parallels generate-vhd-publishing-info.sh for VHDs.

required_env_vars=(
    "STORAGE_ACCT_BLOB_URL"
    "CAPTURED_SIG_VERSION"
    "OS_NAME"
    "OFFER_NAME"
    "SKU_NAME"
    "HYPERV_GENERATION"
)

for v in "${required_env_vars[@]}"
do
    if [ -z "${!v}" ]; then
        echo "$v was not set!"
        exit 1
    fi
done

if [ -z "$IMAGE_VERSION" ]; then
    IMAGE_VERSION=$(date +%Y%m.%d.0)
    echo "IMAGE_VERSION was not set, defaulting to ${IMAGE_VERSION}"
fi

if [ "${ARCHITECTURE,,}" = "arm64" ]; then
    IMAGE_ARCH="Arm64"
else
    IMAGE_ARCH="x64"
fi

COSI_NAME="${CAPTURED_SIG_VERSION}.cosi"
cosi_url="${STORAGE_ACCT_BLOB_URL}/${COSI_NAME}"
echo "COSI URL ---> ${cosi_url}"

cat <<EOF > cosi-publishing-info.json
{
    "cosi_url": "$cosi_url",
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
