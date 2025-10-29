#!/bin/bash
# Test script for RX buffer error detection
# Tests various scenarios including high error rates, missing interfaces, command failures, and edge cases

# Source common functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common/test_common.sh"
source "$SCRIPT_DIR/common/event_log_validation.sh"

# Script under test
readonly SCRIPT_UNDER_TEST="/etc/node-problem-detector.d/plugin/check_rx_buffer_errors.sh"

# Helper function to build volume mounts for RX buffer tests
build_rx_volume_mounts() {
    local mock_state_template="$1"  # e.g., "rx-buffer-high-errors"
    local test_id="$2"              # unique identifier for this test (unused now)
    local volume_mounts=""
    
    # Mount general mock commands and rx-buffer specific commands
    volume_mounts+="-v $SCRIPT_DIR/testdata/mock-commands:/mock-commands:ro"
    
    # Mount state directory directly (read-write, like event logs)
    if [ -n "$mock_state_template" ]; then
        local template_dir="$SCRIPT_DIR/testdata/mock-data/$mock_state_template"
        
        # Mount the state directory directly to where the script expects it
        if [ -d "$template_dir/var/lib/node-problem-detector/rx-buffer-check" ]; then
            volume_mounts+=" -v $template_dir/var/lib/node-problem-detector/rx-buffer-check:/var/lib/node-problem-detector/rx-buffer-check"
        fi
    fi
    
    echo "$volume_mounts"
}


# Helper function to build environment variables for RX buffer tests
build_rx_env_vars() {
    local scenario="$1"
    local additional_vars="$2"
    # Put rx-buffer commands first, then general mock commands, then system paths
    local env_vars="-e RX_BUFFER_SCENARIO=$scenario -e PATH=/mock-commands/rx-buffer:/mock-commands:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
    
    if [ -n "$additional_vars" ]; then
        env_vars="$env_vars $additional_vars"
    fi
    
    echo "$env_vars"
}

start_fixture "check_rx_buffer_errors.sh Tests"

add_section "Basic RX Buffer Error Detection Tests"

# Test: No PCI interfaces found
run_test \
    "$SCRIPT_UNDER_TEST" \
    "No PCI Interfaces Found" \
    "No network interfaces found with required metrics" \
    "$(build_rx_volume_mounts "rx-buffer-baseline" "no-pci-$(date +%s%N)")" \
    "$(build_rx_env_vars "no-pci")"

# Test: First run baseline establishment (no state directory)
run_test \
    "$SCRIPT_UNDER_TEST" \
    "First Run Baseline" \
    "First run for eth0, saving baseline metrics" \
    "$(build_rx_volume_mounts "" "baseline-$(date +%s%N)")" \
    "$(build_rx_env_vars "normal")"

# Test: Normal operation with low error rates
run_test \
    "$SCRIPT_UNDER_TEST" \
    "Normal Operation Low Error Rate" \
    "No new packets received on eth0" \
    "$(build_rx_volume_mounts "rx-buffer-normal" "normal-$(date +%s%N)")" \
    "$(build_rx_env_vars "normal")"

# Test: High error rates detection
run_test \
    "$SCRIPT_UNDER_TEST" \
    "High Error Rate Detection" \
    "PROBLEM DETECTED" \
    "$(build_rx_volume_mounts "rx-buffer-high-errors" "high-errors-$(date +%s%N)")" \
    "$(build_rx_env_vars "high-errors")"

add_section "Error Handling and Edge Cases"

# Test: Missing rx_out_of_buffer metric
run_test \
    "$SCRIPT_UNDER_TEST" \
    "Missing Buffer Error Metric" \
    "rx_out_of_buffer' not found or empty" \
    "$(build_rx_volume_mounts "rx-buffer-baseline" "missing-metrics-$(date +%s%N)")" \
    "$(build_rx_env_vars "missing-metrics")"

# Test: Invalid/non-numeric data
run_test \
    "$SCRIPT_UNDER_TEST" \
    "Invalid Numeric Data" \
    "Invalid buffer error count for eth0" \
    "$(build_rx_volume_mounts "rx-buffer-baseline" "invalid-data-$(date +%s%N)")" \
    "$(build_rx_env_vars "invalid-data")"

# Test: ethtool command failure
run_test \
    "$SCRIPT_UNDER_TEST" \
    "ethtool Command Failure" \
    "Could not get ethtool stats" \
    "$(build_rx_volume_mounts "rx-buffer-baseline" "failure-$(date +%s%N)")" \
    "$(build_rx_env_vars "failure")"

# Test: No new packets (zero delta)
run_test \
    "$SCRIPT_UNDER_TEST" \
    "No New Packets Traffic" \
    "No new packets received on eth0" \
    "$(build_rx_volume_mounts "rx-buffer-no-traffic" "no-traffic-$(date +%s%N)")" \
    "$(build_rx_env_vars "no-traffic")"



end_fixture "$(basename "$0")"

# Exit with appropriate code
if [ "$FAILED" -eq 0 ]; then
    exit 0
else
    exit 1
fi