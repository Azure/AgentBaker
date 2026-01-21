#!/bin/bash
# Integration tests for Memory pressure detection
# Tests OOM event detection, JSON message validation, and event log creation

# Source common functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Go up one directory to fixtures to find common files
FIXTURES_DIR="$(dirname "$SCRIPT_DIR")"
source "$FIXTURES_DIR/common/test_common.sh"
source "$FIXTURES_DIR/common/event_log_validation.sh"

# Script under test
readonly SCRIPT_UNDER_TEST="/etc/node-problem-detector.d/plugin/check_memory_pressure.sh"

# Function to run memory pressure OOM validation tests
run_memory_oom_test() {
    local test_name="$1"
    local mock_dmesg="$2"
    local should_find_oom="$3"  # "true" or "false"
    local test_description="$4"
    
    # Use high-pressure mock data to ensure memory pressure is detected
    local mock_proc_dir="$FIXTURES_DIR/testdata/mock-data/memory-combined-pressure/proc"
    local mock_sys_dir="$FIXTURES_DIR/testdata/mock-data/memory-combined-pressure/sys"
    local memory_env_vars="-e CONSECUTIVE_CHECKS=1 -e PSI_MEMORY_SOME_THRESHOLD=10 -e MEMORY_AVAILABLE_THRESHOLD=10"
    
    # Build volume mounts for the generic run_test function
    local volume_mounts=""
    volume_mounts+="-v \"$mock_proc_dir:/mock-proc:ro\""
    volume_mounts+=" -v \"$mock_sys_dir:/mock-sys:ro\""
    volume_mounts+=" -v \"$mock_dmesg:/usr/bin/dmesg:ro\""
    
    # Run the test using the generic framework
    local exit_code=0
    local docker_cmd="timeout 20s docker run --rm --privileged"
    
    # Always mount entrypoint script
    if [ -n "$DOCKER_ENTRYPOINT" ]; then
        docker_cmd+=" -v \"$DOCKER_ENTRYPOINT:/entrypoint.sh:ro\""
    fi
    
    # Add all volume mounts
    docker_cmd+=" $volume_mounts"
    
    # Add environment variables  
    docker_cmd+=" $memory_env_vars"
    
    # Add image and script to run
    docker_cmd+=" --entrypoint /entrypoint.sh"
    docker_cmd+=" $DOCKER_IMAGE"
    docker_cmd+=" $SCRIPT_UNDER_TEST 2>&1"
    
    # Execute the test
    local script_output
    script_output=$(eval "$docker_cmd" 2>&1) || exit_code=$?
    
    # Check if memory pressure was detected (prerequisite for OOM logging)
    if [ $exit_code -ne 0 ] || ! echo "$script_output" | grep -q "Memory pressure detected on node"; then
        buffer_output "Testing: $test_name... ${RED}FAILED${NC}"
        buffer_output "  Memory pressure not detected - cannot test OOM functionality"
        buffer_output "  Exit code: $exit_code"
        ((FAILED++))
        return 1
    fi
    
    # For OOM tests, we need to create a persistent container to check event logs
    # This is a special case because we need to inspect the generated log files
    local container_id
    container_id=$(create_test_container "auto" "$mock_proc_dir" "$mock_sys_dir" "-v $mock_dmesg:/usr/bin/dmesg:ro" "$memory_env_vars")
    
    if ! validate_container_id "$container_id" "$test_name"; then
        return 1
    fi
    
    # Execute the memory pressure script in the persistent container
    docker exec "$container_id" /entrypoint.sh "$SCRIPT_UNDER_TEST" >/dev/null 2>&1
    
    # Validation results
    local validation_passed=true
    
    if [ "$should_find_oom" = "true" ]; then
        # Test case where OOM events should be found and logged
        
        # Check for OOM event log files
        if ! validate_event_log_files_exist "$container_id" "npd:check_memory_pressure:oom"; then
            validation_passed=false
            buffer_output "  Failed to find expected OOM event log files"
        fi
        
        # Validate OOM JSON structure in Message field
        if ! validate_memory_oom_message_content "$container_id"; then
            validation_passed=false
            buffer_output "  Invalid OOM JSON structure"
        fi
        
    else
        # Test case where no OOM events should be found
        
        # Check that no OOM event log was created
        local host_event_log_dir
        host_event_log_dir=$(get_event_log_host_dir "$container_id")
        
        local oom_files
        oom_files=$(find "$host_event_log_dir" -name "*.json" -exec grep -l "npd:check_memory_pressure:oom" {} \; 2>/dev/null || echo "")
        
        if [ -n "$oom_files" ]; then
            validation_passed=false
            buffer_output "  Unexpected OOM event log created when none should exist"
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
        buffer_output "  Test description: $test_description"
        ((FAILED++))
        return 1
    fi
}

