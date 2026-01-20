#!/bin/bash

# This script configures network interface settings for Azure NICs.

NICS_TO_CONFIGURE_FILE="/etc/azure-network/nics-to-configure"

# Determine default RX buffer size based on number of CPUs
NUM_CPUS=$(nproc)
if [ "$NUM_CPUS" -ge 4 ]; then
    DEFAULT_RX_BUFFER_SIZE=2048
else
    DEFAULT_RX_BUFFER_SIZE=1024
fi

if [ ! -f "$NICS_TO_CONFIGURE_FILE" ]; then
    echo "No NICs to configure."
    exit 0
fi

echo "Configuring NICs listed in $NICS_TO_CONFIGURE_FILE"
echo "Detected $NUM_CPUS CPUs, default RX buffer size: $DEFAULT_RX_BUFFER_SIZE"

# Parse ethtool configuration from file
RX_SIZE=$DEFAULT_RX_BUFFER_SIZE
ETHTOOL_CONFIG_FILE="/etc/azure-network/ethtool.conf"

if [ -f "$ETHTOOL_CONFIG_FILE" ]; then
    CONFIG_RX_SIZE=$(grep "^rx=" "$ETHTOOL_CONFIG_FILE" | cut -d'=' -f2)
    if [ -n "$CONFIG_RX_SIZE" ] && [ "$CONFIG_RX_SIZE" -gt 0 ] 2>/dev/null; then
        RX_SIZE=$CONFIG_RX_SIZE
        echo "Using configured rx buffer size from $ETHTOOL_CONFIG_FILE: $RX_SIZE"
    fi
else
    echo "No ethtool config file found at $ETHTOOL_CONFIG_FILE, using default rx=$RX_SIZE"
fi

while read -r nic; do

    if [ ! -d "/sys/class/net/$nic" ]; then
        echo "NIC $nic does not exist. Skipping."
        continue
    fi

    echo "Configuring NIC: $nic with rx=$RX_SIZE"
    ethtool -G "$nic" rx "$RX_SIZE" || echo "Failed to set ring parameters for $nic"
done < "$NICS_TO_CONFIGURE_FILE"

echo "RX buffer configuration completed."
