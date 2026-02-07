#!/bin/bash

# This script checks if the node is in NotReady state and performs various network validation checks
# If Node NotReady is detected, it logs various networking diagnostics to the Extension's events directory
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
NOTOK=1   # Exit with NOTOK if node is NotReady. Only used for internal checks, script should always exit 0 to avoid triggering events

# Kubeconfig file
KUBECONFIG="/var/lib/kubelet/kubeconfig"

# Log collector constants
LOG_COLLECTOR_TIMESTAMP_FILE="/var/log/azure/aks/log_collector_last_run.timestamp"
LOG_COLLECTOR_SCRIPT="/opt/azure/containers/aks-log-collector.sh"

TEST_TIMEOUT="${TEST_TIMEOUT:-5}"
# Override for manual testing. If true, always run diagnostics and logging
FORCE_DIAGNOSTICS="${FORCE_DIAGNOSTICS:-false}"  
# Override for testing. If false, skip running the log collector
RUN_LOG_COLLECTOR="${RUN_LOG_COLLECTOR:-true}"

# Ensure the node name is in lowercase as this is how it's represented in k8s
NODE_NAME=$(hostname | tr '[:upper:]' '[:lower:]')
log "Node name: $NODE_NAME"

# Clean up old log files
cleanup_old_logs

# Function to check if node is in NotReady state
check_node_ready() {
    log "Checking if node is in Ready state..."
        
    # Check if kubectl is available
    if ! command -v kubectl >/dev/null 2>&1; then
        log "ERROR: kubectl not available"
        exit 0
    fi
    
    # Check if kubeconfig exists
    if [ ! -f "$KUBECONFIG" ]; then
        log "ERROR: Kubeconfig file not found at $KUBECONFIG"
        exit 0
    fi
    
    # Check node status using kubectl with kubeconfig - capture both stdout and stderr
    NODE_STATUS_OUTPUT=$(KUBECONFIG="$KUBECONFIG" kubectl get node "$NODE_NAME" -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>&1)
    KUBECTL_EXIT_CODE=$?
    
    if [ $KUBECTL_EXIT_CODE -ne 0 ]; then
        # Check if error indicates node not found
        if echo "$NODE_STATUS_OUTPUT" | grep -q "NotFound"; then
            log "ERROR: Node $NODE_NAME not found in the cluster."
            # Node not found in the cluster is not expected but shouldn't initiate logging
            return $OK
        else
            log "ERROR: Failed to get node status using kubectl. API Server may not be reachable."
            return $NOTOK
        fi
    fi
    
    # If we get here, the command succeeded, use the output directly
    NODE_STATUS="$NODE_STATUS_OUTPUT"
    
    if [ "$NODE_STATUS" != "True" ]; then
        log "Node is in Not Ready state"
        return $NOTOK
    else
        log "Node is in Ready state"
        return $OK
    fi
}

# Function to check if crictl pods command is working
check_crictl() {
    log "Checking crictl pods command..."
    
    # Check if crictl is available
    if ! command -v crictl >/dev/null 2>&1; then
        log "ERROR: crictl not available"
        return $OK  # Don't trigger logging if crictl is not available
    fi
    
    # Run crictl pods with 15 second timeout
    crictl -t 15s pods --latest >/dev/null 2>&1
    CRICTL_EXIT_CODE=$?
    
    if [ $CRICTL_EXIT_CODE -eq 124 ]; then
        log "ERROR: crictl pods command timed out after 15 seconds"
        return $NOTOK
    elif [ $CRICTL_EXIT_CODE -ne 0 ]; then
        log "ERROR: crictl pods command failed with exit code: $CRICTL_EXIT_CODE"
        return $NOTOK
    else
        log "crictl pods command succeeded"
        return $OK
    fi
}



# Function to call all network diagnostics
log_network_diagnostics() {
    log_route_info
    log_link_info
    log_network_connections
    log_conntrack_stats
}

# Function to log resource usage diagnostics
log_resource_diagnostics() {
    # Get number of CPU cores for percentage calculations
    NUM_CORES=$(nproc)
    log "Number of CPU cores: $NUM_CORES"
    
    # Get total memory for percentage calculations
    TOTAL_MEMORY_KB=""
    if [ -f /proc/meminfo ]; then
        TOTAL_MEMORY_KB=$(grep MemTotal /proc/meminfo | awk '{print $2}')
    else
        log "WARNING: /proc/meminfo not available, cannot determine total memory"
        TOTAL_MEMORY_KB=0
    fi
    
    # Log top results sorted by CPU usage
    log_top_results "npd:node_not_ready:top_cpu" "cpu"
    
    # Log top results sorted by memory usage
    log_top_results "npd:node_not_ready:top_memory" "memory"
    
    # Log systemd-cgtop results sorted by CPU
    log_cgtop_results "npd:node_not_ready:cgtop_cpu" "-c"
    
    # Log systemd-cgtop results sorted by memory
    log_cgtop_results "npd:node_not_ready:cgtop_memory" "-m"
    
    # Log container stats using crictl
    log_crictl_stats "npd:node_not_ready:crictl_cpu" "cpu" "$NUM_CORES" ""
    log_crictl_stats "npd:node_not_ready:crictl_memory" "memory" "$NUM_CORES" "$TOTAL_MEMORY_KB"
}

