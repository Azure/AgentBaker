#!/bin/bash
# Test script for DNS issues detection integration
# Tests RBAC handling and event log creation scenarios

# Source common functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Go up one directory to fixtures to find common files
FIXTURES_DIR="$(dirname "$SCRIPT_DIR")"
source "$FIXTURES_DIR/common/test_common.sh"
source "$FIXTURES_DIR/common/event_log_validation.sh"

# Script under test
readonly SCRIPT_UNDER_TEST="/etc/node-problem-detector.d/plugin/check_dns_issues.sh"

# Function to run DNS RBAC validation tests
run_dns_rbac_test() {
    local test_name="$1"
    local dns_scenario="$2"
    local should_create_logs="$3"
    local test_description="$4"
    
    # Choose appropriate mock data based on scenario
    local mock_data_dir
    local env_vars
    if [ "$dns_scenario" = "unhealthy" ]; then
        mock_data_dir="dns-unhealthy-failure"
        env_vars="-e DNS_SCENARIO=unhealthy -e WGET_SCENARIO=unhealthy -e PATH=/mock-commands/dns:/mock-commands:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin -e SKIP_DNS_CHECK=false"
    elif [ "$dns_scenario" = "rbac-forbidden" ]; then
        mock_data_dir="dns-healthy-rbac"
        env_vars="-e DNS_SCENARIO=rbac-forbidden -e PATH=/mock-commands/dns:/mock-commands:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin -e SKIP_DNS_CHECK=false"
    else
        mock_data_dir="dns-healthy"
        env_vars="-e DNS_SCENARIO=healthy -e PATH=/mock-commands/dns:/mock-commands:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin -e SKIP_DNS_CHECK=false"
    fi
    
    local additional_volumes="-v $FIXTURES_DIR/testdata/mock-commands:/mock-commands:ro"
    
    # Mount state directory and kubeconfig
    additional_volumes="$additional_volumes -v $FIXTURES_DIR/testdata/mock-data/$mock_data_dir/var/run/npd/check_dns_issues_state:/var/run/npd/check_dns_issues_state"
    additional_volumes="$additional_volumes -v $FIXTURES_DIR/testdata/mock-data/$mock_data_dir/var/lib/kubelet/kubeconfig:/var/lib/kubelet/kubeconfig:ro"
    
    local container_id
    container_id=$(create_test_container "auto" \
        "$FIXTURES_DIR/testdata/mock-data/$mock_data_dir/proc" \
        "$FIXTURES_DIR/testdata/mock-data/$mock_data_dir/sys" \
        "$additional_volumes" \
        "$env_vars")
    
    if ! validate_container_id "$container_id" "$test_name"; then
        return 1
    fi
    
    # Execute the DNS issues script
    local script_output
    script_output=$(timeout 20s docker exec "$container_id" /entrypoint.sh "$SCRIPT_UNDER_TEST" 2>&1) || true
    local exit_code=$?
    
    # Validation results
    local validation_passed=true
    
    if [ "$should_create_logs" = "true" ]; then
        # Test case where DNS issues should create event logs
        local validation_errors=""
        
        # Validate event log files exist
        if ! validate_event_log_files_exist "$container_id" "npd:dns_checks:log"; then
            validation_passed=false
            validation_errors="${validation_errors}Failed to find expected DNS event log files\\n"
        fi
        
        # Validate JSON structure
        if ! validate_event_log_json_structure "$container_id" "npd:dns_checks:log"; then
            validation_passed=false
            validation_errors="${validation_errors}Invalid JSON structure in DNS event log\\n"
        fi
        
    else
        # Test case where DNS script should NOT create event logs (RBAC graceful handling)
        if ! validate_event_log_files_not_exist "$container_id" "npd:dns_checks:log"; then
            validation_passed=false
        fi
        
        # Should exit with code 0 (success)
        if [ $exit_code -ne 0 ]; then
            buffer_output "  Script exited with unexpected code: $exit_code (expected: 0)"
            validation_passed=false
        fi
    fi
    
    # Cleanup container
    cleanup_container "$container_id"
    
    # Final result
    if [ "$validation_passed" = true ]; then
        buffer_output "Testing: $test_name... ${GREEN}PASSED${NC}"
        ((PASSED++))
        return 0
    else
        buffer_output "Testing: $test_name... ${RED}FAILED${NC}"
        if [ "$should_create_logs" = "true" ]; then
            buffer_output "  Expected: Valid DNS event log creation"
            if [ -n "$validation_errors" ]; then
                buffer_output "  Validation errors:"
                echo -e "$validation_errors" | sed 's/^/    /' | while read -r line; do
                    [ -n "$line" ] && buffer_output "$line"
                done
            fi
        else
            buffer_output "  Expected: RBAC should fail gracefully (no event log created)"
        fi
        buffer_output "  Test description: $test_description"
        ((FAILED++))
        return 1
    fi
}

start_fixture "check_dns_issues.sh Integration Tests"

add_section "DNS RBAC Handling Tests"

# Test 1: RBAC forbidden - should NOT create event logs
run_dns_rbac_test \
    "API Server RBAC Forbidden - Shouldn't Create Log file" \
    "rbac-forbidden" \
    "false" \
    "RBAC failures should exit gracefully without creating event logs when no other DNS issues exist"

# Test 2: Actual DNS failure - SHOULD create event logs  
run_dns_rbac_test \
    "DNS Failure - Should Create Log files" \
    "unhealthy" \
    "true" \
    "Real DNS failures should create event logs for diagnostics"

# End fixture - this handles summary, results writing, and output flushing
end_fixture "$(basename "$0")"

# Exit with appropriate code
if [ "$FAILED" -eq 0 ]; then
    exit 0
else
    exit 1
fi