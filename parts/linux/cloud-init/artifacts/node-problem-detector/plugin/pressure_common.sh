#!/bin/bash

# Pressure-specific helper functions for Node Problem Detector plugins
# This file contains shared code used by both CPU and memory pressure detection scripts

# Source common functions
SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"
source "${SCRIPT_DIR}/npd_common.sh" || { echo "ERROR: Failed to source npd_common.sh"; exit 1; }

# Function to log top results in JSON format
log_top_results() {
    local taskname="$1"  # Pass in the specific taskname (e.g., "npd:check_cpu_pressure:top")
    local sort_by="$2"  # Pass in the sort flag (-c for CPU, -m for memory)
    
    # Check if top is available
    if ! command -v top >/dev/null 2>&1; then
        log "top not installed, cannot log system usage"
        return
    fi
    
    log "Running top command for $sort_by..."
    
    # Take into account the number of cores to get a reasonable number of lines
    # Larger nodes can dominate the top output with the list of each core's utilization e.g.
    # Tasks: 1249 total,  42 running, 1206 sleeping,   0 stopped,   1 zombie
    # %Cpu0  : 100.0/0.0   100[||||||||||]     %Cpu1  : 100.0/0.0   100[||||||||||]
    # %Cpu2  : 100.0/0.0   100[||||||||||]     %Cpu3  : 100.0/0.0   100[||||||||||]
    # ...
    # Get number of CPU cores
    local num_cores
    num_cores=$(nproc)
    local num_lines=$(($num_cores/2+30))

    log "Using $num_lines lines for top output"

    # Run top in batch mode, sorted by appropriate metric
    if [ "$sort_by" = "cpu" ]; then
        # Sort by CPU for CPU pressure
        TOP_RAW=$(timeout 10s top -b -w 150 -o %CPU -d 5 -n 1 | head -n $num_lines 2>&1)
    else
        # Sort by memory for memory pressure
        TOP_RAW=$(timeout 10s top -b -w 150 -o %MEM -d 1 -n 1 | head -n $num_lines 2>&1)
    fi
    
    TOP_EXIT_CODE=$?
    
    # Note: Exit code 141 is SIGPIPE which is expected when using head, and not actually an error
    if [ $TOP_EXIT_CODE -ne 0 ] && [ $TOP_EXIT_CODE -ne 141 ]; then
        log "WARNING: top command failed with exit code $TOP_EXIT_CODE"
        log "top output: $TOP_RAW"
        return
    elif [[ "$TOP_RAW" == *"Error"* ]]; then
        log "WARNING: top command returned an error."
        return
    elif [ -z "$TOP_RAW" ]; then
        log "WARNING: top returned empty output"
        return
    fi
    
    # Redact sensitive information from top output
    TOP_RAW=$(redact_sensitive_data "$TOP_RAW")
    
    write_chunked_event_log "$taskname" "$TOP_RAW" 3
}

