#!/bin/bash
set -euo pipefail

# Enhanced build script with UEFI secure boot certificate support

# Required env vars declared by the pipeline
required_env_vars=(
    "IMG_CUSTOMIZER_CONTAINER"
    "IMG_CUSTOMIZER_VERSION"
    "IMG_CUSTOMIZER_CONFIG"
    "BASE_IMG"
    "BASE_IMG_VERSION"
)

# Optional UEFI certificate env vars
optional_env_vars=(
    "UEFI_CERT_FILE"
    "UEFI_CERT_BASE64"
    "ENABLE_UEFI_SECURE_BOOT"
)

for v in "${required_env_vars[@]}"
do
    if [ -z "${!v}" ]; then
        echo "$v was not set!"
        exit 1
    fi
done

# Find the absolute path of the directory containing this script
SCRIPTS_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
CONFIG=$IMG_CUSTOMIZER_CONFIG
AGENTBAKER_DIR=`realpath $SCRIPTS_DIR/../../../../`
BUILD_DIR="${AGENTBAKER_DIR}/build"
OUT_DIR="${AGENTBAKER_DIR}/out"
mkdir -p "$OUT_DIR"
mkdir -p "$BUILD_DIR"
mkdir -p "$BUILD_DIR/$CONFIG"

# Validate CONFIG and config file
CONFIG_FILE="$AGENTBAKER_DIR/vhdbuilder/packer/imagecustomizer/$CONFIG/$CONFIG.yml"
if [ ! -f "$CONFIG_FILE" ]; then
    echo "Error: Config file '$CONFIG_FILE' not found" >&2
    echo "Expected path: vhdbuilder/packer/imagecustomizer/$CONFIG/$CONFIG.yml" >&2
    exit 1
fi

if [ ! -r "$CONFIG_FILE" ]; then
    echo "Error: Config file '$CONFIG_FILE' is not readable" >&2
    exit 1
fi

# UEFI Certificate Processing
ENABLE_UEFI=${ENABLE_UEFI_SECURE_BOOT:-false}
CERT_DER_FILE="$BUILD_DIR/$CONFIG/uefi-cert.der"

