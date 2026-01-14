#!/usr/bin/env bash

# Copyright (c) Microsoft Corporation. All rights reserved.
# Licensed under the MIT license.

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}================================${NC}"
echo -e "${GREEN}Git Commit Signing Setup Script${NC}"
echo -e "${GREEN}================================${NC}"
echo ""

# Function to print error messages
error() {
    echo -e "${RED}ERROR: $1${NC}" >&2
}

# Function to print success messages
success() {
    echo -e "${GREEN}✓ $1${NC}"
}

# Function to print info messages
info() {
    echo -e "${YELLOW}ℹ $1${NC}"
}

# Check if gpg is installed
if ! command -v gpg &> /dev/null; then
    error "GPG is not installed. Please install GPG first."
    echo ""
    echo "Installation instructions:"
    echo "  Ubuntu/Debian: sudo apt-get install gnupg"
    echo "  macOS: brew install gnupg"
    echo "  Windows: Download from https://www.gnupg.org/download/"
    exit 1
fi

success "GPG is installed"

# Check for existing GPG keys
echo ""
info "Checking for existing GPG keys..."
EXISTING_KEYS=$(gpg --list-secret-keys --keyid-format=long 2>/dev/null || true)

if [ -z "$EXISTING_KEYS" ]; then
    echo ""
    info "No existing GPG keys found. Creating a new one..."
    echo ""
    echo "Please follow the prompts to create a new GPG key."
    echo "Use the email address associated with your GitHub account."
    echo ""
    
    if ! gpg --default-new-key-algo rsa4096 --gen-key; then
        error "Failed to generate GPG key"
        exit 1
    fi
    
    success "GPG key created successfully"
    EXISTING_KEYS=$(gpg --list-secret-keys --keyid-format=long)
else
    success "Found existing GPG key(s)"
    echo ""
    echo "$EXISTING_KEYS"
fi

# Extract the GPG key ID
echo ""
info "Extracting GPG key ID..."
GPG_KEY_ID=$(gpg --list-secret-keys --keyid-format=long | grep '^sec' | head -n 1 | sed -E 's/.*\/([A-Fa-f0-9]+).*/\1/')

if [ -z "$GPG_KEY_ID" ]; then
    error "Could not extract GPG key ID"
    exit 1
fi

success "GPG Key ID: $GPG_KEY_ID"

# Configure Git to use GPG signing
echo ""
info "Configuring Git to use GPG signing..."

git config --global gpg.program gpg
git config --global commit.gpgsign true
git config --global user.signingkey "$GPG_KEY_ID"

success "Git configured to sign commits with GPG"

# Configure GPG agent settings
echo ""
info "Configuring GPG agent..."

GPG_CONF_DIR="$HOME/.gnupg"
mkdir -p "$GPG_CONF_DIR"
chmod 700 "$GPG_CONF_DIR"

# Add GPG configuration if not already present
if ! grep -q "use-agent" "$GPG_CONF_DIR/gpg.conf" 2>/dev/null; then
    echo "use-agent" >> "$GPG_CONF_DIR/gpg.conf"
fi

if ! grep -q "pinentry-mode loopback" "$GPG_CONF_DIR/gpg.conf" 2>/dev/null; then
    echo "pinentry-mode loopback" >> "$GPG_CONF_DIR/gpg.conf"
fi

if ! grep -q "allow-loopback-pinentry" "$GPG_CONF_DIR/gpg-agent.conf" 2>/dev/null; then
    echo "allow-loopback-pinentry" >> "$GPG_CONF_DIR/gpg-agent.conf"
fi

# Restart GPG agent
gpgconf --kill gpg-agent 2>/dev/null || true
gpgconf --launch gpg-agent 2>/dev/null || true

success "GPG agent configured"

# Export the public key
echo ""
info "Generating public GPG key for GitHub..."
echo ""
echo "======================================================================"
echo "Copy the following GPG public key and add it to your GitHub account:"
echo "======================================================================"
echo ""

gpg --armor --export "$GPG_KEY_ID"

echo ""
echo "======================================================================"
echo ""
echo -e "${YELLOW}Next steps:${NC}"
echo "1. Copy the GPG public key shown above (including the BEGIN and END lines)"
echo "2. Go to GitHub Settings -> SSH and GPG keys -> New GPG key"
echo "   URL: https://github.com/settings/gpg/new"
echo "3. Paste the public key and click 'Add GPG key'"
echo ""
echo -e "${GREEN}Your Git is now configured to sign all commits automatically!${NC}"
echo ""
echo "To sign commits in this repository only (instead of globally), run:"
echo "  cd \$(git rev-parse --show-toplevel)"
echo "  git config --local commit.gpgsign true"
echo ""
