#!/bin/bash

# Script to check for RX out of buffer errors on PCI network interfaces
# The script calculates the ratio of rx_out_of_buffer / rx_packets
# and alerts if the ratio exceeds the threshold

set -o nounset
set -o pipefail

# Capture start time
START_TIME=$EPOCHREALTIME

# Source common functions
SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"
source "${SCRIPT_DIR}/npd_common.sh" || { echo "ERROR: Failed to source npd_common.sh"; exit 0; }

OK=0 # exit code

# Check if bc tool is available
if ! command -v bc >/dev/null 2>&1; then
    echo "ERROR: bc command not found, cannot perform calculations"
    exit 0
fi

# Function to get PCI network interfaces
get_pci_interfaces() {
    # Check if jq is available
    if ! command -v jq >/dev/null 2>&1; then
        log "ERROR: jq command not found, cannot detect PCI interfaces"
        write_logs "npd:rx_buffer_errors:log"
        exit 0
    fi
    
    # Get PCI interfaces using ip and jq
    ip -j -d link show 2>/dev/null | jq -r 'map(select(.parentbus == "pci"))[].ifname' 2>/dev/null || echo ""
}

# Metrics to check
BUFFER_ERROR_METRIC="rx_out_of_buffer"
PACKETS_METRIC="rx_packets"

# Threshold ratio above which we consider it a problem
THRESHOLD_RATIO=0.0001 # 0.01%

# Force diagnostic logging for testing (overrideable via environment)
FORCE_DIAGNOSTICS=${FORCE_DIAGNOSTICS:-false}

BUFFER_ERROR_DETECTED=false

# State file to store previous values (separate file for each interface)
STATE_DIR="/var/lib/node-problem-detector/rx-buffer-check"
mkdir -p "$STATE_DIR" || log "WARNING: Could not create state directory: $STATE_DIR"

log "Starting RX buffer errors check"
log "Checking network interfaces for rx_out_of_buffer metric"

# Get network interfaces to check
INTERFACES_TO_CHECK=$(get_pci_interfaces)
if [ -z "$INTERFACES_TO_CHECK" ]; then
    log "No network interfaces found with required metrics. Exiting."
    exit $OK
fi

log "Network interfaces to check: $INTERFACES_TO_CHECK"

