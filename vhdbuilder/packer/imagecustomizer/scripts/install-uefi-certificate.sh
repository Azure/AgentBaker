#!/bin/bash
set -e

# Script to install UEFI secure boot certificate into VHD during build process
# This runs during VHD creation, not at VM provisioning time

CERT_FILE="/tmp/uefi-secure-boot.der"
MOKUTIL_AVAILABLE=$(command -v mokutil >/dev/null 2>&1 && echo "true" || echo "false")
EFIBOOTMGR_AVAILABLE=$(command -v efibootmgr >/dev/null 2>&1 && echo "true" || echo "false")

echo "Installing UEFI secure boot certificate during VHD build process"
echo "Certificate file: $CERT_FILE"
echo "mokutil available: $MOKUTIL_AVAILABLE"
echo "efibootmgr available: $EFIBOOTMGR_AVAILABLE"

if [ ! -f "$CERT_FILE" ]; then
    echo "Error: UEFI certificate file not found at $CERT_FILE"
    exit 1
fi

# Verify certificate format
if ! openssl x509 -inform DER -in "$CERT_FILE" -noout >/dev/null 2>&1; then
    echo "Error: Invalid DER certificate format"
    exit 1
fi

echo "Certificate verified as valid DER format"

# Method 1: Install using mokutil (Machine Owner Key Utility)
# This adds the certificate to the MOK (Machine Owner Key) list for shim
if [ "$MOKUTIL_AVAILABLE" = "true" ]; then
    echo "Installing certificate using mokutil..."
    
    # Import the certificate into the MOK list
    # Note: This would normally require a password and reboot, but in VHD build we can pre-stage
    mokutil --import "$CERT_FILE" --password="" 2>/dev/null || {
        echo "Warning: mokutil import failed, this may be expected in VHD build environment"
    }
    
    # List enrolled certificates
    mokutil --list-enrolled 2>/dev/null || echo "Could not list enrolled certificates"
fi

# Method 2: Install to EFI system partition
# Copy certificate to EFI system partition for potential boot-time loading
EFI_CERTS_DIR="/boot/efi/EFI/certs"
if [ -d "/boot/efi/EFI" ]; then
    echo "Installing certificate to EFI system partition..."
    
    mkdir -p "$EFI_CERTS_DIR"
    cp "$CERT_FILE" "$EFI_CERTS_DIR/uefi-secure-boot.der"
    
    # Also convert to PEM format for compatibility
    openssl x509 -inform DER -in "$CERT_FILE" -outform PEM -out "$EFI_CERTS_DIR/uefi-secure-boot.pem"
    
    echo "Certificate installed to $EFI_CERTS_DIR"
    ls -la "$EFI_CERTS_DIR"
fi

# Method 3: Install to system certificate store (for kernel module verification)
SYSTEM_CERT_DIR="/usr/share/ca-certificates/uefi"
echo "Installing certificate to system certificate store..."

mkdir -p "$SYSTEM_CERT_DIR"
openssl x509 -inform DER -in "$CERT_FILE" -outform PEM -out "$SYSTEM_CERT_DIR/uefi-secure-boot.crt"

# Update system certificate store
if command -v update-ca-certificates >/dev/null 2>&1; then
    echo "uefi/uefi-secure-boot.crt" >> /etc/ca-certificates.conf
    update-ca-certificates
    echo "System certificate store updated"
elif command -v update-ca-trust >/dev/null 2>&1; then
    # For RHEL/CentOS/Mariner systems
    cp "$SYSTEM_CERT_DIR/uefi-secure-boot.crt" /etc/pki/ca-trust/source/anchors/
    update-ca-trust
    echo "System certificate trust updated"
fi

# Method 4: Create EFI variables (if supported in build environment)
if [ -d "/sys/firmware/efi/efivars" ] && [ "$EFIBOOTMGR_AVAILABLE" = "true" ]; then
    echo "Attempting to configure EFI variables..."
    
    # Note: This may not work in all build environments
    # but can be attempted for completeness
    efibootmgr -v 2>/dev/null || echo "EFI variables not accessible in build environment"
fi

# Method 5: Create dracut configuration for certificate loading
# This ensures the certificate is available during boot process
DRACUT_CONF_DIR="/etc/dracut.conf.d"
if [ -d "$DRACUT_CONF_DIR" ]; then
    echo "Configuring dracut for UEFI certificate loading..."
    
    cat > "$DRACUT_CONF_DIR/99-uefi-certs.conf" << EOF
# Include UEFI certificates in initramfs
install_items+=" /boot/efi/EFI/certs/uefi-secure-boot.der "
install_items+=" /boot/efi/EFI/certs/uefi-secure-boot.pem "
EOF
    
    echo "Dracut configuration created for certificate inclusion"
fi

# Method 6: Create systemd service for boot-time certificate enrollment
# This service will run on first boot to enroll the certificate
SYSTEMD_SERVICE_DIR="/etc/systemd/system"
if [ -d "$SYSTEMD_SERVICE_DIR" ]; then
    echo "Creating systemd service for boot-time certificate enrollment..."
    
    cat > "$SYSTEMD_SERVICE_DIR/uefi-cert-enroll.service" << EOF
[Unit]
Description=Enroll UEFI Secure Boot Certificate
After=multi-user.target
ConditionPathExists=/boot/efi/EFI/certs/uefi-secure-boot.der
ConditionPathExists=!/var/lib/uefi-cert-enrolled

[Service]
Type=oneshot
ExecStart=/bin/bash -c 'if command -v mokutil >/dev/null 2>&1; then mokutil --import /boot/efi/EFI/certs/uefi-secure-boot.der --password="" 2>/dev/null || true; fi; touch /var/lib/uefi-cert-enrolled'
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
EOF
    
    # Enable the service (will run on first boot)
    systemctl enable uefi-cert-enroll.service 2>/dev/null || echo "Could not enable systemd service in build environment"
    
    echo "Systemd service created for boot-time certificate enrollment"
fi

# Log certificate details for debugging
echo "=== UEFI Certificate Installation Summary ==="
echo "Certificate SHA256: $(sha256sum "$CERT_FILE" | cut -d' ' -f1)"
echo "Certificate subject: $(openssl x509 -inform DER -in "$CERT_FILE" -noout -subject 2>/dev/null || echo 'Could not read subject')"
echo "Certificate issuer: $(openssl x509 -inform DER -in "$CERT_FILE" -noout -issuer 2>/dev/null || echo 'Could not read issuer')"
echo "Certificate not before: $(openssl x509 -inform DER -in "$CERT_FILE" -noout -startdate 2>/dev/null || echo 'Could not read start date')"
echo "Certificate not after: $(openssl x509 -inform DER -in "$CERT_FILE" -noout -enddate 2>/dev/null || echo 'Could not read end date')"

echo "UEFI certificate installation completed successfully"