# Function to log route information
log_route_info() {
    log "Collecting IP route information..."
    ROUTE_INFO=$(ip route 2>&1)
    
    write_event_log "npd:node_not_ready:route_info" "$ROUTE_INFO"
}

# Function to log link information
log_link_info() {
    log "Collecting IP link information..."
    LINK_INFO=$(ip -s -s link show 2>&1)
    
    # Use the new chunking function to handle potentially large output
    write_chunked_event_log "npd:node_not_ready:link_stats" "$LINK_INFO"
}


# Function to log network connections
log_network_connections() {
    log "Collecting network socket information..."
    NETSTAT_INFO=$(ss -tunapl 2>&1)
    
    write_chunked_event_log "npd:node_not_ready:network_connections" "$NETSTAT_INFO"
}

# Function to log conntrack statistics
log_conntrack_stats() {
    log "Collecting conntrack statistics..."
    
    # Check if conntrack command is available
    if ! command -v conntrack >/dev/null 2>&1; then
        log "WARNING: conntrack command not available"
        return
    fi
    
    # Get conntrack statistics
    if ! CONNTRACK_STATS=$(conntrack -S 2>&1) || [ -z "$CONNTRACK_STATS" ]; then
        log "WARNING: Failed to get conntrack statistics"
        return
    fi
    
    # Log the conntrack statistics
    write_event_log "npd:node_not_ready:conntrack_stats" "$CONNTRACK_STATS"
}

# Function to check if log collector should be triggered
should_trigger_log_collector() {
    local current_time
    current_time=$(date +%s)
    local min_interval=$((60 * 60)) # 1 hour in seconds
    
    # Check if timestamp file exists
    if [ -f "$LOG_COLLECTOR_TIMESTAMP_FILE" ]; then
        local last_run
        last_run=$(cat "$LOG_COLLECTOR_TIMESTAMP_FILE" 2>/dev/null)
        # If file exists but can't be read, assume we should run
        if [ -z "$last_run" ]; then
            log "WARNING: Could not read timestamp file, will trigger log collector"
            return 0
        fi
        
        # Check if enough time has passed (at least 1 hour)
        local time_diff=$((current_time - last_run))
        if [ $time_diff -lt $min_interval ]; then
            log "Log collector ran $((time_diff / 60)) minutes ago (< $((min_interval / 60)) minutes). Skipping."
            return 1
        fi
    fi
    
    # If we get here, we should trigger the log collector
    return 0
}

# Function to trigger log collector
trigger_log_collector() {
    # Skip if RUN_LOG_COLLECTOR is set to false (for testing)
    if [ "$RUN_LOG_COLLECTOR" != "true" ]; then
        log "RUN_LOG_COLLECTOR is false, skipping log collector"
        return
    fi
    
    local current_time
    current_time=$(date +%s)
    
    # Ensure directory exists
    mkdir -p "$(dirname "$LOG_COLLECTOR_TIMESTAMP_FILE")" 2>/dev/null
    
    # Update timestamp file
    echo "$current_time" > "$LOG_COLLECTOR_TIMESTAMP_FILE"
    
    # Trigger log collector in background
    log "Triggering AKS log collector in background"
    nohup "$LOG_COLLECTOR_SCRIPT" > /dev/null 2>&1 &
}

# Main logic
log "Running checks..."

# Check if node is ready
check_node_ready
node_ready_result=$?

# Check if crictl pods command is working
check_crictl
crictl_result=$?

# Determine if logging should be triggered
if [ $node_ready_result -ne $OK ] || [ $crictl_result -ne $OK ] || [ "$FORCE_DIAGNOSTICS" = "true" ]; then
    log "Issues detected, triggering diagnostics..."
        
    log_network_diagnostics
    
    log_resource_diagnostics
    
    write_logs "npd:node_not_ready:log"
    
    # Trigger log collector for node logs Geneva Action if it's been more than 1 hour since the last run
    if should_trigger_log_collector; then
        trigger_log_collector
    fi

    exit $OK #always return OK as this monitor shouldn't raise any event
else
    log "Node is Ready"
    exit $OK
fi 