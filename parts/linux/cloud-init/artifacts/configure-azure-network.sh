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

# Determine RX buffer size based on number of CPUs
NUM_CPUS=$(nproc)
if [ "$NUM_CPUS" -ge 4 ]; then
    RX_SIZE=2048
else
    RX_SIZE=1024
fi

echo "Detected $NUM_CPUS CPUs, configuring $INTERFACE with rx=$RX_SIZE"
ethtool -G "$INTERFACE" rx "$RX_SIZE" || echo "Failed to set ring parameters for $INTERFACE"
