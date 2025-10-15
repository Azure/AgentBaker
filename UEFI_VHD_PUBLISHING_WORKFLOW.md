# UEFI VHD Publishing Workflow

This document describes the enhanced VHD publishing process that supports UEFI secure boot certificate injection using artifacts downloaded from blob storage.

## Overview

The enhanced `publish-imagecustomizer-image.sh` script can now:

1. **Download VHD from blob storage** if not present locally
2. **Download UEFI certificate from blob storage** (ca-cert.pem)
3. **Inject UEFI certificate into VHD** during the publishing process
4. **Upload modified VHD** to blob storage
5. **Create Azure SIG image** with embedded certificate support

## Required Artifacts in Blob Storage

### VHD File
- **Location**: `${CLASSIC_BLOB}/${IMG_CUSTOMIZER_CONFIG}.vhd`
- **Format**: Fixed VHD format compatible with Azure
- **Example**: `https://mystorageaccount.blob.core.windows.net/vhds/aks-node-uefi.vhd`

### UEFI Certificate (Optional)
- **Location**: `${CLASSIC_BLOB}/ca-cert.pem`
- **Format**: PEM-encoded X.509 certificate
- **Example**: `https://mystorageaccount.blob.core.windows.net/vhds/ca-cert.pem`

## Environment Variables

### Required Variables
```bash
AZURE_MSI_RESOURCE_STRING="your-msi-resource-string"
RESOURCE_GROUP_NAME="your-resource-group"
SIG_IMAGE_NAME="your-sig-image-name"
IMAGE_NAME="your-managed-image-name"
SUBSCRIPTION_ID="your-subscription-id"
CAPTURED_SIG_VERSION="1.0.0"
PACKER_BUILD_LOCATION="eastus"
GENERATE_PUBLISHING_INFO="true"
IMG_CUSTOMIZER_CONFIG="aks-node-uefi"
CLASSIC_BLOB="https://yourstorageaccount.blob.core.windows.net/vhds"
```

### Optional UEFI Variables
```bash
ENABLE_UEFI_SECURE_BOOT="true"  # Enable certificate injection
DEBUG="true"                    # Enable debug output
```

## Workflow Steps

### 1. Artifact Discovery and Download

The script automatically:
- Checks for VHD at `${CLASSIC_BLOB}/${IMG_CUSTOMIZER_CONFIG}.vhd`
- Downloads VHD if not present locally
- Checks for certificate at `${CLASSIC_BLOB}/ca-cert.pem`
- Downloads certificate if present

### 2. Certificate Processing

If certificate is found:
- Validates PEM format
- Converts to base64 DER format
- Logs certificate details (subject, issuer, validity)
- Stores in pipeline variable `UEFI_SECURE_BOOT_CERT`

### 3. VHD Certificate Injection

If `ENABLE_UEFI_SECURE_BOOT=true`:
- Uses `inject-uefi-certificate.sh` script
- Injects certificate into VHD using libguestfs tools
- Creates certificate files in multiple locations:
  - `/boot/efi/EFI/certs/uefi-secure-boot.der`
  - `/boot/efi/EFI/certs/uefi-secure-boot.pem`
  - `/usr/share/ca-certificates/uefi/uefi-secure-boot.crt`
- Creates systemd service for boot-time enrollment
- Updates system certificate store
- Sets pipeline variable `UEFI_CERT_INJECTED=true`

### 4. Azure Publishing

- Uploads VHD to blob storage as `${CAPTURED_SIG_VERSION}.vhd`
- Creates managed image from VHD
- Creates SIG image version
- Replicates to target regions

## Certificate Injection Methods

The injection script supports multiple methods (in order of preference):

### 1. virt-customize (Preferred)
```bash
virt-customize -a "$VHD_FILE" \
    --upload "$CERT_FILE:/tmp/cert.der" \
    --run "/tmp/install-script.sh"
```

### 2. guestfish (Alternative)
```bash
guestfish -a "$VHD_FILE" -f injection_script.fish
```

### 3. guestmount (Manual)
```bash
guestmount -a "$VHD_FILE" -m /dev/sda1 /mnt/vhd
# Manual file operations
guestunmount /mnt/vhd
```

## Usage Examples