# Function to validate OOM JSON structure in memory pressure logs
validate_memory_oom_message_content() {
    local container_id="$1"
    
    debug_output "  Validating OOM message content..."
    
    # Get host event log directory
    local host_event_log_dir
    host_event_log_dir=$(get_event_log_host_dir "$container_id")
    
    # Find the OOM event log file
    local oom_file
    oom_file=$(find "$host_event_log_dir" -name "*.json" -exec grep -l "npd:check_memory_pressure:oom" {} \; 2>/dev/null | head -1)
    
    if [ -z "$oom_file" ]; then
        buffer_output "  No OOM event log file found"
        return 1
    fi
    
    debug_output "    Found OOM event log file: $(basename "$oom_file")"
    
    # Extract the Message field (contains the OOM JSON)
    local message_content
    message_content=$(jq -r '.Message' "$oom_file" 2>/dev/null)
    
    if [ -z "$message_content" ] || [ "$message_content" = "null" ]; then
        buffer_output "  No message content found in OOM event log"
        return 1
    fi
    
    # Validate it's valid JSON
    if ! echo "$message_content" | jq . >/dev/null 2>&1; then
        buffer_output "    OOM message content is not valid JSON"
        return 1
    fi
    
    debug_output "    ✅ OOM message content is valid JSON"
    
    # Validate top-level structure
    local has_oom_events
    has_oom_events=$(echo "$message_content" | jq -e '.oom_events' >/dev/null 2>&1 && echo "true" || echo "false")
    
    if [ "$has_oom_events" = "true" ]; then
        debug_output "    ✅ Has required top-level field: oom_events"
        
        # Validate oom_events is an array
        local is_array
        is_array=$(echo "$message_content" | jq -e '.oom_events | type == "array"' 2>/dev/null)
        
        if [ "$is_array" = "true" ]; then
            debug_output "    ✅ oom_events is an array"
            
            # Check if array has elements and validate structure
            local array_length
            array_length=$(echo "$message_content" | jq '.oom_events | length')
            
            if [ "$array_length" -gt 0 ]; then
                debug_output "    ✅ Found $array_length OOM events"
                
                # Validate first event structure
                local first_event_has_message
                first_event_has_message=$(echo "$message_content" | jq -e '.oom_events[0].message' >/dev/null 2>&1 && echo "true" || echo "false")
                
                if [ "$first_event_has_message" = "true" ]; then
                    debug_output "    ✅ First OOM event has message field"
                    return 0
                else
                    buffer_output "    First OOM event missing message field"
                    return 1
                fi
            else
                debug_output "    ✅ Empty OOM events array (valid structure)"
                return 0
            fi
        else
            buffer_output "    oom_events is not an array"
            return 1
        fi
    else
        buffer_output "    Missing required top-level field: oom_events"
        return 1
    fi
}

start_fixture "check_memory_pressure.sh Integration Tests"

add_section "Memory OOM Event Detection Tests"

# OOM events with recent timestamps (should be detected)
run_memory_oom_test \
    "Recent OOM Events Detection" \
    "$FIXTURES_DIR/testdata/mock-commands/memory/dmesg-memory-oom-recent" \
    "true" \
    "Recent OOM kills should be detected and logged in JSON format"

# OOM events with old timestamps (should not be detected)
run_memory_oom_test \
    "Old OOM Events Filtering" \
    "$FIXTURES_DIR/testdata/mock-commands/memory/dmesg-memory-oom-old" \
    "false" \
    "Old OOM kills outside 5-minute window should be filtered out"

# No OOM events (should not create OOM log)
run_memory_oom_test \
    "No OOM Events Handling" \
    "$FIXTURES_DIR/testdata/mock-commands/memory/dmesg-memory-no-oom" \
    "false" \
    "No OOM events should result in no OOM event log creation"

add_section "Memory OOM Error Handling Tests"

# Malformed dmesg output
run_memory_oom_test \
    "Malformed dmesg Timestamps" \
    "$FIXTURES_DIR/testdata/mock-commands/memory/dmesg-memory-malformed" \
    "true" \
    "Malformed dmesg timestamps should be handled gracefully and still detect OOM events"

# End fixture - this handles summary, results writing, and output flushing
end_fixture "$(basename "$0")"

# Exit with appropriate code
if [ "$FAILED" -eq 0 ]; then
    exit 0
else
    exit 1
fi