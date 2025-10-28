#!/bin/bash
# Test script for Memory pressure detection
# Tests various scenarios including PSI metrics, available memory, OOM events, and edge cases

# Source common functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common/test_common.sh"
source "$SCRIPT_DIR/common/event_log_validation.sh"

# Script under test
readonly SCRIPT_UNDER_TEST="/etc/node-problem-detector.d/plugin/check_memory_pressure.sh"


# Helper function to run memory pressure tests with default environment variables
run_memory_test() {
    local test_name="$1"
    local expected_output="$2"
    local mock_proc_dir="${3:-$SCRIPT_DIR/testdata/mock-data/memory-high-psi/proc}"
    local mock_sys_dir="${4:-$SCRIPT_DIR/testdata/mock-data/memory-high-psi/sys}"
    local memory_dmesg_scenario="${5:-no-oom}"
    local custom_env_vars="$6"
    
    # Default environment variables for memory pressure tests (matching production defaults)
    local default_env_vars="-e CONSECUTIVE_CHECKS=1 -e CHECK_INTERVAL=1 -e PATH=/mock-commands/memory:/mock-commands -e PSI_MEMORY_SOME_THRESHOLD=10 -e MEMORY_AVAILABLE_THRESHOLD=10 -e MEMORY_DMESG_SCENARIO=$memory_dmesg_scenario"
    
    # Combine default and custom env vars
    local all_env_vars="$default_env_vars"
    if [ -n "$custom_env_vars" ]; then
        all_env_vars="$all_env_vars $custom_env_vars"
    fi
    
    # Build volume mounts for memory subdirectory pattern
    local volume_mounts=""
    
    # Mount general mock commands and memory specific commands
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



start_fixture "check_memory_pressure.sh Tests"

add_section "Main Memory Detection Logic Tests"

# High memory PSI pressure detection
run_memory_test "High Memory PSI Pressure Detection" \
    "Memory pressure detected on node" \
    "$SCRIPT_DIR/testdata/mock-data/memory-high-psi/proc" \
    "$SCRIPT_DIR/testdata/mock-data/memory-high-psi/sys" \
    "memory-no-oom"

# Low available memory detection
run_memory_test "Low Available Memory Detection" \
    "Memory pressure detected on node" \
    "$SCRIPT_DIR/testdata/mock-data/memory-low-available/proc" \
    "$SCRIPT_DIR/testdata/mock-data/memory-low-available/sys" \
    "memory-no-oom"

# Combined memory pressure (PSI + low available)
run_memory_test "Combined Memory Pressure Detection" \
    "Memory pressure detected on node" \
    "$SCRIPT_DIR/testdata/mock-data/memory-combined-pressure/proc" \
    "$SCRIPT_DIR/testdata/mock-data/memory-combined-pressure/sys" \
    "with-oom"

# Normal memory conditions
run_memory_test "Normal Memory Conditions" \
    "No memory pressure detected on node" \
    "$SCRIPT_DIR/testdata/mock-data/memory-no-pressure/proc" \
    "$SCRIPT_DIR/testdata/mock-data/memory-no-pressure/sys" \
    "memory-no-oom"

# Legacy kernel support (no MemAvailable field)
run_memory_test "Legacy Kernel Memory Detection" \
    "No memory pressure detected on node" \
    "$SCRIPT_DIR/testdata/mock-data/memory-legacy-kernel/proc" \
    "$SCRIPT_DIR/testdata/mock-data/memory-legacy-kernel/sys" \
    "memory-no-oom"

# Custom threshold testing - low PSI threshold (should trigger)
run_memory_test "Custom Low PSI Threshold" \
    "Memory pressure detected on node" \
    "$SCRIPT_DIR/testdata/mock-data/memory-no-pressure/proc" \
    "$SCRIPT_DIR/testdata/mock-data/memory-no-pressure/sys" \
    "memory-no-oom" \
    "-e PSI_MEMORY_SOME_THRESHOLD=1"

# Custom threshold testing - high memory availability threshold (should trigger)
run_memory_test "Custom High Memory Threshold" \
    "Memory pressure detected on node" \
    "$SCRIPT_DIR/testdata/mock-data/memory-no-pressure/proc" \
    "$SCRIPT_DIR/testdata/mock-data/memory-no-pressure/sys" \
    "memory-no-oom" \
    "-e MEMORY_AVAILABLE_THRESHOLD=80"

# Custom threshold testing - high PSI threshold (should not trigger)
run_memory_test "Custom High PSI Threshold" \
    "No memory pressure detected on node" \
    "$SCRIPT_DIR/testdata/mock-data/memory-high-psi/proc" \
    "$SCRIPT_DIR/testdata/mock-data/memory-high-psi/sys" \
    "memory-no-oom" \
    "-e PSI_MEMORY_SOME_THRESHOLD=90"

# Check if main detection logic tests failed - exit early if so
if [ "$FAILED" -gt 0 ]; then
    buffer_output ""
    buffer_output "${RED}Main memory detection logic tests failed. Exiting.${NC}"
    end_fixture "$(basename "$0")"
    exit 1
fi


add_section "Memory Error Handling Tests"

# Missing memory.pressure file (use sys dir that doesn't contain memory.pressure)
run_memory_test "Missing Memory Pressure File" \
    "sys/fs/cgroup/memory.pressure not available" \
    "$SCRIPT_DIR/testdata/mock-data/memory-no-pressure/proc" \
    "$SCRIPT_DIR/testdata/mock-data/memory-missing-psi/sys" \
    "memory-no-oom"

# dmesg permission denied
run_memory_test "dmesg Permission Denied" \
    "No memory pressure detected on node" \
    "$SCRIPT_DIR/testdata/mock-data/memory-no-pressure/proc" \
    "$SCRIPT_DIR/testdata/mock-data/memory-no-pressure/sys" \
    "memory-permission-denied"

add_section "Memory Event Log Validation Tests"

# Memory pressure event log structure validation
run_memory_test "Memory Event Log Structure" \
    "Memory pressure detected on node" \
    "$SCRIPT_DIR/testdata/mock-data/memory-combined-pressure/proc" \
    "$SCRIPT_DIR/testdata/mock-data/memory-combined-pressure/sys" \
    "memory-oom-recent"

# End fixture - this handles summary, results writing, and output flushing
end_fixture "$(basename "$0")"

# Exit with appropriate code
if [ "$FAILED" -eq 0 ]; then
    exit 0
else
    exit 1
fi