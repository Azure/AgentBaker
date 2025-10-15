#!/bin/bash

# Example usage of the enhanced VHD publishing script with UEFI certificate support

set -e

# Example: Publish VHD with UEFI certificate injection
#
# This script demonstrates how to use the enhanced publish-imagecustomizer-image.sh
# script with UEFI secure boot certificate support.

# Required environment variables for VHD publishing
export AZURE_MSI_RESOURCE_STRING="your-msi-resource-string"
export RESOURCE_GROUP_NAME="your-resource-group"
export SIG_IMAGE_NAME="your-sig-image-name"
export IMAGE_NAME="your-managed-image-name"
export SUBSCRIPTION_ID="your-subscription-id"
export CAPTURED_SIG_VERSION="1.0.0"
export PACKER_BUILD_LOCATION="eastus"
export GENERATE_PUBLISHING_INFO="true"
export IMG_CUSTOMIZER_CONFIG="aks-node-uefi"
export CLASSIC_BLOB="https://yourstorageaccount.blob.core.windows.net/vhds"

# Optional: Enable UEFI secure boot certificate injection
export ENABLE_UEFI_SECURE_BOOT="true"
export DEBUG="false"

echo "=== VHD Publishing with UEFI Certificate Example ==="
echo "Configuration:"
echo "  Resource Group: $RESOURCE_GROUP_NAME"
echo "  SIG Image: $SIG_IMAGE_NAME"
echo "  Version: $CAPTURED_SIG_VERSION"
echo "  Location: $PACKER_BUILD_LOCATION"
echo "  UEFI Enabled: $ENABLE_UEFI_SECURE_BOOT"
echo "  Blob Storage: $CLASSIC_BLOB"
echo ""

# The script will:
# 1. Look for VHD at: ${CLASSIC_BLOB}/${IMG_CUSTOMIZER_CONFIG}.vhd
# 2. Look for certificate at: ${CLASSIC_BLOB}/ca-cert.pem
# 3. Download both if they don't exist locally
# 4. Inject certificate into VHD if ENABLE_UEFI_SECURE_BOOT=true
# 5. Publish to Azure SIG

echo "Expected artifacts in blob storage:"
echo "  VHD: ${CLASSIC_BLOB}/${IMG_CUSTOMIZER_CONFIG}.vhd"
echo "  Certificate: ${CLASSIC_BLOB}/ca-cert.pem"
echo ""

# Check if artifacts exist in blob storage
echo "Checking for required artifacts..."

# Check VHD
if azcopy list "${CLASSIC_BLOB}/${IMG_CUSTOMIZER_CONFIG}.vhd" --output-type text >/dev/null 2>&1; then
    echo "✓ VHD found in blob storage"
else
    echo "⚠ VHD not found in blob storage: ${CLASSIC_BLOB}/${IMG_CUSTOMIZER_CONFIG}.vhd"
fi

# Check certificate (optional)
if azcopy list "${CLASSIC_BLOB}/ca-cert.pem" --output-type text >/dev/null 2>&1; then
    echo "✓ UEFI certificate found in blob storage"
else
    echo "⚠ UEFI certificate not found in blob storage: ${CLASSIC_BLOB}/ca-cert.pem"
    echo "  (This is optional - UEFI injection will be skipped if not present)"
fi

echo ""
echo "Running enhanced VHD publishing script..."

# Execute the enhanced publishing script
exec ./vhdbuilder/packer/imagecustomizer/scripts/publish-imagecustomizer-image.sh

# After execution, the following pipeline variables will be set:
# - UEFI_SECURE_BOOT_CERT: Base64 encoded DER certificate (if certificate was processed)
# - UEFI_CERT_INJECTED: "true" if certificate was successfully injected into VHD
# - MANAGED_SIG_ID: Resource ID of the created SIG image version
