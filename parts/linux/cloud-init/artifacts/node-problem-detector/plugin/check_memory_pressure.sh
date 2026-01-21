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

# Clean up old log files
cleanup_old_logs

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
# Initialize global variables that check_memory_pressure will set
RECENT_OOM_KILLS=""
TOTAL_MEMORY_KB=""

# Run single pressure check
# Thresholds can be overridden via environment variables (see pressure_common.sh)
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
