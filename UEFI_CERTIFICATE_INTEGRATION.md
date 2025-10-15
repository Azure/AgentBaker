# UEFI Secure Boot Certificate Integration in AgentBaker

This document describes how to inject UEFI secure boot certificates into VHDs during the AgentBaker build process.

## Overview

The AgentBaker VHD build process now supports embedding UEFI secure boot certificates directly into the VHD image during creation. This enables custom certificate deployment without requiring AKS deployment-time configuration, which is not supported by Azure's public APIs.

## Architecture

### VHD Build-Time Injection
- **When**: During VHD creation using Image Customizer
- **Where**: Certificates are embedded into the EFI system partition and system certificate stores
- **How**: Multiple installation methods ensure broad compatibility

### Installation Methods
1. **MOK (Machine Owner Key) Integration**: Uses `mokutil` for shim bootloader integration
2. **EFI System Partition**: Direct placement in `/boot/efi/EFI/certs/`
3. **System Certificate Store**: Integration with OS certificate management
4. **Boot-time Enrollment**: Systemd service for first-boot certificate enrollment
5. **Dracut Integration**: Inclusion in initial ramdisk for early boot access

## Usage

### Environment Variables

#### Required (when UEFI is enabled)
- `ENABLE_UEFI_SECURE_BOOT=true` - Enables UEFI certificate processing
- One of:
  - `UEFI_CERT_FILE=/path/to/certificate.pem` - Path to certificate file (PEM or DER format)
  - `UEFI_CERT_BASE64="<base64-encoded-der-cert>"` - Base64 encoded DER certificate

#### Optional
- `IMG_CUSTOMIZER_CONFIG=aks-node-uefi` - Use UEFI-enabled configuration

### Building VHD with UEFI Certificate

#### Method 1: Using Certificate File
```bash
# Set environment variables
export ENABLE_UEFI_SECURE_BOOT=true
export UEFI_CERT_FILE=/path/to/your/uefi-certificate.pem
export IMG_CUSTOMIZER_CONFIG=aks-node-uefi

# Run enhanced build script
./vhdbuilder/packer/imagecustomizer/scripts/build-uefi-image.sh
```

#### Method 2: Using Base64 Certificate
```bash
# Convert certificate to base64 DER format
UEFI_CERT_BASE64=$(openssl x509 -in certificate.pem -outform DER | base64 -w 0)

# Set environment variables
export ENABLE_UEFI_SECURE_BOOT=true
export UEFI_CERT_BASE64="$UEFI_CERT_BASE64"
export IMG_CUSTOMIZER_CONFIG=aks-node-uefi

# Run enhanced build script
./vhdbuilder/packer/imagecustomizer/scripts/build-uefi-image.sh
```

### Pipeline Integration

To integrate with Azure DevOps pipelines, modify the pipeline YAML:

```yaml
- bash: |
    echo '##vso[task.setvariable variable=ENABLE_UEFI_SECURE_BOOT]true'
    echo '##vso[task.setvariable variable=IMG_CUSTOMIZER_CONFIG]aks-node-uefi'
    
    # Process certificate from pipeline variable or file
    if [ -n "$(UEFI_SECURE_BOOT_CERT)" ]; then
      echo '##vso[task.setvariable variable=UEFI_CERT_BASE64]$(UEFI_SECURE_BOOT_CERT)'
    elif [ -f "$(Pipeline.Workspace)/certificate.pem" ]; then
      echo '##vso[task.setvariable variable=UEFI_CERT_FILE]$(Pipeline.Workspace)/certificate.pem'
    fi
  displayName: Setup UEFI Certificate Variables

- bash: ./vhdbuilder/packer/imagecustomizer/scripts/build-uefi-image.sh
  displayName: Build VHD with UEFI Certificate
```

## Certificate Requirements

### Supported Formats
- **PEM**: ASCII-armored X.509 certificate (automatically converted)
- **DER**: Binary X.509 certificate (used directly)

