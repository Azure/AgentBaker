#!/bin/bash

# This script checks various CPU metrics to determine if the node is experiencing CPU pressure
# If pressure is detected, it logs iotop and top results to a log messages in the Extension's events directory
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

# Command truncation length for iotop output
MAX_COMMAND_LENGTH="${MAX_COMMAND_LENGTH:-30}"  # Maximum length for command strings in JSON output

# Clean up old log files
cleanup_old_logs

# Get number of CPU cores
NUM_CORES=$(nproc)

# Function to log iotop results in JSON format
log_iotop_results() {
    # Check if iotop is available
    if ! command -v iotop >/dev/null 2>&1; then
        log "iotop not installed, cannot log I/O statistics"
        return
    fi
    
    log "Running iotop command..."
    
    # Run iotop and capture output
    IOTOP_RAW=$(timeout 15s iotop -b -o -n 2 -d 3 | head -n 20 2>&1)
    IOTOP_EXIT_CODE=$?

    # It is unsafe to directly log IOTOP_RAW as it may contain sensitive information 
    # (e.g. credentials passed on the commandline) 
    # so we use a redacted version for output
    IOTOP_RAW="$(redact_sensitive_data "$IOTOP_RAW")"

    # Check if iotop failed or returned an error message with more detailed error reporting
    # Note: Exit code 141 is SIGPIPE which is expected when using head, and not actually an error
    if [ $IOTOP_EXIT_CODE -ne 0 ] && [ $IOTOP_EXIT_CODE -ne 141 ]; then
        log "WARNING: iotop command failed with exit code $IOTOP_EXIT_CODE"
        log "iotop output: $IOTOP_RAW"
        return
    elif [[ "$IOTOP_RAW" == *"Permission denied"* ]]; then
        log "WARNING: iotop permission denied - this script needs to run as root"
        return
    elif [[ "$IOTOP_RAW" == *"Error:"* ]]; then
        log "WARNING: iotop returned an error: $IOTOP_RAW"
        return
    elif [ -z "$IOTOP_RAW" ]; then
        log "WARNING: iotop returned empty output"
        return
    fi
    
    # Extract summary information from the first line
    # Note: iotop can display B/s, K/s, M/s, G/s, etc.
    TOTAL_READ=$(echo "$IOTOP_RAW" | grep -oP 'Total DISK READ: +\K[0-9.]+ [BKMG]/s' || echo "N/A")
    TOTAL_WRITE=$(echo "$IOTOP_RAW" | grep -oP 'Total DISK WRITE: +\K[0-9.]+ [BKMG]/s' || echo "N/A")
    CURRENT_READ=$(echo "$IOTOP_RAW" | grep -oP 'Current DISK READ: +\K[0-9.]+ [BKMG]/s' || echo "N/A")
    CURRENT_WRITE=$(echo "$IOTOP_RAW" | grep -oP 'Current DISK WRITE: +\K[0-9.]+ [BKMG]/s' || echo "N/A")
    
    # Validate that we have valid iotop data (not all N/A values)
    if [ "$TOTAL_READ" = "N/A" ] && [ "$TOTAL_WRITE" = "N/A" ] && [ "$CURRENT_READ" = "N/A" ] && [ "$CURRENT_WRITE" = "N/A" ]; then
        log "WARNING: iotop output does not contain valid disk I/O statistics"
        return
    fi
    
    # Process the iotop output to create a structured JSON array of processes
    PROCESSES_JSON=$(echo "$IOTOP_RAW" | awk '
      # Skip header lines
      NR <= 2 {next}
      # Process data lines
      {
        # Check if this is a valid process line with a PID
        if ($1 ~ /^[0-9]+$/) {
          pid = $1
          prio = $2
          user = $3
          disk_read = $4 " " $5
          disk_write = $6 " " $7
          
          # Capture the full command line starting from field 9
          cmd = ""
          for (i = 9; i <= NF; i++) {
            if (i == 9) {
              cmd = $i
            } else {
              cmd = cmd " " $i
            }
          }
          
          # Truncate very long commands to prevent JSON issues
          if (length(cmd) > '$MAX_COMMAND_LENGTH') {
            cmd = substr(cmd, 1, '$MAX_COMMAND_LENGTH'-3) "..."
          }
          
          # Escape special characters for JSON
          gsub(/\\/, "\\\\", cmd)  # Escape backslashes first
          gsub(/"/, "\\\"", cmd)   # Escape quotes
          gsub(/\t/, "\\t", cmd)   # Escape tabs
          gsub(/\n/, "\\n", cmd)   # Escape newlines
          gsub(/\r/, "\\r", cmd)   # Escape carriage returns
          
          # Also escape special chars in user field which could contain problematic characters
          gsub(/\\/, "\\\\", user)
          gsub(/"/, "\\\"", user)
          gsub(/\t/, "\\t", user)
          gsub(/\n/, "\\n", user)
          gsub(/\r/, "\\r", user)
          
          # Output in a format that can be converted to JSON
          printf "{\"pid\":\"%s\",\"prio\":\"%s\",\"user\":\"%s\",\"disk_read\":\"%s\",\"disk_write\":\"%s\",\"command\":\"%s\"},\n", 
                 pid, prio, user, disk_read, disk_write, cmd
        }
      }
    ' | sed '$s/,$//' | awk 'BEGIN{print "["} {print} END{print "]"}')
    
    # Validate JSON format before processing
    if ! echo "$PROCESSES_JSON" | jq . >/dev/null 2>&1; then
        log "WARNING: Invalid JSON generated from iotop output, logging raw data instead"
        log "Raw iotop output: $IOTOP_RAW"
        return
    fi
    
    # Limit the number of processes to keep message length within max length
    PROCESSES_JSON_LIMITED=$(echo "$PROCESSES_JSON" | jq -c '.[0:12]' 2>/dev/null)
    
    # Check if jq processing succeeded
    if [ $? -ne 0 ] || [ -z "$PROCESSES_JSON_LIMITED" ]; then
        log "WARNING: Failed to process iotop JSON data"
        return
    fi
    
    IO_DATA_JSON=$(jq -n \
                    --arg total_read "$TOTAL_READ" \
                    --arg total_write "$TOTAL_WRITE" \
                    --arg current_read "$CURRENT_READ" \
                    --arg current_write "$CURRENT_WRITE" \
                    --argjson processes "$PROCESSES_JSON_LIMITED" \
                    '{
                      summary: {
                        total_disk_read: $total_read,
                        total_disk_write: $total_write,
                        current_disk_read: $current_read,
                        current_disk_write: $current_write
                      },
                      processes: $processes
                    }' 2>/dev/null)
    
    # Check if final JSON creation succeeded
    if [ $? -ne 0 ] || [ -z "$IO_DATA_JSON" ]; then
        log "WARNING: Failed to create final iotop JSON data"
        return
    fi
    
    write_chunked_event_log "npd:check_cpu_pressure:iotop" "$IO_DATA_JSON" 3
}

## Main logic - check for CPU pressure once
# Run single pressure check
# Thresholds can be overridden via environment variables (see pressure_common.sh)
if ! check_cpu_pressure; then
    log "CPU pressure detected on node"
        
    log_iotop_results
    
    # Use common functions for logging
    log_top_results "npd:check_cpu_pressure:top" "cpu"
    log_cgtop_results "npd:check_cpu_pressure:systemd-cgtop" "-c"
    log_crictl_stats "npd:check_cpu_pressure:crictl_stats" "cpu" "$NUM_CORES" ""
    
    write_logs "npd:check_cpu_pressure:log"

    exit $NOTOK
else
    log "No CPU pressure detected on node"
    exit $OK
fi
