#!/bin/bash
# Test script for pressure_common.sh functions
# Tests log_top_results, log_cgtop_results, log_crictl_stats and integration scenarios

# Source common functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Go up one directory to fixtures to find common files
FIXTURES_DIR="$(dirname "$SCRIPT_DIR")"
source "$FIXTURES_DIR/common/test_common.sh"
source "$FIXTURES_DIR/common/event_log_validation.sh"

# Create pressure common test data
"$FIXTURES_DIR/testdata/create_pressure_common_test_data.sh" >/dev/null 2>&1

# Helper function to run pressure_common.sh tests
run_pressure_common_test() {
    local test_name="$1"
    local expected_taskname="$2"
    local mock_command_path="$3"
    local command_name="$4"  # top, systemd-cgtop, crictl or ig
    local test_function="$5"  # Function to call in the container
    local expected_validation="$6"  # "should_succeed" or "should_fail"
    local custom_env_vars="$7"
    
    # Create test container with command-specific mounts
    local command_volume=""
    if [ -n "$mock_command_path" ] && [ -f "$mock_command_path" ]; then
        if [ "$command_name" = "crictl" ]; then
            command_volume="-v $mock_command_path:/usr/local/bin/crictl:ro"
        else
            command_volume="-v $mock_command_path:/usr/bin/$command_name:ro"
        fi
    fi
    
    # Build environment variables
    local pressure_env_vars="-e CONSECUTIVE_CHECKS=1 -e PSI_CPU_SOME_THRESHOLD=20"
    if [ -n "$custom_env_vars" ]; then
        pressure_env_vars="$pressure_env_vars $custom_env_vars"
    fi
    
    local container_id
    container_id=$(create_test_container "auto" "$FIXTURES_DIR/testdata/mock-data/cpu-high-pressure/proc" "$FIXTURES_DIR/testdata/mock-data/cpu-high-pressure/sys" "$command_volume" "$pressure_env_vars")
    
    if ! validate_container_id "$container_id" "$test_name"; then
        return 1
    fi
    
    # Execute the test function in container
    docker exec "$container_id" bash -c "
    source /etc/node-problem-detector.d/plugin/npd_common.sh
    source /etc/node-problem-detector.d/plugin/pressure_common.sh
    $test_function
    " >/dev/null 2>&1 || true
    
    # Validation based on expected outcome
    local validation_passed=true
    
    if [ "$expected_validation" = "should_succeed" ]; then
        # Test should succeed and create valid event logs
        
        # Validate event log files exist
        if ! validate_event_log_files_exist "$container_id" "$expected_taskname"; then
            validation_passed=false
        fi
        
        # Validate JSON structure
        if ! validate_event_log_json_structure "$container_id" "$expected_taskname"; then
            validation_passed=false
        fi
        
    elif [ "$expected_validation" = "should_fail" ]; then
        # Test should fail gracefully (no event log should be created)
        local log_files
        log_files=$(docker exec "$container_id" find "$EVENT_LOG_DIR" -name "*.json" -exec grep -l "$expected_taskname" {} \; 2>/dev/null || echo "")
        
        if [ -n "$log_files" ]; then
            validation_passed=false
        fi
    fi
    
    # Cleanup container
    cleanup_container "$container_id"
    
    # Final result
    if [ "$validation_passed" = true ]; then
        test_result "$test_name" "PASSED"
        return 0
    else
        test_result "$test_name" "FAILED"
        return 1
    fi
}

start_fixture "pressure_common.sh Tests"

add_section "Top Command Tests"

# Test 1: Normal top output for CPU sorting
run_pressure_common_test \
    "Top CPU Sorting Normal Output" \
    "test:top:cpu" \
    "$FIXTURES_DIR/testdata/mock-commands/cpu/top-test-cpu-normal" \
    "top" \
    'log_top_results "test:top:cpu" "cpu"' \
    "should_succeed"

# Test 2: Normal top output for memory sorting
run_pressure_common_test \
    "Top Memory Sorting Normal Output" \
    "test:top:memory" \
    "$FIXTURES_DIR/testdata/mock-commands/cpu/top-test-memory-normal" \
    "top" \
    'log_top_results "test:top:memory" "memory"' \
    "should_succeed"

