#!/bin/bash -e

required_env_vars=(
  "CAPTURED_SIG_VERSION"
)

for v in "${required_env_vars[@]}"
do
    if [ -z "${!v}" ]; then
        echo "$v was not set!"
        exit 1
    fi
done

if [ ! -f "${CAPTURED_SIG_VERSION}.vhd" ]; then
  echo "Error: file ${VHD_FILE} not found"
  exit 1
fi

echo "Calculating checksum..."
sha256sum "${CAPTURED_SIG_VERSION}.vhd" > "${CAPTURED_SIG_VERSION}.sha256"
echo "VHD file checksum created"

echo "Importing signing keys..."
gpg --batch --yes --pinentry-mode loopback --import ${PUBLIC_KEY_PATH}
gpg --batch --yes --pinentry-mode loopback --import ${PRIVATE_KEY_PATH}

echo "Parsing Private Key ID..."
KEY_ID=$(gpg --list-secret-key --keyid-format LONG --with-colons 'aks-node' | grep '^sec' | cut -d: -f5)

echo "Signing VHD checksum..."
gpg --batch --yes --pinentry-mode loopback --local-user "$KEY_ID" --armor --detach-sign ${CAPTURED_SIG_VERSION}.sha256

gpg --verify --yes ${CAPTURED_SIG_VERSION}.sha256.asc ${CAPTURED_SIG_VERSION}.sha256

HASH_FILE=""
SIG_FILE=""




