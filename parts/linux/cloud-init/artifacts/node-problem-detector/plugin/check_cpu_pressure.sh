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

# Configurable thresholds (can be overridden via environment variables)
CPU_LOAD_THRESHOLD="${CPU_LOAD_THRESHOLD:-0.9}"  # 90% of available CPUs
CPU_IOWAIT_THRESHOLD="${CPU_IOWAIT_THRESHOLD:-20}"  # 20% in iowait state
CPU_STEAL_THRESHOLD="${CPU_STEAL_THRESHOLD:-10}"  # 10% CPU steal time
# PSI thresholds - these match Kubernetes defaults for node pressure eviction
PSI_CPU_SOME_THRESHOLD="${PSI_CPU_SOME_THRESHOLD:-60}"  # 60% CPU stall over 5 minutes
PSI_IO_SOME_THRESHOLD="${PSI_IO_SOME_THRESHOLD:-40}"  # 40% IO stall over 5 minutes
PSI_CPU_SOME_PERIOD="${PSI_CPU_SOME_PERIOD:-300000}"    # 5 minutes in milliseconds
# Command truncation length for iotop output
MAX_COMMAND_LENGTH="${MAX_COMMAND_LENGTH:-30}"  # Maximum length for command strings in JSON output

# Clean up old log files
cleanup_old_logs

# Get number of CPU cores
NUM_CORES=$(nproc)

# Calculate load threshold based on number of cores
LOAD_THRESHOLD=$(echo "$NUM_CORES * $CPU_LOAD_THRESHOLD" | bc)

log "Number of CPU cores: $NUM_CORES"
log "Load threshold: $LOAD_THRESHOLD (${CPU_LOAD_THRESHOLD} * $NUM_CORES)"

# Function to check if CPU is under pressure
check_cpu_pressure() {
    # Initialize pressure flag
    local pressure_detected=0
        
    # Check cgroup v2 PSI metrics if available
    if [ -f "/sys/fs/cgroup/cpu.pressure" ]; then
        PSI_CPU_SOME=$(cat /sys/fs/cgroup/cpu.pressure | awk '/some/ {print $4}' | cut -d= -f2)
        
        log "PSI CPU some avg300: ${PSI_CPU_SOME:-not available}"
        
        if [ -n "$PSI_CPU_SOME" ] && [ "$(echo "$PSI_CPU_SOME > $PSI_CPU_SOME_THRESHOLD" | bc)" -eq 1 ]; then
            log "PRESSURE: High CPU pressure detected via cgroup PSI (avg300): ${PSI_CPU_SOME}% (threshold: ${PSI_CPU_SOME_THRESHOLD}%)"
            pressure_detected=1
        fi

         # Check for IO pressure
        if [ -f "/sys/fs/cgroup/io.pressure" ]; then
            PSI_IO_SOME=$(cat /sys/fs/cgroup/io.pressure | awk '/some/ {print $4}' | cut -d= -f2)
            
            log "PSI IO some avg300: ${PSI_IO_SOME:-not available}"
            
            if [ -n "$PSI_IO_SOME" ] && [ "$(echo "$PSI_IO_SOME > $PSI_IO_SOME_THRESHOLD" | bc)" -eq 1 ]; then
                log "PRESSURE: High IO pressure detected via cgroup PSI (avg300): ${PSI_IO_SOME}% (threshold: ${PSI_IO_SOME_THRESHOLD}%)"
                pressure_detected=1
            fi
        else
            log "sys/fs/cgroup/io.pressure not available"
        fi        
    else
        log "sys/fs/cgroup/cpu.pressure not available"
    fi

    # Check current load average (1 minute)
    LOAD_AVG=$(cat /proc/loadavg | awk '{print $1}')
    log "/proc/loadavg: $LOAD_AVG"
    
    if [ "$(echo "$LOAD_AVG > $LOAD_THRESHOLD" | bc)" -eq 1 ]; then
        log "INFO: High CPU load: $LOAD_AVG"
        # pressure_detected=1 # Not setting pressure here as this may be too sensitive
    fi

    # Get CPU usage statistics
    if command -v mpstat >/dev/null 2>&1; then
        MPSTAT_OUTPUT=$(mpstat 1 1 | grep -v Linux | tail -n 1)
        
        if [ -n "$MPSTAT_OUTPUT" ]; then
            # Format: CPU %usr %nice %sys %iowait %irq %soft %steal %guest %nice %idle
            USER_PCT=0
            SYSTEM_PCT=0
            IOWAIT_PCT=0
            STEAL_PCT=0
            IDLE_PCT=0
                            
            USER_PCT=$(echo "$MPSTAT_OUTPUT" | awk '{print $3}')
            SYSTEM_PCT=$(echo "$MPSTAT_OUTPUT" | awk '{print $5}')
            IOWAIT_PCT=$(echo "$MPSTAT_OUTPUT" | awk '{print $6}')
            STEAL_PCT=$(echo "$MPSTAT_OUTPUT" | awk '{print $9}')
            IDLE_PCT=$(echo "$MPSTAT_OUTPUT" | awk '{print $12}')
            
            log "CPU stats from mpstat: user: ${USER_PCT}% system: ${SYSTEM_PCT}% iowait: ${IOWAIT_PCT}% steal: ${STEAL_PCT}% idle: ${IDLE_PCT}%"
        else
            log "Failed to get CPU statistics from mpstat"
        fi
    else
        log "mpstat not available"
    fi    
    
    # Check for high iowait
    if [ "$(echo "$IOWAIT_PCT > $CPU_IOWAIT_THRESHOLD" | bc)" -eq 1 ]; then
        log "INFO: High CPU iowait: ${IOWAIT_PCT}% (threshold: ${CPU_IOWAIT_THRESHOLD}%)"
        pressure_detected=1
    fi
    
    # Check for high CPU steal
    if [ "$(echo "$STEAL_PCT > $CPU_STEAL_THRESHOLD" | bc)" -eq 1 ]; then
        log "INFO: High CPU steal time: ${STEAL_PCT}%"
        # pressure_detected=1 # Not setting pressure here as this may be too sensitive
    fi
    
    # Check for throttling events
    if [ -f "/sys/fs/cgroup/cpu.stat" ]; then
        THROTTLED_TIME=$(grep 'throttled_usec' /sys/fs/cgroup/cpu.stat | awk '{print $2}')
        
        log "CPU throttled time: ${THROTTLED_TIME:-0} us"
        
        if [ -n "$THROTTLED_TIME" ] && [ "$THROTTLED_TIME" -gt 0 ]; then
            log "PRESSURE: CPU throttling detected: $THROTTLED_TIME us"
            # pressure_detected=1 # Not setting pressure here as this may be too sensitive
        fi
    else
        log "sys/fs/cgroup/cpu/cpu.stat not available"
    fi
    
    # Return overall pressure status
    if [ "$pressure_detected" -eq 1 ]; then
        return 1
    else
        return 0
    fi
}

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