### Basic Usage (No UEFI)
```bash
export ENABLE_UEFI_SECURE_BOOT="false"
./publish-imagecustomizer-image.sh
```

### With UEFI Certificate Injection
```bash
export ENABLE_UEFI_SECURE_BOOT="true"
export DEBUG="true"
./publish-imagecustomizer-image.sh
```

### Pipeline Integration
```yaml
- bash: |
    # Upload artifacts to blob storage
    azcopy copy "$(Build.ArtifactStagingDirectory)/aks-node.vhd" "$(CLASSIC_BLOB)/aks-node-uefi.vhd"
    azcopy copy "$(Build.ArtifactStagingDirectory)/ca-cert.pem" "$(CLASSIC_BLOB)/ca-cert.pem"
  displayName: Upload VHD and Certificate to Blob Storage

- bash: |
    export ENABLE_UEFI_SECURE_BOOT="true"
    export IMG_CUSTOMIZER_CONFIG="aks-node-uefi"
    ./vhdbuilder/packer/imagecustomizer/scripts/publish-imagecustomizer-image.sh
  displayName: Publish VHD with UEFI Certificate
```

## Output Variables

After execution, the following pipeline variables are set:

- `UEFI_SECURE_BOOT_CERT`: Base64 DER certificate (if processed)
- `UEFI_CERT_INJECTED`: "true" if injection succeeded, "false" otherwise
- `MANAGED_SIG_ID`: Azure resource ID of created SIG image

## Verification

### Build-Time Verification
```bash
# Check pipeline variables
echo "UEFI Certificate Length: ${#UEFI_SECURE_BOOT_CERT}"
echo "Certificate Injected: $UEFI_CERT_INJECTED"
echo "SIG Image ID: $MANAGED_SIG_ID"
```

### Runtime Verification
After VM deployment from SIG image:
```bash
# Check certificate files
ls -la /boot/efi/EFI/certs/
ls -la /usr/share/ca-certificates/uefi/

# Check injection marker
cat /var/lib/uefi-cert-injected

# Check systemd service
systemctl status uefi-cert-enroll.service

# Check MOK enrollment (if available)
sudo mokutil --list-enrolled
```

## Troubleshooting

### Common Issues

1. **VHD Not Found**
   ```
   Error: VHD not found locally at /out/aks-node-uefi.vhd, attempting to download from blob storage
   ```
   - Ensure VHD exists in blob storage
   - Check blob storage permissions
   - Verify `IMG_CUSTOMIZER_CONFIG` matches VHD filename

2. **Certificate Download Failed**
   ```
   No UEFI certificate found at https://storage.blob.core.windows.net/vhds/ca-cert.pem
   ```
   - Certificate is optional - this is informational
   - Upload certificate to blob storage if UEFI injection is needed

3. **Certificate Injection Failed**
   ```
   Warning: UEFI certificate injection failed, using pipeline variable method
   ```
   - libguestfs tools may not be available
   - VHD may be corrupted or in wrong format
   - Check debug output with `DEBUG=true`

4. **Permission Denied**
   ```
   Error: No libguestfs tools available for VHD modification
   ```
   - Install libguestfs-tools: `sudo apt-get install libguestfs-tools`
   - Ensure adequate permissions for VHD modification

### Debug Information

Enable debug mode:
```bash
export DEBUG="true"
```

Check log outputs:
- Certificate processing details
- VHD mount/unmount operations
- File creation and permissions
- Pipeline variable settings

## Security Considerations

1. **Certificate Storage**: Certificates in blob storage should have appropriate access controls
2. **Pipeline Variables**: UEFI certificate is stored as base64 in pipeline variables
3. **VHD Security**: Modified VHDs contain embedded certificates accessible to system administrators
4. **Network Security**: Ensure blob storage connections use HTTPS and proper authentication

## Dependencies

### Required Tools
- `azcopy`: Azure storage operations
- `az cli`: Azure resource management
- `openssl`: Certificate processing

### Optional Tools (for UEFI injection)
- `libguestfs-tools`: VHD manipulation
- `virt-customize`: Preferred injection method
- `guestfish`: Alternative injection method
- `guestmount`: Manual injection method

Install on Ubuntu:
```bash
sudo apt-get update
sudo apt-get install libguestfs-tools
```
