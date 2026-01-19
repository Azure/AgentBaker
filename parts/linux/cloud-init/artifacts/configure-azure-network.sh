#!/bin/bash

# This script configures network interface settings for Azure NICs.

NICS_TO_CONFIGURE_FILE="/etc/azure-network/nics-to-configure"
DEFAULT_RX_BUFFER_SIZE=1024  # Fallback only - CPU logic handled by Node Controller

if [ ! -f "$NICS_TO_CONFIGURE_FILE" ]; then
    echo "No NICs to configure."
    exit 0
fi

echo "Configuring NICs listed in $NICS_TO_CONFIGURE_FILE"

# Parse ETHTOOL_CONTENT environment variable for RX buffer size
RX_SIZE=$DEFAULT_RX_BUFFER_SIZE
if [ -n "$ETHTOOL_CONTENT" ]; then
    # Decode base64 ETHTOOL_CONTENT and extract rx value
    ETHTOOL_CONFIG=$(echo "$ETHTOOL_CONTENT" | base64 -d 2>/dev/null)
    if [ -n "$ETHTOOL_CONFIG" ]; then
        CONFIG_RX_SIZE=$(echo "$ETHTOOL_CONFIG" | grep "^rx=" | cut -d'=' -f2)
        if [ -n "$CONFIG_RX_SIZE" ] && [ "$CONFIG_RX_SIZE" -gt 0 ] 2>/dev/null; then
            RX_SIZE=$CONFIG_RX_SIZE
            echo "Using ETHTOOL_CONTENT rx buffer size: $RX_SIZE"
        fi
    fi
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
