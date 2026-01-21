#!/bin/bash
# Azure Network Tuning Script
# This script sets the network interface RX ring buffer to 2048 for multi-core systems
# Only updates the value if the current RX buffer is 1024
# Called by udev with interface name as first argument

set -e

INTERFACE="$1"
LOGFILE="/var/log/azure-network-tuning.log"

log() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') - $*" >> "$LOGFILE"
}

# Exit successfully if no interface provided
if [ -z "$INTERFACE" ]; then
    log "No interface provided, exiting"
    exit 0
fi

log "Processing interface: $INTERFACE"

# Check if system has at least 4 CPU cores
NPROC=$(/usr/bin/nproc)
log "Number of CPU cores: $NPROC"

if [ "$NPROC" -ge 4 ]; then
    # Get current RX buffer size
    current=$(/usr/sbin/ethtool -g "$INTERFACE" 2>/dev/null | grep -A5 "Current hardware settings:" | grep "RX:" | awk '{print $2}')

    if [ -z "$current" ]; then
        log "Could not retrieve RX buffer size for $INTERFACE (ethtool may not support -g)"
        exit 0
    fi

    log "Current RX buffer size for $INTERFACE: $current"

    # Only update if current RX buffer is exactly 1024
    if [ "$current" = "1024" ]; then
        log "Updating RX buffer size from 1024 to 2048 for $INTERFACE"
        if /usr/sbin/ethtool -G "$INTERFACE" rx 2048; then
            log "Successfully updated RX buffer size for $INTERFACE"
        else
            log "Failed to update RX buffer size for $INTERFACE"
        fi
    else
        log "RX buffer size is $current, not updating (only updates when current is 1024)"
    fi
else
    log "System has less than 4 CPU cores ($NPROC), skipping RX buffer update"
fi

exit 0
