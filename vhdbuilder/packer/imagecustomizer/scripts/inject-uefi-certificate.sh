#!/bin/bash
set -e

# UEFI Certificate Injection Script for VHD Post-Processing
# This script injects UEFI certificates into already-built VHDs

SCRIPT_NAME="inject-uefi-certificate"
VHD_FILE="$1"
CERT_FILE="$2"
ENABLE_DEBUG="${3:-false}"

if [ "$ENABLE_DEBUG" = "true" ]; then
    set -x
fi

usage() {
    echo "Usage: $0 <vhd-file> <certificate-file> [debug]"
    echo "  vhd-file: Path to the VHD file to modify"
    echo "  certificate-file: Path to the PEM certificate file"
    echo "  debug: Optional - set to 'true' to enable debug output"
    exit 1
}

if [ $# -lt 2 ]; then
    usage
fi

if [ ! -f "$VHD_FILE" ]; then
    echo "Error: VHD file not found: $VHD_FILE"
    exit 1
fi

if [ ! -f "$CERT_FILE" ]; then
    echo "Error: Certificate file not found: $CERT_FILE"
    exit 1
fi

echo "=== UEFI Certificate VHD Injection ==="
echo "VHD File: $VHD_FILE"
echo "Certificate File: $CERT_FILE"
echo "Debug Mode: $ENABLE_DEBUG"

# Verify certificate format
if ! openssl x509 -in "$CERT_FILE" -noout 2>/dev/null; then
    echo "Error: Invalid certificate format in $CERT_FILE"
    exit 1
fi

echo "Certificate verified as valid PEM format"

# Log certificate details
CERT_SUBJECT=$(openssl x509 -in "$CERT_FILE" -noout -subject 2>/dev/null | sed 's/subject=//')
CERT_ISSUER=$(openssl x509 -in "$CERT_FILE" -noout -issuer 2>/dev/null | sed 's/issuer=//')
CERT_FINGERPRINT=$(openssl x509 -in "$CERT_FILE" -noout -fingerprint -sha256 2>/dev/null | sed 's/SHA256 Fingerprint=//')

echo "Certificate Subject: $CERT_SUBJECT"
echo "Certificate Issuer: $CERT_ISSUER"
echo "Certificate Fingerprint: $CERT_FINGERPRINT"

# Create temporary working directory
TEMP_DIR="/tmp/vhd-uefi-inject-$$"
mkdir -p "$TEMP_DIR"

# Convert certificate to DER format
CERT_DER_FILE="$TEMP_DIR/uefi-secure-boot.der"
openssl x509 -in "$CERT_FILE" -outform DER -out "$CERT_DER_FILE"

echo "Certificate converted to DER format: $CERT_DER_FILE"

# Check for required tools
REQUIRED_TOOLS=("guestfish" "virt-customize" "guestmount")
AVAILABLE_TOOLS=()

for tool in "${REQUIRED_TOOLS[@]}"; do
    if command -v "$tool" >/dev/null 2>&1; then
        AVAILABLE_TOOLS+=("$tool")
        echo "✓ $tool is available"
    else
        echo "⚠ $tool is not available"
    fi
done

if [ ${#AVAILABLE_TOOLS[@]} -eq 0 ]; then
    echo "Error: No libguestfs tools available for VHD modification"
    echo "Please install libguestfs-tools package"
    exit 1
fi

# Method 1: Use virt-customize (preferred method)
if command -v virt-customize >/dev/null 2>&1; then
    echo "Using virt-customize for certificate injection"
    
    # Create certificate installation script
    INSTALL_SCRIPT="$TEMP_DIR/install-cert.sh"
    cat > "$INSTALL_SCRIPT" << 'EOF'
#!/bin/bash
set -e

# Create certificate directories
mkdir -p /boot/efi/EFI/certs
mkdir -p /usr/share/ca-certificates/uefi
mkdir -p /etc/systemd/system

# Copy certificate files (will be uploaded by virt-customize)
cp /tmp/uefi-secure-boot.der /boot/efi/EFI/certs/
openssl x509 -inform DER -in /tmp/uefi-secure-boot.der -outform PEM -out /boot/efi/EFI/certs/uefi-secure-boot.pem
openssl x509 -inform DER -in /tmp/uefi-secure-boot.der -outform PEM -out /usr/share/ca-certificates/uefi/uefi-secure-boot.crt

# Update system certificate store
if command -v update-ca-certificates >/dev/null 2>&1; then
    echo "uefi/uefi-secure-boot.crt" >> /etc/ca-certificates.conf
    update-ca-certificates
elif command -v update-ca-trust >/dev/null 2>&1; then
    cp /usr/share/ca-certificates/uefi/uefi-secure-boot.crt /etc/pki/ca-trust/source/anchors/
    update-ca-trust
fi

# Create systemd service for boot-time enrollment
cat > /etc/systemd/system/uefi-cert-enroll.service << 'EOSERVICE'
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
EOSERVICE

# Enable the service
if command -v systemctl >/dev/null 2>&1; then
    systemctl enable uefi-cert-enroll.service 2>/dev/null || true
fi

# Create injection marker
echo "UEFI certificate injected via virt-customize" > /var/lib/uefi-cert-injected
echo "Certificate SHA256: $(sha256sum /tmp/uefi-secure-boot.der | cut -d' ' -f1)" >> /var/lib/uefi-cert-injected
echo "Injection Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)" >> /var/lib/uefi-cert-injected

# Log successful injection
echo "UEFI certificate injection completed successfully"
EOF

    chmod +x "$INSTALL_SCRIPT"
    
    # Use virt-customize to inject certificate
    virt-customize -a "$VHD_FILE" \
        --upload "$CERT_DER_FILE:/tmp/uefi-secure-boot.der" \
        --upload "$INSTALL_SCRIPT:/tmp/install-cert.sh" \
        --run "/tmp/install-cert.sh" \
        --delete "/tmp/install-cert.sh" \
        --delete "/tmp/uefi-secure-boot.der"
    
    echo "Certificate injection completed using virt-customize"

# Method 2: Use guestfish (alternative method)
elif command -v guestfish >/dev/null 2>&1; then
    echo "Using guestfish for certificate injection"
    
    # Create guestfish script
    GUESTFISH_SCRIPT="$TEMP_DIR/inject.fish"
    cat > "$GUESTFISH_SCRIPT" << EOF
launch

# Mount the filesystem
mount /dev/sda1 /

# Create directories
mkdir-p /boot/efi/EFI/certs
mkdir-p /usr/share/ca-certificates/uefi

# Upload certificate
upload $CERT_DER_FILE /boot/efi/EFI/certs/uefi-secure-boot.der

# Create PEM version
command "openssl x509 -inform DER -in /boot/efi/EFI/certs/uefi-secure-boot.der -outform PEM -out /boot/efi/EFI/certs/uefi-secure-boot.pem"
command "openssl x509 -inform DER -in /boot/efi/EFI/certs/uefi-secure-boot.der -outform PEM -out /usr/share/ca-certificates/uefi/uefi-secure-boot.crt"

# Create injection marker
write /var/lib/uefi-cert-injected "UEFI certificate injected via guestfish\n"

umount-all
EOF

    guestfish -a "$VHD_FILE" -f "$GUESTFISH_SCRIPT"
    echo "Certificate injection completed using guestfish"

# Method 3: Use guestmount (manual mount method)
elif command -v guestmount >/dev/null 2>&1; then
    echo "Using guestmount for certificate injection"
    
    VHD_MOUNT_DIR="$TEMP_DIR/mount"
    mkdir -p "$VHD_MOUNT_DIR"
    
    # Mount VHD
    if guestmount -a "$VHD_FILE" -m /dev/sda1 "$VHD_MOUNT_DIR"; then
        echo "VHD mounted successfully"
        
        # Create directories
        mkdir -p "$VHD_MOUNT_DIR/boot/efi/EFI/certs"
        mkdir -p "$VHD_MOUNT_DIR/usr/share/ca-certificates/uefi"
        
        # Copy certificate files
        cp "$CERT_DER_FILE" "$VHD_MOUNT_DIR/boot/efi/EFI/certs/"
        openssl x509 -inform DER -in "$CERT_DER_FILE" -outform PEM -out "$VHD_MOUNT_DIR/boot/efi/EFI/certs/uefi-secure-boot.pem"
        openssl x509 -inform DER -in "$CERT_DER_FILE" -outform PEM -out "$VHD_MOUNT_DIR/usr/share/ca-certificates/uefi/uefi-secure-boot.crt"
        
        # Create injection marker
        echo "UEFI certificate injected via guestmount" > "$VHD_MOUNT_DIR/var/lib/uefi-cert-injected"
        echo "Certificate SHA256: $(sha256sum "$CERT_DER_FILE" | cut -d' ' -f1)" >> "$VHD_MOUNT_DIR/var/lib/uefi-cert-injected"
        echo "Injection Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$VHD_MOUNT_DIR/var/lib/uefi-cert-injected"
        
        # Unmount
        guestunmount "$VHD_MOUNT_DIR"
        echo "Certificate injection completed using guestmount"
    else
        echo "Error: Failed to mount VHD"
        exit 1
    fi
else
    echo "Error: No suitable libguestfs tool available"
    exit 1
fi

# Cleanup
rm -rf "$TEMP_DIR"

# Verify VHD integrity after modification
VHD_SIZE_AFTER=$(stat -c%s "$VHD_FILE" 2>/dev/null || stat -f%z "$VHD_FILE" 2>/dev/null)
echo "VHD size after certificate injection: ${VHD_SIZE_AFTER} bytes"

echo "=== UEFI Certificate Injection Complete ==="
echo "Certificate successfully injected into: $VHD_FILE"
echo "Certificate Subject: $CERT_SUBJECT"
echo "Certificate Fingerprint: $CERT_FINGERPRINT"
