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

# Function to log IG top_process results in JSON format
log_ig_top_process_results() {
    local taskname="$1"  # Pass in the specific taskname (e.g., "npd:check_cpu_pressure_ig:top_process")
    local sort_by="$2"  # Pass in the sort flag ("cpu" or "memory")

    # Check if ig is available
    if ! command -v ig >/dev/null 2>&1; then
        log "ig not installed, cannot log process information via IG"
        return
    fi

    log "Running ig top_process command for $sort_by..."

    # Set max entries to a reasonable number of processes we want to log
    local max_entries=15
    log "Using $max_entries max entries for IG output"

    # Determine sort parameter based on sort_by
    local sort_param
    if [ "$sort_by" = "cpu" ]; then
        sort_param="-cpuUsageRelative"
    else
        sort_param="-memoryRelative"
    fi

    # Run ig command to get top process with the following parameters:
    # --sort: sort by CPU or memory
    # --max-entries: limit to a reasonable number of processes
    # --output jsonpretty: output in JSON format with pretty printing
    # --count 1: get a single snapshot. Note that ig will print two snapshots because ig takes a
    #     fast initial snapshot (which is not counted) and then a second one after the interval
    # --interval 5s: wait 5 seconds between snapshots to get more accurate values
    # --timeout 7: timeout the command after 7 seconds to force the ig process to exit (Otherwise it remains hanging indefinitely)
    #
    # Future improvements to ig could help here:
    # - ig should exit automatically after the count of snapshots is reached so that we don't need the --timeout parameter:
    #   https://github.com/inspektor-gadget/inspektor-gadget/issues/4926
    # - ig should have a parameter to avoid producing the fast initial snapshot to avoid the need for filtering it out below with awk:
    #   https://github.com/inspektor-gadget/inspektor-gadget/issues/4955
    local IG_RAW
    IG_RAW=$(timeout 10s ig run top_process \
        --sort "$sort_param" \
        --max-entries "$max_entries" \
        --output jsonpretty \
        --count 1 \
        --interval 5s \
        --timeout 7 2>&1)
    local IG_EXIT_CODE=$?

    # Redact sensitive information from ig output
    IG_RAW=$(redact_sensitive_data "$IG_RAW")

    if [ $IG_EXIT_CODE -ne 0 ]; then
        log "WARNING: ig command failed with exit code $IG_EXIT_CODE"
        log "ig output: $IG_RAW"
        return
    elif [ -z "$IG_RAW" ]; then
        log "WARNING: ig returned empty output"
        return
    fi

    # Check if output contains an error/warning logs (any text before the first '[' character).
    # If so, log it as a warning and continue processing the rest of the output
    local IG_LOGS
    IG_LOGS=$(printf '%s' "$IG_RAW" | sed '/^\[/q' | sed '/^\[/,$d')
    if [ -n "$IG_LOGS" ]; then
        log "WARNING: ig command returned a warning: $IG_LOGS"
    fi

    # Filter out error/warning logs and the first fast iteration (after 250ms) and keep only the second iteration which is after 5 seconds
    IG_RAW=$(printf '%s' "$IG_RAW" | awk 'BEGIN{scr=0} /^\[/ {scr++} scr==2')

    write_chunked_event_log "$taskname" "$IG_RAW"
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

# Function to check if CPU is under pressure
# Uses environment variables for thresholds (with defaults):
#   CPU_LOAD_THRESHOLD - Load threshold multiplier (default: 0.9 = 90% of cores)
#   PSI_CPU_SOME_THRESHOLD - PSI CPU some avg300 threshold (default: 60)
#   PSI_IO_SOME_THRESHOLD - PSI IO some avg300 threshold (default: 40)
#   CPU_IOWAIT_THRESHOLD - CPU iowait percentage threshold (default: 20)
#   CPU_STEAL_THRESHOLD - CPU steal percentage threshold (default: 10)
# Returns: 0 if no pressure, 1 if pressure detected
check_cpu_pressure() {
    # Get number of CPU cores
    local num_cores
    num_cores=$(nproc)
    
    # Configurable thresholds (can be overridden via environment variables)
    local cpu_load_threshold="${CPU_LOAD_THRESHOLD:-0.9}"      # 90% of available CPUs
    local psi_cpu_some_threshold="${PSI_CPU_SOME_THRESHOLD:-60}"  # 60% CPU stall over 5 minutes
    local psi_io_some_threshold="${PSI_IO_SOME_THRESHOLD:-40}"    # 40% IO stall over 5 minutes
    local cpu_iowait_threshold="${CPU_IOWAIT_THRESHOLD:-20}"      # 20% in iowait state
    local cpu_steal_threshold="${CPU_STEAL_THRESHOLD:-10}"        # 10% CPU steal time
    
    # Calculate load threshold based on number of cores
    local load_threshold
    load_threshold=$(echo "$num_cores * $cpu_load_threshold" | bc)
    
    log "Number of CPU cores: $num_cores"
    log "Load threshold: $load_threshold (${cpu_load_threshold} * $num_cores)"
    
    # Initialize pressure flag
    local pressure_detected=0
    
    # Initialize mpstat variables
    local user_pct=0
    local system_pct=0
    local iowait_pct=0
    local steal_pct=0
    local idle_pct=0
        
    # Check cgroup v2 PSI metrics if available
    if [ -f "/sys/fs/cgroup/cpu.pressure" ]; then
        local psi_cpu_some
        psi_cpu_some=$(awk '/some/ {print $4}' /sys/fs/cgroup/cpu.pressure | cut -d= -f2)
        
        log "PSI CPU some avg300: ${psi_cpu_some:-not available}"
        
        if [ -n "$psi_cpu_some" ] && [ "$(echo "$psi_cpu_some > $psi_cpu_some_threshold" | bc)" -eq 1 ]; then
            log "PRESSURE: High CPU pressure detected via cgroup PSI (avg300): ${psi_cpu_some}% (threshold: ${psi_cpu_some_threshold}%)"
            pressure_detected=1
        fi

        # Check for IO pressure
        if [ -f "/sys/fs/cgroup/io.pressure" ]; then
            local psi_io_some
            psi_io_some=$(awk '/some/ {print $4}' /sys/fs/cgroup/io.pressure | cut -d= -f2)
            
            log "PSI IO some avg300: ${psi_io_some:-not available}"
            
            if [ -n "$psi_io_some" ] && [ "$(echo "$psi_io_some > $psi_io_some_threshold" | bc)" -eq 1 ]; then
                log "PRESSURE: High IO pressure detected via cgroup PSI (avg300): ${psi_io_some}% (threshold: ${psi_io_some_threshold}%)"
                pressure_detected=1
            fi
        else
            log "sys/fs/cgroup/io.pressure not available"
        fi        
    else
        log "sys/fs/cgroup/cpu.pressure not available"
    fi

    # Check current load average (1 minute)
    local load_avg
    load_avg=$(awk '{print $1}' /proc/loadavg)
    log "/proc/loadavg: $load_avg"
    
    if [ -n "$load_threshold" ] && [ "$(echo "$load_avg > $load_threshold" | bc)" -eq 1 ]; then
        log "INFO: High CPU load: $load_avg"
        # pressure_detected=1 # Not setting pressure here as this may be too sensitive
    fi

    # Get CPU usage statistics
    if command -v mpstat >/dev/null 2>&1; then
        local mpstat_output
        mpstat_output=$(mpstat 1 1 | grep -v Linux | tail -n 1)
        
        if [ -n "$mpstat_output" ]; then
            # Format: CPU %usr %nice %sys %iowait %irq %soft %steal %guest %nice %idle
            user_pct=$(echo "$mpstat_output" | awk '{print $3}')
            system_pct=$(echo "$mpstat_output" | awk '{print $5}')
            iowait_pct=$(echo "$mpstat_output" | awk '{print $6}')
            steal_pct=$(echo "$mpstat_output" | awk '{print $9}')
            idle_pct=$(echo "$mpstat_output" | awk '{print $12}')
            
            log "CPU stats from mpstat: user: ${user_pct}% system: ${system_pct}% iowait: ${iowait_pct}% steal: ${steal_pct}% idle: ${idle_pct}%"
        else
            log "Failed to get CPU statistics from mpstat"
        fi
    else
        log "mpstat not available"
    fi    
    
    # Check for high iowait
    if [ "$(echo "$iowait_pct > $cpu_iowait_threshold" | bc)" -eq 1 ]; then
        log "INFO: High CPU iowait: ${iowait_pct}% (threshold: ${cpu_iowait_threshold}%)"
        pressure_detected=1
    fi
    
    # Check for high CPU steal
    if [ "$(echo "$steal_pct > $cpu_steal_threshold" | bc)" -eq 1 ]; then
        log "INFO: High CPU steal time: ${steal_pct}%"
        # pressure_detected=1 # Not setting pressure here as this may be too sensitive
    fi
    
    # Check for throttling events
    if [ -f "/sys/fs/cgroup/cpu.stat" ]; then
        local throttled_time
        throttled_time=$(grep 'throttled_usec' /sys/fs/cgroup/cpu.stat | awk '{print $2}')
        
        log "CPU throttled time: ${throttled_time:-0} us"
        
        if [ -n "$throttled_time" ] && [ "$throttled_time" -gt 0 ]; then
            log "PRESSURE: CPU throttling detected: $throttled_time us"
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

# Function to check if memory is under pressure
# Uses environment variables for thresholds (with defaults):
#   MEMORY_AVAILABLE_THRESHOLD - Percentage of total memory that should be available (default: 10)
#   PSI_MEMORY_SOME_THRESHOLD - PSI memory some avg60 threshold (default: 50)
# Sets global variable:
#   RECENT_OOM_KILLS - Recent OOM kill messages (for use by caller)
#   TOTAL_MEMORY_KB - Total memory in KB (for use by caller)
# Returns: 0 if no pressure, 1 if pressure detected
check_memory_pressure() {
    # Configurable thresholds (can be overridden via environment variables)
    local memory_available_threshold="${MEMORY_AVAILABLE_THRESHOLD:-10}"  # 10% of total memory
    local psi_memory_some_threshold="${PSI_MEMORY_SOME_THRESHOLD:-50}"    # 50% memory stall over 1 minute
    
    # Initialize pressure flag
    local pressure_detected=0
    
    # Reset global OOM messages variable
    RECENT_OOM_KILLS=""
    
    # Get total memory in kB
    local total_memory_kb
    total_memory_kb=$(grep MemTotal /proc/meminfo | awk '{print $2}')
    if [ -z "$total_memory_kb" ]; then
        log "Failed to determine total memory"
        return 2  # Return different code for unknown state
    fi
    
    # Export total memory for caller's use (e.g., for log_crictl_stats)
    TOTAL_MEMORY_KB="$total_memory_kb"
    
    # Convert to MB for easier reading
    local total_memory_mb=$((total_memory_kb / 1024))
    
    # Calculate memory threshold in kB
    local memory_available_threshold_kb=$((total_memory_kb * memory_available_threshold / 100))
    
    log "Total memory: $total_memory_mb MB"
    log "Memory available threshold: $memory_available_threshold% ($((memory_available_threshold_kb / 1024)) MB)"
    
    # Check cgroup v2 PSI metrics if available
    if [ -f "/sys/fs/cgroup/memory.pressure" ]; then
        local psi_memory_some
        psi_memory_some=$(awk '/some/ {print $3}' /sys/fs/cgroup/memory.pressure | cut -d= -f2)
        
        log "PSI memory some avg60: ${psi_memory_some:-not available}"
        
        if [ -n "$psi_memory_some" ] && [ "$(echo "$psi_memory_some > $psi_memory_some_threshold" | bc)" -eq 1 ]; then
            log "PRESSURE: High memory pressure detected via cgroup PSI: ${psi_memory_some}% (threshold: ${psi_memory_some_threshold}%)"
            pressure_detected=1
        fi
    else
        log "sys/fs/cgroup/memory.pressure not available"
    fi

    # Check available memory
    local memory_available_kb
    local memory_free_kb
    local memory_buffers_kb
    local memory_cached_kb
    
    memory_available_kb=$(grep MemAvailable /proc/meminfo | awk '{print $2}')
    memory_free_kb=$(grep MemFree /proc/meminfo | awk '{print $2}')
    memory_buffers_kb=$(grep Buffers /proc/meminfo | awk '{print $2}')
    memory_cached_kb=$(grep "^Cached:" /proc/meminfo | awk '{print $2}')
    
    # If MemAvailable is not available (older kernels), calculate it
    if [ -z "$memory_available_kb" ]; then
        memory_available_kb=$((memory_free_kb + memory_buffers_kb + memory_cached_kb))
    fi
    
    # Calculate memory usage percentage
    local memory_used_kb=$((total_memory_kb - memory_available_kb))
    local memory_used_pct=$((memory_used_kb * 100 / total_memory_kb))
    
    log "Memory stats from /proc/meminfo:"
    log "  Total: $((total_memory_kb / 1024)) MB"
    log "  Available: $((memory_available_kb / 1024)) MB"
    log "  Used: $((memory_used_kb / 1024)) MB ($memory_used_pct%)"
    
    if [ "$memory_available_kb" -lt "$memory_available_threshold_kb" ]; then
        log "PRESSURE: Low available memory: $((memory_available_kb / 1024)) MB (threshold: $((memory_available_threshold_kb / 1024)) MB)"
        pressure_detected=1
    fi

    # Check for recent OOM events
    if command -v dmesg >/dev/null 2>&1; then        
        log "Checking for OOM events in the last 5 minutes..."
        
        # Get system uptime in seconds
        local uptime_seconds
        uptime_seconds=$(awk '{print $1}' /proc/uptime | cut -d. -f1)
        local dmesg_cutoff=$((uptime_seconds - 300))  # 5 minutes ago in uptime seconds
        
        if [ "$dmesg_cutoff" -lt 0 ]; then
            dmesg_cutoff=0  # If system uptime is less than 5 minutes
        fi
        
        log "Current uptime: $uptime_seconds seconds, cutoff time: $dmesg_cutoff seconds"
                
        RECENT_OOM_KILLS=$(dmesg | awk -v cutoff="$dmesg_cutoff" '
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
