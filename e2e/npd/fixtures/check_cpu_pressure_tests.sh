#!/bin/bash
# Test script for CPU pressure detection
# Tests various scenarios including PSI metrics, load averages, CPU stats, and edge cases

# Source common functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common/test_common.sh"
source "$SCRIPT_DIR/common/event_log_validation.sh"

# Script under test
readonly SCRIPT_UNDER_TEST="/etc/node-problem-detector.d/plugin/check_cpu_pressure.sh"

# Helper function to run CPU pressure tests with default environment variables
run_cpu_test() {
    local test_name="$1"
    local expected_output="$2"
    local mock_proc_dir="${3:-$SCRIPT_DIR/testdata/mock-data/cpu-high-pressure/proc}"
    local mock_sys_dir="${4:-$SCRIPT_DIR/testdata/mock-data/cpu-high-pressure/sys}"
    local cpu_scenario="${5:-high}"
    local custom_env_vars="$6"
    
    # Default environment variables for CPU pressure tests
    local default_env_vars="-e CONSECUTIVE_CHECKS=1 -e CHECK_INTERVAL=1 -e PATH=/mock-commands/cpu:/mock-commands -e PSI_CPU_SOME_THRESHOLD=20 -e CPU_LOAD_THRESHOLD=0.8 -e CPU_IOWAIT_THRESHOLD=20 -e CPU_STEAL_THRESHOLD=10 -e PSI_IO_SOME_THRESHOLD=40 -e CPU_SCENARIO=$cpu_scenario -e IOTOP_SCENARIO=test-standard"
    
    # Combine default and custom env vars
    local all_env_vars="$default_env_vars"
    if [ -n "$custom_env_vars" ]; then
        all_env_vars="$all_env_vars $custom_env_vars"
    fi
    
    # Build volume mounts for CPU subdirectory pattern
    local volume_mounts=""
    
    # Mount general mock commands and cpu specific commands
    volume_mounts+="-v \"$SCRIPT_DIR/testdata/mock-commands:/mock-commands:ro\""
    
    if [ -n "$mock_proc_dir" ]; then
        volume_mounts+=" -v \"$mock_proc_dir:/mock-proc:ro\""
    fi
    if [ -n "$mock_sys_dir" ]; then
        volume_mounts+=" -v \"$mock_sys_dir:/mock-sys:ro\""
    fi
    
    # Call the new generic run_test function
    run_test "$SCRIPT_UNDER_TEST" "$test_name" "$expected_output" "$volume_mounts" "$all_env_vars"
}


start_fixture "check_cpu_pressure.sh Tests"

add_section "Main detection logic Tests"

# High CPU PSI pressure detection
run_cpu_test "High CPU PSI Pressure Detection" \
    "CPU pressure detected on node" \
    "$SCRIPT_DIR/testdata/mock-data/cpu-high-pressure/proc" \
    "$SCRIPT_DIR/testdata/mock-data/cpu-high-pressure/sys" \
    "high"

# Normal CPU conditions
run_cpu_test "Normal CPU Conditions" \
    "No CPU pressure detected on node" \
    "$SCRIPT_DIR/testdata/mock-data/cpu-normal/proc" \
    "$SCRIPT_DIR/testdata/mock-data/cpu-normal/sys" \
    "normal"

# High IO pressure detection
run_cpu_test "High IO Pressure Detection" \
    "CPU pressure detected on node" \
    "$SCRIPT_DIR/testdata/mock-data/cpu-normal/proc" \
    "$SCRIPT_DIR/testdata/mock-data/high-io-pressure/sys" \
    "normal"

# CPU throttling detection
run_cpu_test "CPU Throttling Detection" \
    "CPU pressure detected on node" \
    "$SCRIPT_DIR/testdata/mock-data/cpu-normal/proc" \
    "$SCRIPT_DIR/testdata/mock-data/cpu-throttling/sys" \
    "normal"

# High iowait detection
run_cpu_test "High IOWait Detection" \
    "CPU pressure detected on node" \
    "$SCRIPT_DIR/testdata/mock-data/high-iowait/proc" \
    "$SCRIPT_DIR/testdata/mock-data/cpu-normal/sys" \
    "high-iowait"

# High steal time detection
run_cpu_test "High Steal Time Detection" \
    "CPU pressure detected on node" \
    "$SCRIPT_DIR/testdata/mock-data/cpu-normal/proc" \
    "$SCRIPT_DIR/testdata/mock-data/cpu-normal/sys" \
    "high-steal"

# Custom threshold testing
run_cpu_test "Custom Low PSI Threshold" \
    "CPU pressure detected on node" \
    "$SCRIPT_DIR/testdata/mock-data/cpu-normal/proc" \
    "$SCRIPT_DIR/testdata/mock-data/cpu-normal/sys" \
    "normal" \
    "-e PSI_CPU_SOME_THRESHOLD=5"

# Custom high threshold (should not trigger)
run_cpu_test "Custom High PSI Threshold" \
    "No CPU pressure detected on node" \
    "$SCRIPT_DIR/testdata/mock-data/cpu-high-pressure/proc" \
    "$SCRIPT_DIR/testdata/mock-data/cpu-high-pressure/sys" \
    "high" \
    "-e PSI_CPU_SOME_THRESHOLD=90"

# ONLY PSI CPU pressure detection (isolated)
run_cpu_test "PSI CPU Pressure Only" \
    "CPU pressure detected on node" \
    "$SCRIPT_DIR/testdata/mock-data/psi-cpu-only/proc" \
    "$SCRIPT_DIR/testdata/mock-data/psi-cpu-only/sys" \
    "low-iowait" \
    "-e PSI_CPU_SOME_THRESHOLD=60"

# ONLY PSI IO pressure detection (isolated)
run_cpu_test "PSI IO Pressure Only" \
    "CPU pressure detected on node" \
    "$SCRIPT_DIR/testdata/mock-data/psi-io-only/proc" \
    "$SCRIPT_DIR/testdata/mock-data/psi-io-only/sys" \
    "low-iowait" \
    "-e PSI_IO_SOME_THRESHOLD=50"

# ONLY iowait pressure detection (isolated)
run_cpu_test "IOWait Pressure Only" \
    "CPU pressure detected on node" \
    "$SCRIPT_DIR/testdata/mock-data/iowait-only/proc" \
    "$SCRIPT_DIR/testdata/mock-data/iowait-only/sys" \
    "iowait-only" \
    "-e CPU_IOWAIT_THRESHOLD=30"

# No pressure anywhere (baseline)
run_cpu_test "No Pressure Baseline" \
    "No CPU pressure detected on node" \
    "$SCRIPT_DIR/testdata/mock-data/no-pressure/proc" \
    "$SCRIPT_DIR/testdata/mock-data/no-pressure/sys" \
    "all-low"

# Check if main detection logic tests failed - exit early if so
if [ "$FAILED" -gt 0 ]; then
    buffer_output ""
    buffer_output "${RED}Main detection logic tests failed. Exiting.${NC}"
    end_fixture "$(basename "$0")"
    exit 1
fi


# End fixture - this handles summary, results writing, and output flushing
end_fixture "$(basename "$0")"

# Exit with appropriate code
if [ "$FAILED" -eq 0 ]; then
    exit 0
else
    exit 1
fi