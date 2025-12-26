#!/bin/bash -u

# This script checks various memory metrics to determine if the node is experiencing memory pressure
# If pressure is detected, it logs top, cgtop, and crictl results to log messages in the Extension's events directory
# Refer to this spec for fileformat for sending logs to Kusto via Wireserver:
# https://github.com/Azure/azure-vmextension-publishing/wiki/5.0-Telemetry-Events

set -o nounset
set -o pipefail

# Source common functions
SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"
source "${SCRIPT_DIR}/npd_common.sh" || { echo "ERROR: Failed to source npd_common.sh"; exit 1; }
source "${SCRIPT_DIR}/pressure_common.sh" || { echo "ERROR: Failed to source pressure_common.sh"; exit 1; }
# Exit codes
OK=0
NOTOK=0   # Always exit with OK for now so we don't raise an event
UNKNOWN=0 # Always exit with OK for now so we don't raise an event

# Configurable thresholds (can be overridden via environment variables)
MEMORY_AVAILABLE_THRESHOLD="${MEMORY_AVAILABLE_THRESHOLD:-10}"  # 10% of total memory available
PSI_MEMORY_SOME_THRESHOLD="${PSI_MEMORY_SOME_THRESHOLD:-50}"  # 50% memory stall over 1 minutes

# Clean up old log files
cleanup_old_logs

# Get total memory in kB
TOTAL_MEMORY_KB=$(grep MemTotal /proc/meminfo | awk '{print $2}')
if [ -z "$TOTAL_MEMORY_KB" ]; then
    log "Failed to determine total memory"
    exit $UNKNOWN
fi

# Convert to MB for easier reading
TOTAL_MEMORY_MB=$((TOTAL_MEMORY_KB / 1024))

# Calculate memory threshold in kB
MEMORY_AVAILABLE_THRESHOLD_KB=$((TOTAL_MEMORY_KB * MEMORY_AVAILABLE_THRESHOLD / 100))

log "Total memory: $TOTAL_MEMORY_MB MB"
log "Memory available threshold: $MEMORY_AVAILABLE_THRESHOLD% ($((MEMORY_AVAILABLE_THRESHOLD_KB / 1024)) MB)"