# Test 3: Top output with many CPU cores (line limiting)
run_pressure_common_test \
    "Top Many CPU Cores Line Limiting" \
    "test:top:many-cores" \
    "$FIXTURES_DIR/testdata/mock-commands/cpu/top-test-many-cores" \
    "top" \
    'log_top_results "test:top:many-cores" "cpu"' \
    "should_succeed"

# Test 4: Top command timeout
run_pressure_common_test \
    "Top Command Timeout Handling" \
    "test:top:timeout" \
    "$FIXTURES_DIR/testdata/mock-commands/cpu/top-test-timeout" \
    "top" \
    'log_top_results "test:top:timeout" "cpu"' \
    "should_fail"

# Test 5: Top command permission denied
run_pressure_common_test \
    "Top Permission Denied Handling" \
    "test:top:permission" \
    "$FIXTURES_DIR/testdata/mock-commands/cpu/top-test-permission" \
    "top" \
    'log_top_results "test:top:permission" "cpu"' \
    "should_fail"

# Test 6: Missing top command
run_pressure_common_test \
    "Missing Top Command Handling" \
    "test:top:missing" \
    "" \
    "top" \
    'mv /usr/bin/top /usr/bin/top.bak 2>/dev/null || true; log_top_results "test:top:missing" "cpu"; mv /usr/bin/top.bak /usr/bin/top 2>/dev/null || true' \
    "should_fail"

add_section "Systemd-cgtop Command Tests"

# Test 7: Normal systemd-cgtop output for CPU
run_pressure_common_test \
    "Systemd-cgtop CPU Normal Output" \
    "test:cgtop:cpu" \
    "$FIXTURES_DIR/testdata/mock-commands/cpu/systemd-cgtop-test-cpu" \
    "systemd-cgtop" \
    'log_cgtop_results "test:cgtop:cpu" "-c"' \
    "should_succeed"

# Test 8: Normal systemd-cgtop output for memory
run_pressure_common_test \
    "Systemd-cgtop Memory Normal Output" \
    "test:cgtop:memory" \
    "$FIXTURES_DIR/testdata/mock-commands/cpu/systemd-cgtop-test-memory" \
    "systemd-cgtop" \
    'log_cgtop_results "test:cgtop:memory" "-m"' \
    "should_succeed"

# Test 9: Systemd-cgtop with second iteration extraction
run_pressure_common_test \
    "Systemd-cgtop Second Iteration Extraction" \
    "test:cgtop:second-iter" \
    "$FIXTURES_DIR/testdata/mock-commands/cpu/systemd-cgtop-test-second-iteration" \
    "systemd-cgtop" \
    'log_cgtop_results "test:cgtop:second-iter" "-c"' \
    "should_succeed"

# Test 10: Systemd-cgtop error handling
run_pressure_common_test \
    "Systemd-cgtop Error Handling" \
    "test:cgtop:error" \
    "$FIXTURES_DIR/testdata/mock-commands/cpu/systemd-cgtop-test-error" \
    "systemd-cgtop" \
    'log_cgtop_results "test:cgtop:error" "-c"' \
    "should_fail"

# Test 11: Missing systemd-cgtop command
run_pressure_common_test \
    "Missing Systemd-cgtop Command" \
    "test:cgtop:missing" \
    "" \
    "systemd-cgtop" \
    'mv /usr/bin/systemd-cgtop /usr/bin/systemd-cgtop.bak 2>/dev/null || true; log_cgtop_results "test:cgtop:missing" "-c"; mv /usr/bin/systemd-cgtop.bak /usr/bin/systemd-cgtop 2>/dev/null || true' \
    "should_fail"

add_section "Crictl Stats Tests"

# Test 12: Normal crictl stats for CPU
run_pressure_common_test \
    "Crictl Stats CPU Normal Output" \
    "test:crictl:cpu" \
    "$FIXTURES_DIR/testdata/mock-commands/cpu/crictl-test-cpu" \
    "crictl" \
    'log_crictl_stats "test:crictl:cpu" "cpu" "4" "8388608"' \
    "should_succeed"

