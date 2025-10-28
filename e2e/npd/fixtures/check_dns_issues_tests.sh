#!/bin/bash
# Test script for DNS issues detection
# Tests various DNS scenarios including CoreDNS health checks, DNS resolution, and UDP errors

# Source common functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common/test_common.sh"
source "$SCRIPT_DIR/common/event_log_validation.sh"

# Script under test
readonly SCRIPT_UNDER_TEST="/etc/node-problem-detector.d/plugin/check_dns_issues.sh"

# Helper function to run DNS tests with default environment variables
run_dns_test() {
    local test_name="$1"
    local expected_output="$2"
    local mock_data_dir="${3:-$SCRIPT_DIR/testdata/mock-data/dns-healthy}"
    local dns_scenario="${4:-healthy}"
    local wget_scenario="${5:-no-leading-spaces}"
    local nslookup_scenario="${6:-success}"
    local custom_env_vars="$7"
    
    # Default environment variables for DNS tests
    local default_env_vars="-e DNS_SCENARIO=$dns_scenario -e WGET_SCENARIO=$wget_scenario -e NSLOOKUP_SCENARIO=$nslookup_scenario -e PATH=/mock-commands/dns:/mock-commands -e SKIP_DNS_CHECK=false -e FORCE_DIAGNOSTICS=false"
    
    # Combine default and custom env vars
    local all_env_vars="$default_env_vars"
    if [ -n "$custom_env_vars" ]; then
        all_env_vars="$all_env_vars $custom_env_vars"
    fi
    
    # Build volume mounts for DNS subdirectory pattern
    local volume_mounts=""
    
    # Mount general mock commands and dns specific commands
    volume_mounts+="-v \"$SCRIPT_DIR/testdata/mock-commands:/mock-commands:ro\""
    
    # Mount mock data (proc, var directories)
    if [ -n "$mock_data_dir" ]; then
        volume_mounts+=" -v \"$mock_data_dir/proc:/mock-proc:ro\""
        volume_mounts+=" -v \"$mock_data_dir/var:/mock-var:rw\""
        
        # Mount DNS state directory directly like RX buffer tests do
        if [ -d "$mock_data_dir/var/run/npd/check_dns_issues_state" ]; then
            volume_mounts+=" -v \"$mock_data_dir/var/run/npd/check_dns_issues_state:/var/run/npd/check_dns_issues_state\""
        fi
    fi
    
    # Call the generic run_test function
    run_test "$SCRIPT_UNDER_TEST" "$test_name" "$expected_output" "$volume_mounts" "$all_env_vars"
}

start_fixture "check_dns_issues.sh Tests"

add_section "CoreDNS Health Check Tests"

# CoreDNS health check with leading spaces
run_dns_test "CoreDNS Health Check - HTTP Response: Leading Spaces" \
    "All checked CoreDNS pods are healthy" \
    "$SCRIPT_DIR/testdata/mock-data/dns-leading-spaces-bug" \
    "healthy" \
    "leading-spaces" \
    "success" \
    "-e FORCE_DIAGNOSTICS=true"

# CoreDNS health check without leading spaces
run_dns_test "CoreDNS Health Check - HTTP Response: No Leading Spaces" \
    "All checked CoreDNS pods are healthy" \
    "$SCRIPT_DIR/testdata/mock-data/dns-no-leading-spaces" \
    "healthy" \
    "no-leading-spaces" \
    "success" \
    "-e FORCE_DIAGNOSTICS=true"

# CoreDNS health check with unhealthy response
run_dns_test "CoreDNS Health Check - Unhealthy Response" \
    "One or more CoreDNS pods are unhealthy" \
    "$SCRIPT_DIR/testdata/mock-data/dns-unhealthy" \
    "unhealthy" \
    "unhealthy" \
    "success" \
    "-e FORCE_DIAGNOSTICS=true"

# CoreDNS health check with HTTP/2 Response
run_dns_test "CoreDNS Health Check - HTTP/2 Response" \
    "One or more CoreDNS pods are unhealthy" \
    "$SCRIPT_DIR/testdata/mock-data/dns-leading-spaces-bug" \
    "healthy" \
    "http2-bug" \
    "success" \
    "-e FORCE_DIAGNOSTICS=true"

# CoreDNS health check with timeout
run_dns_test "CoreDNS Health Check - Timeout" \
    "One or more CoreDNS pods are unhealthy" \
    "$SCRIPT_DIR/testdata/mock-data/dns-healthy" \
    "healthy" \
    "timeout" \
    "success" \
    "-e FORCE_DIAGNOSTICS=true"

# CoreDNS health check with RBAC permission failure
run_dns_test "CoreDNS Health Check - RBAC Forbidden" \
    "kubectl RBAC permission limitation detected. Skipping CoreDNS health check." \
    "$SCRIPT_DIR/testdata/mock-data/dns-healthy" \
    "rbac-forbidden" \
    "no-leading-spaces" \
    "success" \
    "-e FORCE_DIAGNOSTICS=true"

add_section "DNS Resolution Tests"

# DNS resolution success
run_dns_test "DNS Resolution - Success" \
    "Successfully resolved external DNS name" \
    "$SCRIPT_DIR/testdata/mock-data/dns-healthy" \
    "healthy" \
    "no-leading-spaces" \
    "success"

# DNS resolution failure
run_dns_test "DNS Resolution - Failure" \
    "Failed to resolve external DNS name" \
    "$SCRIPT_DIR/testdata/mock-data/dns-healthy" \
    "healthy" \
    "no-leading-spaces" \
    "failure"

add_section "UDP Error Tests"

# UDP errors detected - using dedicated scenario
run_dns_test "UDP Errors - Detected" \
    "PROBLEM: UDP InErrors increased" \
    "$SCRIPT_DIR/testdata/mock-data/dns-udp-errors" \
    "unhealthy" \
    "no-leading-spaces" \
    "success" \
    "-e FORCE_DIAGNOSTICS=true"

# No UDP errors (baseline - no prior state)
run_dns_test "UDP Errors - None Detected (Baseline)" \
    "First run for UDP InErrors delta" \
    "$SCRIPT_DIR/testdata/mock-data/dns-udp-errors-baseline" \
    "unhealthy" \
    "no-leading-spaces" \
    "success"

add_section "Edge Cases Tests"

# Skip DNS check (IP address endpoint)
run_dns_test "Skip DNS Check - IP Address Endpoint" \
    "API Server endpoint is an IP address" \
    "$SCRIPT_DIR/testdata/mock-data/dns-ip-endpoint" \
    "ip-endpoint" \
    "no-leading-spaces" \
    "success"

# Force diagnostics mode
run_dns_test "Force Diagnostics Mode" \
    "All checked CoreDNS pods are healthy" \
    "$SCRIPT_DIR/testdata/mock-data/dns-healthy" \
    "healthy" \
    "no-leading-spaces" \
    "success" \
    "-e FORCE_DIAGNOSTICS=true"

# End fixture - this handles summary, results writing, and output flushing
end_fixture "$(basename "$0")"

# Exit with appropriate code
if [ "$FAILED" -eq 0 ]; then
    exit 0
else
    exit 1
fi