# Function to log systemd-cgtop results
log_cgtop_results() {
    local taskname="$1"  # Pass in the specific taskname
    local sort_by="$2"  # Pass in the sort flag (-c for CPU, -m for memory)
    
    # Check if systemd-cgtop is available
    if ! command -v systemd-cgtop >/dev/null 2>&1; then
        log "systemd-cgtop not installed, cannot log cgroup statistics"
        return
    fi
    
    # Get total memory in KB for percentage calculations
    local TOTAL_MEMORY_KB=""
    if [ -f /proc/meminfo ]; then
        TOTAL_MEMORY_KB=$(grep MemTotal /proc/meminfo | awk '{print $2}')
    else
        log "WARNING: /proc/meminfo not available, cannot determine total memory"
        TOTAL_MEMORY_KB=0
    fi
    
    log "Running systemd-cgtop command for $sort_by..."
    
    CGTOP_RAW=$(timeout 15s systemd-cgtop -b -n 2 -d 2 --depth=3 "$sort_by" 2>&1)
    CGTOP_EXIT_CODE=$?
    
    # Note: Exit code 141 is SIGPIPE which is expected when using pipelines, and not actually an error
    if [ $CGTOP_EXIT_CODE -ne 0 ] && [ $CGTOP_EXIT_CODE -ne 141 ]; then
        log "WARNING: systemd-cgtop command failed with exit code $CGTOP_EXIT_CODE"
        log "systemd-cgtop output: $CGTOP_RAW"
        return
    elif [[ "$CGTOP_RAW" == *"Error"* ]]; then
        log "WARNING: systemd-cgtop returned an error"
        return
    elif [ -z "$CGTOP_RAW" ]; then
        log "WARNING: systemd-cgtop returned empty output"
        return
    fi
    
    # Extract only the second iteration which contains the usage data
    SECOND_ITERATION=$(echo "$CGTOP_RAW" | awk '
        BEGIN { found_first_iteration = 0; blank_line_found = 0; }
        /^$/ { 
            if (found_first_iteration == 1) {
                blank_line_found = 1;
            }
            next;
        }
        {
            if (blank_line_found == 1) {
                print;
            } else if ($0 ~ /^\//) {
                found_first_iteration = 1;
            }
        }
    ')
    
    # If we couldn't extract the second iteration, use whatever we have
    if [ -z "$SECOND_ITERATION" ]; then
        log "WARNING: Could not extract second iteration from systemd-cgtop output, using full output"
        SECOND_ITERATION="$CGTOP_RAW"
    fi
    
    MAX_LINES_TO_PROCESS=10
    
    TOTAL_LINES=$(echo "$SECOND_ITERATION" | wc -l)
    
    if [ "$TOTAL_LINES" -gt "$MAX_LINES_TO_PROCESS" ]; then
        log "Truncating cgtop output from $TOTAL_LINES to $MAX_LINES_TO_PROCESS lines before processing"
        SECOND_ITERATION=$(echo "$SECOND_ITERATION" | head -n 1; echo "$SECOND_ITERATION" | tail -n +2 | head -n "$MAX_LINES_TO_PROCESS")
    fi
        
        
    CGROUPS_JSON=$(echo "$SECOND_ITERATION" | awk -v total_memory="$TOTAL_MEMORY_KB" '
      # Skip empty lines
      /^$/ {next}
      # Process data lines
      {
        # Extract fields - systemd-cgtop output has these columns:
        # Control Group, Tasks, %CPU, Memory, Input/s, Output/s
        cgroup = $1;
        tasks = $2;
        cpu_pct = $3;
        memory = $4;
        input = $5;
        output = $6;
        
        # Calculate memory percentage if possible
        memory_pct = "-";
        if (memory ~ /[0-9.]+[MGT]/) {
            # Extract the numeric part and unit
            mem_value = memory;
            gsub(/[^0-9.]/, "", mem_value);
            mem_unit = memory;
            gsub(/[0-9.]/, "", mem_unit);
            
            # Convert to KB based on unit
            if (mem_unit == "G") {
                mem_kb = mem_value * 1024 * 1024;
            } else if (mem_unit == "M") {
                mem_kb = mem_value * 1024;
            } else {
                mem_kb = mem_value;
            }
            
            # Calculate percentage if total_memory is provided
            if (total_memory != "") {
                memory_pct = sprintf("%.1f", (mem_kb / total_memory) * 100);
            }
        }
        
        # Output in a format that can be converted to JSON
        printf "{\"cgroup\":\"%s\",\"tasks\":\"%s\",\"cpu_percent\":\"%s\",\"memory\":\"%s\",\"memory_percent\":\"%s%%\",\"input_per_sec\":\"%s\",\"output_per_sec\":\"%s\"},\n", 
               cgroup, tasks, cpu_pct, memory, memory_pct, input, output;
      }
    ' | sed '$s/,$//' | awk 'BEGIN{print "["} {print} END{print "]"}')
    
    CGTOP_DATA_JSON=$(jq -n \
                    --argjson cgroups "$CGROUPS_JSON" \
                    '{
                      cgroups: $cgroups
                    }')
    
    write_event_log "$taskname" "$CGTOP_DATA_JSON"
}

# Function to log container stats using crictl (containerd CLI)
log_crictl_stats() {
    local taskname="$1"  # Pass in the specific taskname
    local sort_by="$2"   # Pass in "cpu" or "memory" to determine sorting
    local num_cores="$3" # Pass in number of cores (needed for CPU percentage calculation)
    local total_memory="$4" # Pass in total memory in KB (needed for memory percentage calculation)
    
    # Check if crictl is available
    if ! command -v crictl >/dev/null 2>&1; then
        log "crictl not installed, cannot log container resource usage"
        return
    fi
    
    log "Running crictl stats for $sort_by..."
    
    local CMD_EXIT_CODE=0

    if [ "$sort_by" = "cpu" ]; then
        # sort by cpu usage
        CONTAINERS_JSON=$(crictl stats --seconds 3 --output json 2>/dev/null | \
            jq --argjson cores "$num_cores" \
            '.stats | 
            sort_by(-(if .cpu.usageNanoCores.value then (.cpu.usageNanoCores.value | tonumber // 0) else 0 end)) | 
            .[0:15] | 
            map({
                namespace: (.attributes.labels."io.kubernetes.pod.namespace" // "unknown"),
                pod: (.attributes.labels."io.kubernetes.pod.name" // "unknown"),
                container: (.attributes.metadata.name // "unknown"),
                restarts: (.attributes.annotations."io.kubernetes.container.restartCount" // "0"),
                cpu_mcore: (if .cpu.usageNanoCores.value then 
                    ((.cpu.usageNanoCores.value | tonumber) / 1000000 | floor | tostring) + "m" 
                else "0m" end),
                cpu_pct: (if .cpu.usageNanoCores.value then 
                    (((.cpu.usageNanoCores.value | tonumber) / 1000000 / (1000 * ($cores | tonumber))) * 100 | .*10 | floor/10 | tostring) + "%" 
                else "0.0%" end),
                throttled_periods: (.cpu.throttlingData.throttledPeriods // 0),
                total_periods: (.cpu.throttlingData.periods // 0),
                throttled_pct: (if (.cpu.throttlingData.periods // 0) > 0 then 
                    (((.cpu.throttlingData.throttledPeriods // 0) / (.cpu.throttlingData.periods // 0)) * 100 | .*10 | floor/10 | tostring) + "%" 
                else "0.0%" end),
                mem_mib: (if .memory.workingSetBytes.value then 
                    ((.memory.workingSetBytes.value | tonumber) / 1048576 | floor | tostring) + "Mi" 
                else "0Mi" end)
            })' 2>/dev/null)
    else
        # sort by memory usage
        CONTAINERS_JSON=$(crictl stats --seconds 3 --output json 2>/dev/null | \
            jq --argjson total_mem "${total_memory:-1}" \
            '.stats | 
            sort_by(-(if .memory.workingSetBytes.value then (.memory.workingSetBytes.value | tonumber // 0) else 0 end)) | 
            .[0:10] | 
            map({
                namespace: (.attributes.labels."io.kubernetes.pod.namespace" // "unknown"),
                pod: (.attributes.labels."io.kubernetes.pod.name" // "unknown"),
                container: (.attributes.metadata.name // "unknown"),
                restarts: (.attributes.annotations."io.kubernetes.container.restartCount" // "0"),
                cpu_mcore: (if .cpu.usageNanoCores.value then 
                    ((.cpu.usageNanoCores.value | tonumber) / 1000000 | floor | tostring) + "m" 
                else "0m" end),
                mem_mib: (if .memory.workingSetBytes.value then 
                    ((.memory.workingSetBytes.value | tonumber) / 1048576 | floor | tostring) + "Mi" 
                else "0Mi" end),
                mem_pct: (if .memory.workingSetBytes.value then 
                    (((.memory.workingSetBytes.value | tonumber) / 1024 / $total_mem) * 100 | .*10 | floor/10 | tostring) + "%" 
                else "0.0%" end)
            })' 2>/dev/null)
    fi
    
    CMD_EXIT_CODE=$?
    
    # Note: Exit code 141 is SIGPIPE which is expected when using pipelines, and not actually an error
    if [ $CMD_EXIT_CODE -ne 0 ] && [ $CMD_EXIT_CODE -ne 141 ]; then
        log "WARNING: Failed to collect or process container stats (error $CMD_EXIT_CODE)"
        return
    fi
    
    # Check if we have valid container data before proceeding
    if [ -z "$CONTAINERS_JSON" ] || [ "$CONTAINERS_JSON" = "[]" ] || [ "$CONTAINERS_JSON" = "null" ]; then
        log "WARNING: No valid container stats data found"
        return
    fi
    
    VALID_JSON=$(jq -n --argjson containers "$CONTAINERS_JSON" '{ containers: $containers }')
    
    write_chunked_event_log "$taskname" "$VALID_JSON" 10
}

# Function to run multiple pressure checks
run_pressure_checks() {
    local check_function="$1"  # Function to call for each check
    local consecutive_checks="$2"  # Number of checks to run
    local check_interval="$3"  # Interval between checks
    local resource_type="$4"  # "CPU" or "Memory" for logging
    
    local pressure_count=0
    log "Starting $resource_type pressure checks (will run $consecutive_checks checks)"

    for i in $(seq 1 "$consecutive_checks"); do
        log "Running check $i of $consecutive_checks"
        
        if ! $check_function; then
            pressure_count=$((pressure_count + 1))
            log "Check $i failed, pressure count now $pressure_count"
        else
            log "Check $i passed, no pressure detected"
        fi
        
        # If we've already detected pressure enough times, no need to continue
        if [ "$pressure_count" -ge "$((consecutive_checks / 2 + 1))" ]; then
            log "Detected pressure in majority of checks, breaking early"
            break
        fi
        
        # Wait before next check (except for the last iteration)
        if [ "$i" -lt "$consecutive_checks" ]; then
            log "Waiting $check_interval seconds before next check"
            sleep "$check_interval"
        fi
    done
    
    # Return pressure count for caller to handle - use return code instead of echo
    return $pressure_count
} 