# Function to check if memory is under pressure
check_memory_pressure() {
    # Initialize pressure flag
    local pressure_detected=0
        
    # Check cgroup v2 PSI metrics if available
    if [ -f "/sys/fs/cgroup/memory.pressure" ]; then
        PSI_MEMORY_SOME=$(cat /sys/fs/cgroup/memory.pressure | awk '/some/ {print $3}' | cut -d= -f2)
        
        log "PSI memory some avg60: ${PSI_MEMORY_SOME:-not available}"
        
        if [ -n "$PSI_MEMORY_SOME" ] && [ "$(echo "$PSI_MEMORY_SOME > $PSI_MEMORY_SOME_THRESHOLD" | bc)" -eq 1 ]; then
            log "PRESSURE: High memory pressure detected via cgroup PSI: ${PSI_MEMORY_SOME}% (threshold: ${PSI_MEMORY_SOME_THRESHOLD}%)"
            pressure_detected=1
        fi
    else
        log "sys/fs/cgroup/memory.pressure not available"
    fi

    # Check available memory
    MEMORY_AVAILABLE_KB=$(grep MemAvailable /proc/meminfo | awk '{print $2}')
    MEMORY_FREE_KB=$(grep MemFree /proc/meminfo | awk '{print $2}')
    MEMORY_BUFFERS_KB=$(grep Buffers /proc/meminfo | awk '{print $2}')
    MEMORY_CACHED_KB=$(grep "^Cached:" /proc/meminfo | awk '{print $2}')
    
    # If MemAvailable is not available (older kernels), calculate it
    if [ -z "$MEMORY_AVAILABLE_KB" ]; then
        MEMORY_AVAILABLE_KB=$((MEMORY_FREE_KB + MEMORY_BUFFERS_KB + MEMORY_CACHED_KB))
    fi
    
    # Calculate memory usage percentage
    MEMORY_USED_KB=$((TOTAL_MEMORY_KB - MEMORY_AVAILABLE_KB))
    MEMORY_USED_PCT=$((MEMORY_USED_KB * 100 / TOTAL_MEMORY_KB))
    
    log "Memory stats from /proc/meminfo:"
    log "  Total: $((TOTAL_MEMORY_KB / 1024)) MB"
    log "  Available: $((MEMORY_AVAILABLE_KB / 1024)) MB"
    log "  Used: $((MEMORY_USED_KB / 1024)) MB ($MEMORY_USED_PCT%)"
    
    if [ "$MEMORY_AVAILABLE_KB" -lt "$MEMORY_AVAILABLE_THRESHOLD_KB" ]; then
        log "PRESSURE: Low available memory: $((MEMORY_AVAILABLE_KB / 1024)) MB (threshold: $((MEMORY_AVAILABLE_THRESHOLD_KB / 1024)) MB)"
        pressure_detected=1
    fi

    # Reset the global OOM messages variable
    RECENT_OOM_KILLS=""

    if command -v dmesg >/dev/null 2>&1; then        
        log "Checking for OOM events in the last 5 minutes..."
        
        # Get system uptime in seconds
        UPTIME_SECONDS=$(cat /proc/uptime | awk '{print $1}' | cut -d. -f1)
        DMESG_CUTOFF=$((UPTIME_SECONDS - 300))  # 5 minutes ago in uptime seconds
        
        if [ "$DMESG_CUTOFF" -lt 0 ]; then
            DMESG_CUTOFF=0  # If system uptime is less than 5 minutes
        fi
        
        log "Current uptime: $UPTIME_SECONDS seconds, cutoff time: $DMESG_CUTOFF seconds"
                
        RECENT_OOM_KILLS=$(dmesg | awk -v cutoff="$DMESG_CUTOFF" '
            # Match various OOM message patterns
            /[Oo]ut of [Mm]emory/ || /OOM/ || /oom/ {
                # Extract timestamp (seconds) from dmesg output
                if ($1 ~ /^\[/) {
                    # Extract the timestamp between [ and ]
                    ts = $1
                    gsub(/[\[\]]/, "", ts)
                    # Extract seconds part before the dot
                    split(ts, parts, ".")
                    timestamp = parts[1]
                    # Compare with cutoff time
                    if (timestamp >= cutoff) {
                        print
                    }
                } else {
                    # No timestamp available, include all matches as a fallback
                    print
                }
            }
        ')
        
        if [ -n "$RECENT_OOM_KILLS" ]; then
            log "PRESSURE: Recent OOM kills detected in kernel log (last 5 minutes):"
            # Not setting pressure here for now as OOMs in logs often don't correlate 
            # with actual memory pressure at the time of the check
            # pressure_detected=1 
        else
            log "No OOM kills detected in the last 5 minutes"
        fi
    fi
    
    # Return overall pressure status
    if [ "$pressure_detected" -eq 1 ]; then
        return 1
    else
        return 0
    fi
}

# Function to log OOM events in JSON format
log_ooms() {
    local OOM_MESSAGES="$1"
    
    if [ -z "$OOM_MESSAGES" ]; then
        log "No OOM messages to log"
        return
    fi
    
    log "Logging OOM events"
    
    OOM_MESSAGES_TRUNCATED=$(echo "$OOM_MESSAGES" | tail -n 10)
    
    OOM_MESSAGES_COUNT=$(echo "$OOM_MESSAGES_TRUNCATED" | wc -l)
    if [ "$OOM_MESSAGES_COUNT" -gt 10 ]; then
        log "Truncated OOM messages from $OOM_MESSAGES_COUNT to 10 (showing most recent)"
    fi
    
    BOOT_TIME=$(date +%s -d "$(uptime -s)")
    log "System boot time: $(date -d @"$BOOT_TIME")"
    
    TEMP_JSON_FILE=$(mktemp)
    
    echo "[" > "$TEMP_JSON_FILE"
    
    LINE_COUNT=0
    while IFS= read -r line; do
        LINE_COUNT=$((LINE_COUNT + 1))
        
        timestamp=""
        message="$line"
        if [[ "$line" =~ ^\[(.*)\] ]]; then
            dmesg_timestamp="${BASH_REMATCH[1]}"
            dmesg_seconds=$(echo "$dmesg_timestamp" | cut -d. -f1)
            
            if [[ "$dmesg_seconds" =~ ^[0-9]+$ ]]; then
                system_time=$((BOOT_TIME + dmesg_seconds))
                timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ" -d @$system_time)
            fi
            
            # Remove timestamp from message for cleaner output
            message="${line#*] }"
        fi
        
        # Escape special characters for JSON
        message=$(echo "$message" | sed 's/\\/\\\\/g; s/"/\\"/g')
        
        comma=""
        if [ "$LINE_COUNT" -lt "$OOM_MESSAGES_COUNT" ]; then
            comma=","
        fi
        
        echo "  {\"timestamp\":\"$timestamp\",\"message\":\"$message\"}$comma" >> "$TEMP_JSON_FILE"
    done <<< "$OOM_MESSAGES_TRUNCATED"
    
    echo "]" >> "$TEMP_JSON_FILE"
    
    OOM_JSON=$(cat "$TEMP_JSON_FILE")
    
    rm -f "$TEMP_JSON_FILE"
    
    if ! echo "$OOM_JSON" | jq . >/dev/null 2>&1; then
        log "ERROR: Failed to create valid JSON from OOM messages. Using simplified format."
        # Create a simplified valid JSON array as fallback
        OOM_JSON="[{\"message\":\"OOM events detected but could not be parsed into JSON\"}]"
    fi
    
    OOM_DATA_JSON=$(jq -n \
                    --argjson oom_events "$OOM_JSON" \
                    '{
                      oom_events: $oom_events
                    }')
        
    write_event_log "npd:check_memory_pressure:oom" "$OOM_DATA_JSON"
}

# Main logic - check for memory pressure once
RECENT_OOM_KILLS=""  # Initialize RECENT_OOM_KILLS at the global scope

# Run single pressure check
if ! check_memory_pressure; then
    log "Memory pressure detected on node"
    
    # Use common functions for logging
    log_top_results "npd:check_memory_pressure:top" "memory"
    log_cgtop_results "npd:check_memory_pressure:systemd-cgtop" "-m"
    log_crictl_stats "npd:check_memory_pressure:crictl_stats" "memory" "" "$TOTAL_MEMORY_KB"
    
    # Log OOM events if we have any
    if [ -n "$RECENT_OOM_KILLS" ]; then
        log_ooms "$RECENT_OOM_KILLS"
    fi
    
    write_logs "npd:check_memory_pressure:log"

    exit $NOTOK
else
    log "No memory pressure detected on node"
    exit $OK
fi
