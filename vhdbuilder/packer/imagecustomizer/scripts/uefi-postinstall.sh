#!/bin/bash
set -e

# UEFI Secure Boot Post-Installation Script
# Runs after certificate installation to finalize UEFI configuration

echo "=== UEFI Secure Boot Post-Installation ==="

# Verify UEFI environment
if [ ! -d "/sys/firmware/efi" ]; then
    echo "Warning: Not running in UEFI environment"
    exit 0
fi

echo "UEFI environment detected"

# Check if certificates were installed
CERT_INSTALLED=false
if [ -f "/boot/efi/EFI/certs/uefi-secure-boot.der" ]; then
    echo "UEFI certificate found in EFI partition"
    CERT_INSTALLED=true
fi

if [ -f "/usr/share/ca-certificates/uefi/uefi-secure-boot.crt" ]; then
    echo "UEFI certificate found in system certificate store"
    CERT_INSTALLED=true
fi

if [ "$CERT_INSTALLED" = "false" ]; then
    echo "Warning: No UEFI certificates found"
    exit 1
fi

# Configure GRUB for secure boot
if [ -f "/etc/default/grub" ]; then
    echo "Configuring GRUB for secure boot compatibility"
    
    # Backup original GRUB configuration
    cp /etc/default/grub /etc/default/grub.backup
    
    # Add secure boot friendly options
    if ! grep -q "GRUB_DISABLE_OS_PROBER" /etc/default/grub; then
        echo "GRUB_DISABLE_OS_PROBER=true" >> /etc/default/grub
    fi
    
    # Ensure proper module loading for secure boot
    if ! grep -q "GRUB_PRELOAD_MODULES" /etc/default/grub; then
        echo "GRUB_PRELOAD_MODULES=\"part_gpt part_msdos\"" >> /etc/default/grub
    fi
    
    # Update GRUB configuration
    if command -v update-grub >/dev/null 2>&1; then
        update-grub
        echo "GRUB configuration updated"
    elif command -v grub2-mkconfig >/dev/null 2>&1; then
        grub2-mkconfig -o /boot/grub2/grub.cfg
        echo "GRUB2 configuration updated"
    fi
fi

# Configure shim for certificate loading
SHIM_CONFIG="/boot/efi/EFI/BOOT"
if [ -d "$SHIM_CONFIG" ]; then
    echo "Configuring shim bootloader"
    
    # Create MOK configuration if needed
    MOK_CONFIG="/boot/efi/EFI/BOOT/mok.conf"
    if [ ! -f "$MOK_CONFIG" ]; then
        cat > "$MOK_CONFIG" << EOF
# MOK (Machine Owner Key) Configuration
# This file configures additional certificates for secure boot

# Certificate file locations
cert=/EFI/certs/uefi-secure-boot.der
EOF
        echo "MOK configuration created"
    fi
fi

# Verify secure boot preparation
echo "=== Secure Boot Verification ==="

# Check EFI variables (may not be available in build environment)
if [ -d "/sys/firmware/efi/efivars" ]; then
    echo "EFI variables directory exists"
    ls -la /sys/firmware/efi/efivars/SecureBoot* 2>/dev/null || echo "SecureBoot variables not found (expected in build environment)"
else
    echo "EFI variables not accessible (expected in build environment)"
fi

# Check boot loader files
echo "Boot loader files:"
find /boot/efi -name "*.efi" 2>/dev/null | head -10 || echo "No EFI boot files found"

# Check certificate files
echo "Certificate files:"
find /boot/efi -name "*.der" -o -name "*.pem" 2>/dev/null || echo "No certificate files found in EFI partition"

# Verify system integrity
echo "=== System Integrity Check ==="

# Check if required secure boot packages are installed
REQUIRED_PACKAGES=("mokutil" "efibootmgr" "shim-signed")
for pkg in "${REQUIRED_PACKAGES[@]}"; do
    if command -v "$pkg" >/dev/null 2>&1 || dpkg -l | grep -q "^ii.*$pkg" 2>/dev/null || rpm -q "$pkg" >/dev/null 2>&1; then
        echo "✓ $pkg is installed"
    else
        echo "⚠ $pkg is not installed"
    fi
done

# Generate VHD build report
VHD_LOGS_FILEPATH="/opt/azure/vhd-install.complete"
echo "=== UEFI Secure Boot Configuration === Begin" >> ${VHD_LOGS_FILEPATH}
echo "Certificate SHA256: $(sha256sum /boot/efi/EFI/certs/uefi-secure-boot.der 2>/dev/null | cut -d' ' -f1 || echo 'Certificate not found')" >> ${VHD_LOGS_FILEPATH}
echo "UEFI Environment: $([ -d /sys/firmware/efi ] && echo 'Yes' || echo 'No')" >> ${VHD_LOGS_FILEPATH}
echo "Secure Boot Packages: $(echo "${REQUIRED_PACKAGES[@]}")" >> ${VHD_LOGS_FILEPATH}
echo "Build Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)" >> ${VHD_LOGS_FILEPATH}
echo "=== UEFI Secure Boot Configuration === End" >> ${VHD_LOGS_FILEPATH}

echo "UEFI Secure Boot post-installation completed successfully"
