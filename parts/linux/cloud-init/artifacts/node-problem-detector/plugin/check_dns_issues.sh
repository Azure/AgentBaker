#!/bin/bash

# This script checks for various DNS related issues, including DNS resolution,
# CoreDNS pod health, and UDP errors. It logs diagnostics to the Extension's events directory.

set -o nounset
set -o pipefail

# Source common functions
SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"
# Attempt to source npd_common.sh, exit if it fails as it's required for logging
source "${SCRIPT_DIR}/npd_common.sh" || { echo "ERROR: Critical dependency npd_common.sh not found or failed to source." >&2; exit 0; }

# Exit codes (NPD scripts typically exit 0, problems are reported via logs)
OK=0
NOTOK=1 # Used internally for function return statuses

NSLOOKUP_TEST_TIMEOUT=5 # Seconds for nslookup timeout
COREDNS_HEALTH_TIMEOUT=10 # Seconds for querying CoreDNS health endpoint

# CoreDNS Check constants
COREDNS_NAMESPACE="kube-system"
COREDNS_LABEL="k8s-app=kube-dns" # Standard label for CoreDNS (or kube-dns)
COREDNS_HEALTH_PORT="8080"       
COREDNS_HEALTH_PATH="/health"    

# State directory for persisting data between script runs
STATE_DIR="/var/run/npd/check_dns_issues_state"
# Used for testing
FORCE_DIAGNOSTICS=${FORCE_DIAGNOSTICS:-false}
SKIP_DNS_CHECK=${SKIP_DNS_CHECK:-false}
# Kubeconfig file
KUBECONFIG_PATH="/var/lib/kubelet/kubeconfig"