### Certificate Properties
- Must be a valid X.509 certificate
- Should be signed by a trusted authority or be a self-signed root certificate
- Must have appropriate key usage extensions for code signing
- Should have sufficient validity period for intended use

### Example Certificate Generation
```bash
# Generate private key
openssl genpkey -algorithm RSA -out uefi-key.pem -pkcs8 -aes256

# Generate self-signed certificate
openssl req -new -x509 -key uefi-key.pem -out uefi-cert.pem -days 3650 \
    -subj "/C=US/ST=WA/L=Seattle/O=MyOrg/CN=UEFI Secure Boot Certificate"

# Convert to DER format (if needed)
openssl x509 -in uefi-cert.pem -outform DER -out uefi-cert.der
```

## Files Created

The UEFI certificate integration creates the following components:

### New Files
- `install-uefi-certificate.sh` - Certificate installation script
- `uefi-postinstall.sh` - Post-installation configuration
- `aks-node-uefi.yml` - Image Customizer configuration with UEFI support
- `build-uefi-image.sh` - Enhanced build script

### VHD Locations (after build)
- `/boot/efi/EFI/certs/uefi-secure-boot.der` - Certificate in EFI partition
- `/boot/efi/EFI/certs/uefi-secure-boot.pem` - PEM format certificate
- `/usr/share/ca-certificates/uefi/uefi-secure-boot.crt` - System certificate store
- `/etc/systemd/system/uefi-cert-enroll.service` - Boot-time enrollment service

## Verification

### Build-Time Verification
The build process verifies:
- Certificate format and validity
- Successful installation to all target locations
- Package installation (mokutil, efibootmgr, etc.)
- System configuration updates

### Runtime Verification
After VHD deployment, verify certificate installation:
```bash
# Check MOK list (if mokutil available)
sudo mokutil --list-enrolled

# Check EFI certificate location
ls -la /boot/efi/EFI/certs/

# Check system certificate store
ls -la /usr/share/ca-certificates/uefi/

# Check systemd service status
systemctl status uefi-cert-enroll.service
```

## Limitations

1. **Build Environment**: Some UEFI operations may not be fully functional in VHD build containers
2. **First Boot**: Certificate enrollment may require first boot to complete
3. **Hardware Dependencies**: Secure boot behavior depends on target hardware UEFI implementation
4. **Key Enrollment**: Additional steps may be required for key enrollment in production systems

## Security Considerations

1. **Certificate Storage**: Certificates are embedded in VHD and accessible to system administrators
2. **Private Keys**: Never include private keys in VHD builds
3. **Certificate Validation**: Ensure certificates are from trusted sources
4. **Access Control**: Protect certificate files during build process

## Troubleshooting

### Common Issues

#### Certificate Not Found
```
Error: UEFI certificate file not found at /tmp/uefi-secure-boot.der
```
- Verify `UEFI_CERT_FILE` or `UEFI_CERT_BASE64` is correctly set
- Check certificate file permissions and format

#### Invalid Certificate Format
```
Error: Invalid DER certificate format
```
- Verify certificate is valid X.509 format
- Try converting: `openssl x509 -in cert.pem -outform DER -out cert.der`

#### Build Environment Issues
```
Warning: mokutil import failed, this may be expected in VHD build environment
```
- Normal in containerized build environments
- Certificate will be available for first-boot enrollment

### Debug Information

Build logs include:
- Certificate SHA256 hash
- Certificate subject and issuer
- Installation locations
- Package installation status
- System configuration changes

## Integration with Existing Workflow

This UEFI certificate injection integrates with the existing AgentBaker pipeline:

1. **Pre-Build**: Certificate processing and validation
2. **Build**: Image Customizer with enhanced configuration
3. **Post-Build**: Standard SIG publishing with certificate metadata
4. **Deployment**: Standard AKS cluster creation (no changes required)

The certificate becomes available during VM boot process without requiring AKS-level configuration changes.