for IFACE in $INTERFACES_TO_CHECK; do
    log "Checking interface: $IFACE"

    # Get ethtool statistics
    STATS=$(ethtool -S "$IFACE" 2>/dev/null || echo "")
    if [ -z "$STATS" ]; then
        log "Could not get ethtool stats for $IFACE, skipping."
        continue
    fi
    
    # Check for required metrics
    BUFFER_ERROR_COUNT=$(echo "$STATS" | awk "tolower(\$0) ~ /$BUFFER_ERROR_METRIC/ {printf \$NF}")
    PACKETS_COUNT=$(echo "$STATS" | awk "tolower(\$0) ~ /$PACKETS_METRIC/ {printf \$NF}")
    
    # Skip if either metric is missing
    if [ -z "$BUFFER_ERROR_COUNT" ]; then
        log "Metric '$BUFFER_ERROR_METRIC' not found or empty for $IFACE, skipping."
        continue
    fi
    if [ -z "$PACKETS_COUNT" ]; then
        log "Metric '$PACKETS_METRIC' not found or empty for $IFACE, skipping."
        continue
    fi
    
    # Verify both counts are numbers
    if ! [[ "$BUFFER_ERROR_COUNT" =~ ^[0-9]+$ ]]; then
        log "Invalid buffer error count for $IFACE: $BUFFER_ERROR_COUNT"
        continue
    fi
    if ! [[ "$PACKETS_COUNT" =~ ^[0-9]+$ ]]; then
        log "Invalid packets count for $IFACE: $PACKETS_COUNT"
        continue
    fi
    
    log "Current metrics for $IFACE:"
    log "  $BUFFER_ERROR_METRIC: $BUFFER_ERROR_COUNT"
    log "  $PACKETS_METRIC: $PACKETS_COUNT"
    
    # State file for this interface
    STATE_FILE="$STATE_DIR/$IFACE.state"
    
    # Initialize variables for previous values
    PREV_BUFFER_ERROR_COUNT=0
    PREV_PACKETS_COUNT=0
    PREV_TIMESTAMP=0
    
    # Read previous values if state file exists
    if [ -f "$STATE_FILE" ]; then
        # File format: timestamp buffer_errors packets
        PREV_DATA=$(cat "$STATE_FILE" 2>/dev/null || echo "")
        if [ -n "$PREV_DATA" ]; then
            PREV_TIMESTAMP=$(echo "$PREV_DATA" | awk '{print $1}')
            PREV_BUFFER_ERROR_COUNT=$(echo "$PREV_DATA" | awk '{print $2}')
            PREV_PACKETS_COUNT=$(echo "$PREV_DATA" | awk '{print $3}')
            
            log "Previous metrics for $IFACE (from $(date -d @"$PREV_TIMESTAMP")):"
            log "  $BUFFER_ERROR_METRIC: $PREV_BUFFER_ERROR_COUNT"
            log "  $PACKETS_METRIC: $PREV_PACKETS_COUNT"
        fi
    fi
    
    # Save current values for next run
    TIMESTAMP=$(date +%s)
    echo "$TIMESTAMP $BUFFER_ERROR_COUNT $PACKETS_COUNT" > "$STATE_FILE"
    
    # Skip ratio calculation on first run (no previous data)
    if [ "$PREV_TIMESTAMP" -eq 0 ]; then
        log "First run for $IFACE, saving baseline metrics. Error rate will be calculated on next run."
        continue
    fi
    
    # Calculate the deltas
    DELTA_TIME=$((TIMESTAMP - PREV_TIMESTAMP))
    if [ "$DELTA_TIME" -eq 0 ]; then
        log "Warning: Timestamp delta is 0, skipping calculation for $IFACE"
        continue
    fi
    
    BUFFER_ERROR_DELTA=$((BUFFER_ERROR_COUNT - PREV_BUFFER_ERROR_COUNT))
    PACKETS_DELTA=$((PACKETS_COUNT - PREV_PACKETS_COUNT))
    
    log "Metric deltas for $IFACE over ${DELTA_TIME}s:"
    log "  $BUFFER_ERROR_METRIC delta: $BUFFER_ERROR_DELTA"
    log "  $PACKETS_METRIC delta: $PACKETS_DELTA"
    
    # Skip if no new packets
    if [ "$PACKETS_DELTA" -eq 0 ]; then
        log "No new packets received on $IFACE since last check, skipping error rate calculation."
        continue
    fi
    
    # Calculate the ratio
    RATIO=$(echo "scale=10; $BUFFER_ERROR_DELTA / $PACKETS_DELTA" | bc)
    log "Ratio of $BUFFER_ERROR_METRIC to $PACKETS_METRIC: $RATIO (threshold: $THRESHOLD_RATIO)"
    
    if [ "$(echo "$RATIO > $THRESHOLD_RATIO" | bc)" -eq 1 ] || [ "$FORCE_DIAGNOSTICS" = "true" ]; then
        # Problem detected - rate of buffer errors is above threshold
        BUFFER_ERROR_DETECTED=true
        log "PROBLEM DETECTED: Interface $IFACE buffer error ratio ($RATIO) exceeds threshold ($THRESHOLD_RATIO)"
                
        # Use already collected ethtool statistics
        log "Recording ethtool stats for $IFACE..."
        write_chunked_event_log "npd:rx_buffer_errors:ethtool_stats_${IFACE}" "$STATS" 3
        
        # Collect additional interface details
        log "Collecting detailed link stats for $IFACE..."
        IFACE_DETAILS=$(ip -s -s link show dev "$IFACE" 2>&1)
        write_event_log "npd:rx_buffer_errors:link_stats_${IFACE}" "$IFACE_DETAILS"
    else
        log "Interface $IFACE buffer error ratio ($RATIO) is below threshold ($THRESHOLD_RATIO)"
    fi
done

# Log script execution time
ELAPSED_TIME=$(bc <<<"scale=2; $EPOCHREALTIME - $START_TIME")
log "Script execution time: ${ELAPSED_TIME} seconds"

if [ "$BUFFER_ERROR_DETECTED" = true ]; then
    # Write all collected logs to the event file
    write_logs "npd:rx_buffer_errors:log"
fi

# always exit 0 for now so we don't raise an event
exit $OK 