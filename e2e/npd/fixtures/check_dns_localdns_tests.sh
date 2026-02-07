#!/bin/bash
# Test script for DNS LocalDNS script
# Tests check_dns_to_localdns.sh only

# Source common functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common/test_common.sh"
source "$SCRIPT_DIR/common/event_log_validation.sh"

# Script under test
readonly LOCALDNS_SCRIPT="/etc/node-problem-detector.d/plugin/check_dns_to_localdns.sh"

# Helper function to run LocalDNS tests
run_localdns_test() {
    local test_name="$1"
    local expected_output="$2"
    local mock_data_dir="${3:-$SCRIPT_DIR/testdata/mock-data/localdns-healthy}"
    local dig_scenario="${4:-success}"
    local systemctl_scenario="${5:-active}"
    local custom_env_vars="$6"
    local expected_exit_code="${7:-0}"
    
    # Default environment variables for LocalDNS tests
    local default_env_vars="-e DIG_SCENARIO=$dig_scenario -e SYSTEMCTL_SCENARIO=$systemctl_scenario -e FORCE_DIAGNOSTICS=false"
    default_env_vars+=" -e PATH=/mock-commands/dns-localdns:/mock-commands:/usr/bin:/bin"
    
    # Combine default and custom env vars
    local all_env_vars="$default_env_vars"
    if [ -n "$custom_env_vars" ]; then
        all_env_vars="$all_env_vars $custom_env_vars"
    fi
    
    # Build volume mounts
    local volume_mounts=""
    volume_mounts+="-v \"$SCRIPT_DIR/testdata/mock-commands:/mock-commands:ro\""
    
    if [ -n "$mock_data_dir" ] && [ -d "$mock_data_dir" ]; then
        # Mount mock data files to their expected system paths
        if [ -f "$mock_data_dir/opt/azure/containers/localdns/updated.localdns.corefile" ]; then
            volume_mounts+=" -v \"$mock_data_dir/opt/azure/containers/localdns/updated.localdns.corefile:/opt/azure/containers/localdns/updated.localdns.corefile:ro\""
        fi
        if [ -f "$mock_data_dir/etc/default/kubelet" ]; then
            volume_mounts+=" -v \"$mock_data_dir/etc/default/kubelet:/etc/default/kubelet:ro\""
        fi
        if [ -f "$mock_data_dir/run/systemd/resolve/resolv.conf" ]; then
            volume_mounts+=" -v \"$mock_data_dir/run/systemd/resolve/resolv.conf:/run/systemd/resolve/resolv.conf:ro\""
        fi
        if [ -f "$mock_data_dir/var/lib/kubelet/kubeconfig" ]; then
            volume_mounts+=" -v \"$mock_data_dir/var/lib/kubelet/kubeconfig:/var/lib/kubelet/kubeconfig:ro\""
        fi
        # Also mount the entire mock data directory for any other files
        volume_mounts+=" -v \"$mock_data_dir:/mock-data:ro\""
    fi
    
    # Call the generic run_test function with expected exit code
    run_test "$LOCALDNS_SCRIPT" "$test_name" "$expected_output" "$volume_mounts" "$all_env_vars" "30" "$expected_exit_code"
}

echo "=== Unit tests for check_dns_to_localdns.sh ==="

start_fixture "Unit tests for check_dns_to_localdns.sh"

# LocalDNS - healthy scenario
run_localdns_test "LocalDNS - Healthy Scenario" \
    "" \
    "$SCRIPT_DIR/testdata/mock-data/localdns-healthy" \
    "success" \
    "active" \
    "" \
    0

# LocalDNS - disabled via label
run_localdns_test "LocalDNS - Disabled Label" \
    "" \
    "$SCRIPT_DIR/testdata/mock-data/localdns-disabled" \
    "success" \
    "active" \
    "" \
    0

# LocalDNS - service inactive
run_localdns_test "LocalDNS - Service Inactive" \
    "localdns service is not running" \
    "$SCRIPT_DIR/testdata/mock-data/localdns-service-down" \
    "success" \
    "inactive" \
    "" \
    1

# LocalDNS - service failed
run_localdns_test "LocalDNS - Service Failed" \
    "localdns service is not running" \
    "$SCRIPT_DIR/testdata/mock-data/localdns-service-down" \
    "success" \
    "failed" \
    "" \
    1

# LocalDNS - service check timeout
run_localdns_test "LocalDNS - Service Check Timeout" \
    "systemctl command to check localdns service timed out" \
    "$SCRIPT_DIR/testdata/mock-data/localdns-healthy" \
    "success" \
    "timeout" \
    "" \
    2

# LocalDNS - config mismatch (enabled via label but no corefile)
run_localdns_test "LocalDNS - Config Mismatch" \
    "localdns corefile not found" \
    "$SCRIPT_DIR/testdata/mock-data/localdns-mismatch" \
    "success" \
    "active" \
    "" \
    1

# LocalDNS - no node labels available
run_localdns_test "LocalDNS - No Node Labels" \
    "" \
    "$SCRIPT_DIR/testdata/mock-data/localdns-no-labels" \
    "success" \
    "active" \
    "" \
    0

# LocalDNS - missing dig dependency
run_localdns_test "LocalDNS - Missing Dig Dependency" \
    "" \
    "$SCRIPT_DIR/testdata/mock-data/localdns-healthy" \
    "success" \
    "active" \
    "-e PATH=/mock-commands/dns-localdns-no-dig" \
    0

# LocalDNS - Successful DNS Resolution (Standard Success)
run_localdns_test "LocalDNS - Standard DNS Success" \
    "" \
    "$SCRIPT_DIR/testdata/mock-data/localdns-healthy" \
    "success" \
    "active" \
    "" \
    0

# LocalDNS - Successful DNS Resolution (Enhanced Validation)
run_localdns_test "LocalDNS - Enhanced DNS Success" \
    "" \
    "$SCRIPT_DIR/testdata/mock-data/localdns-healthy" \
    "success-enhanced" \
    "active" \
    "" \
    0

# LocalDNS - Successful DNS with Fast Response
run_localdns_test "LocalDNS - Fast DNS Response Success" \
    "" \
    "$SCRIPT_DIR/testdata/mock-data/localdns-healthy" \
    "success" \
    "active" \
    "" \
    0

end_fixture "$(basename "${BASH_SOURCE[0]}")"