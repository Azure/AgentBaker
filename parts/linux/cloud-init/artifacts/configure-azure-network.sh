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

# Get current RX buffer size
CURRENT_RX=$(ethtool -g "$INTERFACE" 2>/dev/null | grep -A4 "Current hardware settings" | grep "^RX:" | awk '{print $2}')

# Only proceed if current RX is 1024
if [ "$CURRENT_RX" != "1024" ]; then
    echo "Current RX buffer size is $CURRENT_RX (not 1024), skipping configuration for $INTERFACE"
    exit 0
fi

# Use default unless overridden by config file
RX_SIZE=$DEFAULT_RX_BUFFER_SIZE

echo "Detected $NUM_CPUS CPUs, current RX is 1024, configuring $INTERFACE with rx=$RX_SIZE"
ethtool -G "$INTERFACE" rx "$RX_SIZE" || echo "Failed to set ring parameters for $INTERFACE"
