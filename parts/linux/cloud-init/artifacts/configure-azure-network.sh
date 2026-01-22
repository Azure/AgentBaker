#!/bin/bash

# This script configures network interface settings for Azure NICs.
# Called by udev with interface name as argument

INTERFACE="$1"

# Exit if no interface provided
if [ -z "$INTERFACE" ]; then
    echo "No interface provided, exiting"
    exit 0
fi

# Check if interface exists
if [ ! -d "/sys/class/net/$INTERFACE" ]; then
    echo "NIC $INTERFACE does not exist. Skipping."
    exit 0
fi

# Determine default RX buffer size based on number of CPUs
NUM_CPUS=$(nproc)
if [ "$NUM_CPUS" -ge 4 ]; then
    DEFAULT_RX_BUFFER_SIZE=2048
else
    DEFAULT_RX_BUFFER_SIZE=1024
fi

# Use default unless overridden by config file
RX_SIZE=$DEFAULT_RX_BUFFER_SIZE
ETHTOOL_CONFIG_FILE="/etc/azure-network/ethtool.conf"

if [ -f "$ETHTOOL_CONFIG_FILE" ]; then
    CONFIG_RX_SIZE=$(grep "^rx=" "$ETHTOOL_CONFIG_FILE" | cut -d'=' -f2)
    if [ -n "$CONFIG_RX_SIZE" ] && [ "$CONFIG_RX_SIZE" -gt 0 ] 2>/dev/null; then
        RX_SIZE=$CONFIG_RX_SIZE
        echo "Using configured rx buffer size from $ETHTOOL_CONFIG_FILE: $RX_SIZE"
    fi
fi

echo "Detected $NUM_CPUS CPUs, configuring $INTERFACE with rx=$RX_SIZE"
ethtool -G "$INTERFACE" rx "$RX_SIZE" || echo "Failed to set ring parameters for $INTERFACE"
