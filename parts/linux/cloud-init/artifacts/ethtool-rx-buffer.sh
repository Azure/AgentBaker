#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

# Configure ethtool RX buffer size for multi-core machines
# Only applies on machines with 4 or more CPU cores

# Check if the system has 4 or more CPU cores using nproc
cpu_cores=$(nproc)
echo "Detected ${cpu_cores} CPU cores"

if [ "${cpu_cores}" -lt 4 ]; then
    echo "System has fewer than 4 CPU cores (${cpu_cores}), skipping ethtool RX buffer configuration"
    exit 0
fi

# Read configuration from environment file if it exists
ETHTOOL_RX_BUFFER_SIZE=${ETHTOOL_RX_BUFFER_SIZE:-2048}

echo "Configuring ethtool RX buffer size to ${ETHTOOL_RX_BUFFER_SIZE} on multi-core system (${cpu_cores} cores)"

# Find the primary network interface (exclude loopback)
primary_interface=$(ip route | grep default | awk '{print $5}' | head -n1)
if [ -z "${primary_interface}" ]; then
    echo "Could not determine primary network interface, skipping ethtool configuration"
    exit 0
fi

echo "Primary network interface: ${primary_interface}"

# Check if interface supports RX buffer size configuration
if ! ethtool -g "${primary_interface}" >/dev/null 2>&1; then
    echo "Interface ${primary_interface} does not support ring parameter configuration, skipping"
    exit 0
fi

# Get current RX buffer size
current_rx=$(ethtool -g "${primary_interface}" | grep -A1 "Ring parameters for" | grep "RX:" | awk '{print $2}' | head -n1)
echo "Current RX buffer size: ${current_rx}"

# Set RX buffer size if different from target
if [ "${current_rx}" != "${ETHTOOL_RX_BUFFER_SIZE}" ]; then
    echo "Setting RX buffer size to ${ETHTOOL_RX_BUFFER_SIZE}"
    if ethtool -G "${primary_interface}" rx "${ETHTOOL_RX_BUFFER_SIZE}"; then
        echo "Successfully configured ethtool RX buffer size to ${ETHTOOL_RX_BUFFER_SIZE}"
    else
        echo "Failed to set ethtool RX buffer size, but continuing..."
        exit 0
    fi
else
    echo "RX buffer size already set to ${ETHTOOL_RX_BUFFER_SIZE}, no changes needed"
fi