# Function to discover CoreDNS IP address from iptables
get_coredns_ip() {
    log "Discovering CoreDNS IP address from iptables..." >&2
    
    local coredns_ip
    # Try to find CoreDNS IP from iptables NAT rules using the exact pattern
    # This is generally CoreDNS IP (10.0.0.10) but coud be node local dns cache ip
    # Looking for: -A KUBE-SERVICES -d 10.0.0.10/32 -p tcp -m comment --comment "kube-system/kube-dns:dns-tcp cluster IP"
    coredns_ip=$(iptables-save -t nat 2>/dev/null | \
        grep -m1 -- 'kube-system/kube-dns:dns-tcp cluster IP' | \
        sed -n 's/.*-d \([0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*\)\/32.*/\1/p')
    
    # Fallback: try without the specific port if not found
    if [[ -z "$coredns_ip" ]]; then
        coredns_ip=$(iptables-save -t nat 2>/dev/null | \
            grep -m1 -- 'kube-system/kube-dns' | \
            sed -n 's/.*-d \([0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*\)\/32.*/\1/p')
    fi
    
    if [[ -n "$coredns_ip" && "$coredns_ip" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        log "Discovered CoreDNS IP from iptables: $coredns_ip" >&2
        echo "$coredns_ip"
    else
        log "Could not discover CoreDNS IP from iptables, falling back to default: 10.0.0.10" >&2
        # Debug: show what iptables rules we found for troubleshooting
        log "Debug: Available iptables NAT rules with 'kube-system' or 'dns':" >&2
        iptables-save -t nat 2>/dev/null | grep -E 'kube-system|dns' | head -3 >&2 || true
        echo "10.0.0.10"
    fi
}

# Retrieve API Server FQDN from kubeconfig
APISERVER_FQDN=$(kubectl --kubeconfig "$KUBECONFIG_PATH" config view --minify -o jsonpath='{.clusters[0].cluster.server}' | cut -d/ -f3 | cut -d: -f1)

# Check if APISERVER_FQDN is an IP address or FQDN (unless overridden)
if [ "$SKIP_DNS_CHECK" != true ]; then
    if [[ "$APISERVER_FQDN" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]] || [[ "$APISERVER_FQDN" =~ ^([0-9a-fA-F]{0,4}:){1,7}[0-9a-fA-F]{0,4}$ ]]; then
        log "API Server endpoint is an IP address ($APISERVER_FQDN). Skipping DNS resolution check."
        SKIP_DNS_CHECK=true
    elif [ -n "${HTTP_PROXY:-}" ]; then
        log "HTTP_PROXY environment variable is set ($HTTP_PROXY). Skipping DNS resolution check in proxy environment."
        SKIP_DNS_CHECK=true
    else
        log "API Server endpoint is an FQDN ($APISERVER_FQDN). DNS resolution check will be performed."
    fi
else
    log "SKIP_DNS_CHECK override is set to true. DNS resolution check will be skipped."
fi


# Function to check external DNS resolution
check_external_dns() {    

    local coredns_ip
    coredns_ip=$(get_coredns_ip)
    
    log "Checking external DNS resolution for $APISERVER_FQDN using CoreDNS IP: $coredns_ip..."

    local nslookup_output
    local nslookup_exit_code

    # Call nslookup with local dns IP address configured for the pod overlay network.
    nslookup_output=$(nslookup -timeout="$NSLOOKUP_TEST_TIMEOUT" "$APISERVER_FQDN" "$coredns_ip" 2>&1)
    nslookup_exit_code=$?
    
    if [ $nslookup_exit_code -ne 0 ]; then
        log "ERROR: Failed to resolve external DNS name: $APISERVER_FQDN using CoreDNS IP: $coredns_ip. Exit code: $nslookup_exit_code. Output: $nslookup_output"
        return $NOTOK
    else
        log "Successfully resolved external DNS name: $APISERVER_FQDN using CoreDNS IP: $coredns_ip. Output: $nslookup_output"
        return $OK
    fi
}

# Function to check CoreDNS pod health
check_coredns_health() {
    log "Checking CoreDNS pod health..."

    # Check if kubectl is available
    if ! command -v kubectl >/dev/null 2>&1; then
        log "ERROR: kubectl command not found. Skipping CoreDNS health check."
        return $NOTOK # Cannot perform check
    fi

    # Check if Kubeconfig exists
    if [ ! -f "$KUBECONFIG_PATH" ]; then
        log "ERROR: Kubeconfig file not found at $KUBECONFIG_PATH. Skipping CoreDNS health check."
        return $NOTOK # Cannot perform check
    fi

    local pod_info_output
    pod_info_output=$(KUBECONFIG="$KUBECONFIG_PATH" kubectl get pods -n "$COREDNS_NAMESPACE" -l "$COREDNS_LABEL" -o jsonpath='{range .items[*]}{.metadata.name}:{.status.podIP}{"\n"}{end}' 2>&1)
    local kubectl_exit_code=$?

    if [ $kubectl_exit_code -ne 0 ]; then
        # Check if it's an RBAC permission error
        if echo "$pod_info_output" | grep -q "pods is forbidden"; then
            log "INFO: kubectl RBAC permission limitation detected. Skipping CoreDNS health check."
            return $OK  # Return OK for RBAC-only failures
        else
            log "ERROR: Failed to get CoreDNS pod info. kubectl exit code: $kubectl_exit_code. Output: $pod_info_output"
            return $NOTOK  # Return NOTOK for other failures
        fi
    fi

    if [ -z "$pod_info_output" ]; then
        log "WARNING: No CoreDNS pods found in namespace $COREDNS_NAMESPACE with label $COREDNS_LABEL."
        return $NOTOK # Considered a problematic state
    fi

    log "Found CoreDNS pods: $pod_info_output"
    
    local all_pods_healthy=true

    # Check if wget is available before looping through pods
    if ! command -v wget >/dev/null 2>&1; then
        log "ERROR: wget command not found. Skipping CoreDNS pod health checks."
        # Depending on desired behavior, could return $NOTOK or just log and proceed
        # For now, assume if wget is not there, we can't assess pod health, so it's a form of failure/unknown for this check part
        return $NOTOK 
    fi

    # Process each pod (format: "pod-name:pod-ip")
    while IFS= read -r pod_info; do
        [ -z "$pod_info" ] && continue  # Skip empty lines
        
        local pod_name="${pod_info%:*}"  # Extract pod name (everything before last colon)
        local pod_ip="${pod_info#*:}"    # Extract pod IP (everything after first colon)
        
        local health_url="http://${pod_ip}:${COREDNS_HEALTH_PORT}${COREDNS_HEALTH_PATH}"
        log "Checking CoreDNS pod $pod_name ($pod_ip) health at $health_url..."
        
        local wget_output
        local wget_exit_code
        local http_status_ok=false
        
        # -S / --server-response: print server response headers.
        # -q / --quiet: don't print progress information and ensures outputs to stdout 
        #        as there's a bug in wget where it doesn't output to stdout in some cases.
        # -O /dev/null: explicitly discard any downloaded content to avoid file creation.
        # --timeout: network timeout in seconds.
        # --no-check-certificate: for HTTPS (though URL is HTTP here).
        # 2>&1 redirects stderr (where -S prints headers) to stdout for capture.
        wget_output=$(wget -S -q -O /dev/null --timeout="$COREDNS_HEALTH_TIMEOUT" --no-check-certificate "$health_url" 2>&1)
        wget_exit_code=$?
        
        if [ $wget_exit_code -ne 0 ]; then
            if [ $wget_exit_code -eq 4 ]; then # Exit code 4 typically means network failure (includes timeouts)
                log "ERROR: Health check command failed for CoreDNS pod $pod_name ($pod_ip) with network failure (exit code 4 - likely timeout or host unreachable). Output: $wget_output"
            else
                log "ERROR: Health check command failed  for CoreDNS pod $pod_name ($pod_ip) with exit code $wget_exit_code. Output: $wget_output"
            fi
            all_pods_healthy=false
        else
            # Look for HTTP/x.x 200 anywhere in the output (not just at line start)
            # This handles the case where wget -S outputs "  HTTP/1.1 200 OK" with leading spaces
            if echo "$wget_output" | grep -q -E 'HTTP/[0-9]+\.[0-9]+\s+200(\s+OK)?'; then
                http_status_ok=true
            fi
            
            if [ "$http_status_ok" = true ]; then
                log "CoreDNS pod $pod_name ($pod_ip) is healthy (HTTP 200 OK)."
            else
                log "ERROR: CoreDNS pod $pod_name ($pod_ip) returned non-200 HTTP status or status unclear. wget headers: $wget_output"
                all_pods_healthy=false
            fi
        fi
    done <<< "$pod_info_output"

    if [ "$all_pods_healthy" = true ]; then
        log "All checked CoreDNS pods are healthy."
        return $OK
    else
        log "One or more CoreDNS pods are unhealthy."
        return $NOTOK
    fi
}

# Function to log UDP errors from /proc/net/snmp
# Returns $NOTOK if Udp InErrors has increased since the last check, otherwise $OK
check_udp_errors() {
    # If external DNS check failed, then we want to log full SNMP stats regardless of UDP errors
    local dns_check_failed=false
    if [ "$1" = true ]; then
        dns_check_failed=true
    fi

    local UDP_IN_ERRORS_STATE_FILE="${STATE_DIR}/udp_in_errors.state"

    # Ensure state directory exists
    if ! mkdir -p "$STATE_DIR"; then
        log "ERROR: Failed to create state directory $STATE_DIR. UDP error delta checking will be impaired."
        # Fallback to not reporting an error if state cannot be managed
        # but still attempt to log current stats if possible.
    fi

    log "Collecting UDP error statistics from /proc/net/snmp..."
    if [ ! -f /proc/net/snmp ]; then
        log "WARNING: /proc/net/snmp not found. Cannot log UDP errors or check delta."
        return $OK # Cannot determine error state
    fi

    # Read and log the entire /proc/net/snmp file for comprehensive visibility
    local full_snmp_stats
    full_snmp_stats=$(cat /proc/net/snmp 2>/dev/null)
    if [ -z "$full_snmp_stats" ]; then
        log "WARNING: Could not read /proc/net/snmp content."
        return $OK # Cannot determine error state
    fi
    
    # Extract the second line starting with Udp: which contains the values for error checking
    local udp_stats_line
    udp_stats_line=$(echo "$full_snmp_stats" | grep '^Udp:' | awk 'NR==2')

    if [ -z "$udp_stats_line" ]; then
        log "WARNING: Could not parse UDP statistics from /proc/net/snmp."
        return $OK # Cannot determine error state
    fi
    
    # Fields: Udp: InDatagrams NoPorts InErrors OutDatagrams RcvbufErrors SndbufErrors InCsumErrors IgnoredMulti MemErrors
    local current_in_errors
    current_in_errors=$(echo "$udp_stats_line" | awk '{val = int($4); print val}') 

    local udp_summary
    udp_summary=$(echo "$udp_stats_line" | awk '{printf "InDatagrams=%s NoPorts=%s InErrors=%s OutDatagrams=%s RcvbufErrors=%s SndbufErrors=%s InCsumErrors=%s IgnoredMulti=%s MemErrors=%s", $2, $3, $4, $5, $6, $7, $8, $9, $10}')

    log "UDP Raw Statistics Line: $udp_stats_line"
    log "UDP Parsed Statistics (Current Absolute): $udp_summary"
    
    # --- Delta Calculation Logic ---
    local prev_in_errors=0
    local prev_timestamp=0

    if [ -f "$UDP_IN_ERRORS_STATE_FILE" ]; then
        local prev_data
        prev_data=$(cat "$UDP_IN_ERRORS_STATE_FILE" 2>/dev/null)
        if [[ "$prev_data" =~ ^[0-9]+\ [0-9]+$ ]]; then # Basic validation for "timestamp errors_count"
            prev_timestamp=$(echo "$prev_data" | awk '{print $1}')
            prev_in_errors=$(echo "$prev_data" | awk '{print $2}')
            log "Previous UDP InErrors: $prev_in_errors (Timestamp: $prev_timestamp)"
        else
            log "WARNING: Could not parse previous UDP InErrors state from $UDP_IN_ERRORS_STATE_FILE. Content: '$prev_data'. Treating as first run for delta."
            prev_timestamp=0 # Mark to trigger first-run logic
        fi
    else
        log "No previous UDP InErrors state file found ($UDP_IN_ERRORS_STATE_FILE). This is the first run for delta calculation."
        prev_timestamp=0 # Mark as first run
    fi

    local current_timestamp
    current_timestamp=$(date +%s)
    if ! echo "$current_timestamp $current_in_errors" > "$UDP_IN_ERRORS_STATE_FILE"; then
        log "ERROR: Failed to write current UDP InErrors state to $UDP_IN_ERRORS_STATE_FILE. Delta checking may be inaccurate on next run."
        # Continue without returning an error due to state file write failure.
    fi

    if [ "$prev_timestamp" -eq 0 ]; then # First run (or state file was invalid/missing)
        log "First run for UDP InErrors delta. Current InErrors: $current_in_errors. Baseline saved."
        return $OK # No delta to compare yet
    else
        local delta_in_errors
        delta_in_errors=$((current_in_errors - prev_in_errors))
        
        log "UDP InErrors Delta: $delta_in_errors (Current: $current_in_errors, Previous: $prev_in_errors, Time Diff: $((current_timestamp - prev_timestamp))s)"

        if [ "$delta_in_errors" -gt 0 ]; then
            log "PROBLEM: UDP InErrors increased by $delta_in_errors since last check."
            # Write the full SNMP stats to event log for detailed analysis
            write_event_log "npd:dns_checks:snmp_stats" "$full_snmp_stats"
            return $NOTOK
        elif [ "$dns_check_failed" = true ] || [ "$FORCE_DIAGNOSTICS" = true ]; then
            # if external DNS check failed, then we want to log full SNMP stats regardless of UDP errors
            write_event_log "npd:dns_checks:snmp_stats" "$full_snmp_stats"
            return $OK
        else
            log "UDP InErrors have not increased since last check (Delta: $delta_in_errors)."
            return $OK
        fi
    fi
}

# Main logic

EXTERNAL_DNS_CHECK_FAILED=false
if [ "$SKIP_DNS_CHECK" == false ]; then
    check_external_dns
    EXTERNAL_DNS_RESULT=$?
    if [ $EXTERNAL_DNS_RESULT -ne $OK ]; then
        EXTERNAL_DNS_CHECK_FAILED=true
    fi
fi

UDP_ERRORS_FOUND=false
# Check for UDP errors. Passes result of DNS check so can log full SNMP stats if check fails
check_udp_errors "$EXTERNAL_DNS_CHECK_FAILED"
UDP_ERRORS_RESULT=$?
if [ $UDP_ERRORS_RESULT -ne $OK ]; then
    UDP_ERRORS_FOUND=true
fi

# Handle any failures from DNS and UDP Error checks
if [ "$EXTERNAL_DNS_CHECK_FAILED" = true ] || [ "$UDP_ERRORS_FOUND" = true ] || [ "$SKIP_DNS_CHECK" = true ] || [ "$FORCE_DIAGNOSTICS" = true ]; then
    check_coredns_health
    COREDNS_HEALTH_RESULT=$?
    # Log if CoreDNS health check failed or if UDP errors found
    if [ $COREDNS_HEALTH_RESULT -ne $OK ] || [ "$UDP_ERRORS_FOUND" = true ]; then
        write_logs "npd:dns_checks:log"
    fi
else
    log "External DNS check passed and no UDP errors found. Skipping CoreDNS health check."
fi

exit $OK # Always exit OK for NPD custom plugins that provide diagnostics 