if [ "${ENABLE_UEFI,,}" = "true" ]; then
    echo "=== UEFI Secure Boot Certificate Processing ==="
    
    # Method 1: Use provided certificate file
    if [ -n "${UEFI_CERT_FILE:-}" ] && [ -f "$UEFI_CERT_FILE" ]; then
        echo "Using UEFI certificate from file: $UEFI_CERT_FILE"
        
        # Determine if file is PEM or DER format
        if openssl x509 -inform PEM -in "$UEFI_CERT_FILE" -noout 2>/dev/null; then
            echo "Converting PEM certificate to DER format"
            openssl x509 -inform PEM -in "$UEFI_CERT_FILE" -outform DER -out "$CERT_DER_FILE"
        elif openssl x509 -inform DER -in "$UEFI_CERT_FILE" -noout 2>/dev/null; then
            echo "Certificate is already in DER format"
            cp "$UEFI_CERT_FILE" "$CERT_DER_FILE"
        else
            echo "Error: Invalid certificate format in $UEFI_CERT_FILE"
            exit 1
        fi
    
    # Method 2: Use base64 encoded certificate
    elif [ -n "${UEFI_CERT_BASE64:-}" ]; then
        echo "Using base64 encoded UEFI certificate"
        echo "$UEFI_CERT_BASE64" | base64 -d > "$CERT_DER_FILE"
        
        # Verify the decoded certificate
        if ! openssl x509 -inform DER -in "$CERT_DER_FILE" -noout 2>/dev/null; then
            echo "Error: Invalid base64 DER certificate"
            exit 1
        fi
    
    else
        echo "Error: UEFI secure boot enabled but no certificate provided"
        echo "Set UEFI_CERT_FILE or UEFI_CERT_BASE64 environment variable"
        exit 1
    fi
    
    # Verify and log certificate details
    echo "UEFI certificate processed successfully:"
    echo "  File: $CERT_DER_FILE"
    echo "  Size: $(wc -c < "$CERT_DER_FILE") bytes"
    echo "  SHA256: $(sha256sum "$CERT_DER_FILE" | cut -d' ' -f1)"
    
    # Extract certificate details for logging
    CERT_SUBJECT=$(openssl x509 -inform DER -in "$CERT_DER_FILE" -noout -subject 2>/dev/null | sed 's/subject=//')
    CERT_ISSUER=$(openssl x509 -inform DER -in "$CERT_DER_FILE" -noout -issuer 2>/dev/null | sed 's/issuer=//')
    CERT_NOT_BEFORE=$(openssl x509 -inform DER -in "$CERT_DER_FILE" -noout -startdate 2>/dev/null | sed 's/notBefore=//')
    CERT_NOT_AFTER=$(openssl x509 -inform DER -in "$CERT_DER_FILE" -noout -enddate 2>/dev/null | sed 's/notAfter=//')
    
    echo "  Subject: $CERT_SUBJECT"
    echo "  Issuer: $CERT_ISSUER"
    echo "  Valid From: $CERT_NOT_BEFORE"
    echo "  Valid To: $CERT_NOT_AFTER"
    
    # Create certificate staging directory for Image Customizer
    CERT_STAGING_DIR="$BUILD_DIR/$CONFIG/certs"
    mkdir -p "$CERT_STAGING_DIR"
    cp "$CERT_DER_FILE" "$CERT_STAGING_DIR/build-uefi-cert.der"
    
    # Copy UEFI installation scripts to staging area
    SCRIPTS_STAGING_DIR="$BUILD_DIR/$CONFIG/scripts"
    mkdir -p "$SCRIPTS_STAGING_DIR"
    cp "$AGENTBAKER_DIR/vhdbuilder/packer/imagecustomizer/scripts/install-uefi-certificate.sh" "$SCRIPTS_STAGING_DIR/"
    cp "$AGENTBAKER_DIR/vhdbuilder/packer/imagecustomizer/scripts/uefi-postinstall.sh" "$SCRIPTS_STAGING_DIR/"
    chmod +x "$SCRIPTS_STAGING_DIR"/*.sh
    
    echo "UEFI certificate and scripts staged for VHD build"
    
else
    echo "UEFI secure boot not enabled, skipping certificate processing"
fi

IMAGE_PATH="${OUT_DIR}/$CONFIG/$CONFIG.vhd"

BASE_IMAGE_ORAS=$BASE_IMG:$BASE_IMG_VERSION
if [ ! -f "$BUILD_DIR/$CONFIG/image.vhdx" ]; then
    echo "Pulling base image $BASE_IMAGE_ORAS from registry..."
    docker run \
        --rm \
        --interactive \
        --privileged=true \
        -v "$BUILD_DIR:/container/build" \
        $IMG_CUSTOMIZER_CONTAINER:$IMG_CUSTOMIZER_VERSION \
        oras pull $BASE_IMAGE_ORAS -o /container/build/$CONFIG
else
    echo "Base image already exists, skipping pull."
fi

echo "Using following Image Customizer config:"
cat $CONFIG_FILE

# Prepare Docker volume mounts
DOCKER_MOUNTS=(
    -v "$BUILD_DIR:/container/build"
    -v "$OUT_DIR:/container/out"
    -v "$(realpath "$(dirname "$CONFIG_FILE")")":/container/config
    -v /dev:/dev
    -v "$AGENTBAKER_DIR/:/AgentBaker:z"
)

# Add certificate and scripts volumes if UEFI is enabled
if [ "${ENABLE_UEFI,,}" = "true" ]; then
    DOCKER_MOUNTS+=(
        -v "$BUILD_DIR/$CONFIG/certs:/tmp:z"
        -v "$BUILD_DIR/$CONFIG/scripts:/imageconfigs/scripts:z"
    )
    echo "Added UEFI certificate volumes to Docker mounts"
fi

echo Building $CONFIG_FILE image with Image Customizer...
docker run \
    --rm \
    --interactive \
    --privileged=true \
    "${DOCKER_MOUNTS[@]}" \
    -e "ENABLE_UEFI_SECURE_BOOT=${ENABLE_UEFI}" \
    $IMG_CUSTOMIZER_CONTAINER:$IMG_CUSTOMIZER_VERSION \
    imagecustomizer \
        --log-level "debug" \
        --config-file /container/config/"$(basename "$CONFIG_FILE")" \
        --build-dir /container/build \
        --image-file /container/build/$CONFIG/image.vhdx \
        --output-image-format vhd-fixed \
        --output-image-file /container/out/$CONFIG/"$(basename "$IMAGE_PATH")"

cp $IMAGE_PATH $OUT_DIR/$CONFIG.vhd

# Place build artifacts where later pipeline stages expect them
cp "$AGENTBAKER_DIR/vhdbuilder/packer/imagecustomizer/$CONFIG/out/release-notes.txt" "$AGENTBAKER_DIR"
cp "$AGENTBAKER_DIR/vhdbuilder/packer/imagecustomizer/$CONFIG/out/bcc-tools-installation.log" "$AGENTBAKER_DIR"
cp "$AGENTBAKER_DIR/vhdbuilder/packer/imagecustomizer/$CONFIG/out/image-bom.json" "$AGENTBAKER_DIR"
cp "$AGENTBAKER_DIR/vhdbuilder/packer/imagecustomizer/$CONFIG/out/vhd-build-performance-data.json" "$AGENTBAKER_DIR"

# Generate comprehensive release notes
{
  echo "Install completed successfully on " $(date)
  echo "VSTS Build NUMBER: ${BUILD_NUMBER:-}"
  echo "VSTS Build ID: ${BUILD_ID:-}"
  echo "Commit: ${COMMIT:-}"
  echo "Hyperv generation: ${HYPERV_GENERATION:-}"
  echo "Feature flags: ${FEATURE_FLAGS:-}"
  echo "Container runtime: ${CONTAINER_RUNTIME:-}"
  echo "FIPS enabled: ${ENABLE_FIPS:-}"
  echo "UEFI Secure Boot enabled: ${ENABLE_UEFI}"
  
  if [ "${ENABLE_UEFI,,}" = "true" ] && [ -f "$CERT_DER_FILE" ]; then
    echo "UEFI Certificate SHA256: $(sha256sum "$CERT_DER_FILE" | cut -d' ' -f1)"
    echo "UEFI Certificate Subject: ${CERT_SUBJECT:-}"
    echo "UEFI Certificate Valid From: ${CERT_NOT_BEFORE:-}"
    echo "UEFI Certificate Valid To: ${CERT_NOT_AFTER:-}"
  fi
} >> $AGENTBAKER_DIR/release-notes.txt

echo "=== VHD Build Complete ==="
echo "Output VHD: $OUT_DIR/$CONFIG.vhd"
if [ "${ENABLE_UEFI,,}" = "true" ]; then
    echo "UEFI Secure Boot: Enabled with custom certificate"
else
    echo "UEFI Secure Boot: Disabled"
fi