# Test 13: Normal crictl stats for memory
run_pressure_common_test \
    "Crictl Stats Memory Normal Output" \
    "test:crictl:memory" \
    "$FIXTURES_DIR/testdata/mock-commands/cpu/crictl-test-memory" \
    "crictl" \
    'log_crictl_stats "test:crictl:memory" "memory" "4" "8388608"' \
    "should_succeed"

# Test 14: Crictl stats with throttling data
run_pressure_common_test \
    "Crictl Stats Throttling Data" \
    "test:crictl:throttling" \
    "$FIXTURES_DIR/testdata/mock-commands/cpu/crictl-test-throttling" \
    "crictl" \
    'log_crictl_stats "test:crictl:throttling" "cpu" "4" "8388608"' \
    "should_succeed"

# Test 15: Crictl stats with many containers (limiting)
run_pressure_common_test \
    "Crictl Stats Container Limiting" \
    "test:crictl:limiting" \
    "$FIXTURES_DIR/testdata/mock-commands/cpu/crictl-test-many-containers" \
    "crictl" \
    'log_crictl_stats "test:crictl:limiting" "cpu" "4" "8388608"' \
    "should_succeed"

# Test 16: Crictl stats error handling
run_pressure_common_test \
    "Crictl Stats Error Handling" \
    "test:crictl:error" \
    "$FIXTURES_DIR/testdata/mock-commands/cpu/crictl-test-error" \
    "crictl" \
    'log_crictl_stats "test:crictl:error" "cpu" "4" "8388608"' \
    "should_fail"

# Test 17: Crictl stats empty output
run_pressure_common_test \
    "Crictl Stats Empty Output" \
    "test:crictl:empty" \
    "$FIXTURES_DIR/testdata/mock-commands/cpu/crictl-test-empty" \
    "crictl" \
    'log_crictl_stats "test:crictl:empty" "cpu" "4" "8388608"' \
    "should_fail"

# Test 18: Missing crictl command
run_pressure_common_test \
    "Missing Crictl Command" \
    "test:crictl:missing" \
    "" \
    "crictl" \
    'mv /usr/local/bin/crictl /usr/local/bin/crictl.bak 2>/dev/null || true; log_crictl_stats "test:crictl:missing" "cpu" "4" "8388608"; mv /usr/local/bin/crictl.bak /usr/local/bin/crictl 2>/dev/null || true' \
    "should_fail"

add_section "Integration Tests"

# Test 19: All logging functions together
run_pressure_common_test \
    "All Logging Functions Integration" \
    "test:integration:all" \
    "$FIXTURES_DIR/testdata/mock-commands/integration-all-commands" \
    "all" \
    "log_top_results \"test:integration:all:top\" \"cpu\"
    log_cgtop_results \"test:integration:all:cgtop\" \"-c\"
    log_crictl_stats \"test:integration:all:crictl\" \"cpu\" \"4\" \"8388608\"" \
    "should_succeed"

# Test 20: Event log chunking integration
run_pressure_common_test \
    "Event Log Chunking Integration" \
    "test:integration:chunking" \
    "$FIXTURES_DIR/testdata/mock-commands/cpu/crictl-test-chunking" \
    "crictl" \
    'log_crictl_stats "test:integration:chunking" "cpu" "4" "8388608"' \
    "should_succeed"

# Test 21: Resource calculation integration
run_pressure_common_test \
    "Resource Calculation Integration" \
    "test:integration:resources" \
    "$FIXTURES_DIR/testdata/mock-commands/integration-resource-calc" \
    "all" \
    "NUM_CORES=\$(nproc)
    TOTAL_MEMORY_KB=\$(grep MemTotal /proc/meminfo | awk '{print \$2}')
    log_cgtop_results \"test:integration:resources:cgtop\" \"-c\"
    log_crictl_stats \"test:integration:resources:crictl\" \"cpu\" \"$NUM_CORES\" \"$TOTAL_MEMORY_KB\"" \
    "should_succeed" \
    "-e TEST_RESOURCE_CALC=true"

# End fixture - this handles summary, results writing, and output flushing
end_fixture "$(basename "$0")"

# Exit with appropriate code
if [ "$FAILED" -eq 0 ]; then
    exit 0
else
    exit 1
fi