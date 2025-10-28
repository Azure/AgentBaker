#!/bin/bash
# Test script for CPU pressure detection
# Tests various scenarios including PSI metrics, load averages, CPU stats, and edge cases

# Source common functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Go up one directory to fixtures to find common files
FIXTURES_DIR="$(dirname "$SCRIPT_DIR")"
source "$FIXTURES_DIR/common/test_common.sh"
source "$FIXTURES_DIR/common/event_log_validation.sh"

# Script under test
readonly SCRIPT_UNDER_TEST="/etc/node-problem-detector.d/plugin/check_cpu_pressure.sh"

# Function to run iotop message validation tests
run_iotop_message_test() {
    local test_name="$1"
    local iotop_mock="$2"
    local expected_summary="$3"
    local expected_process_count="$4"
    local max_command_length="$5"
    local test_description="$6"
    local should_succeed="$7"
    
    # Create test container with iotop command mounted
    local iotop_volume="-v $iotop_mock:/mock-commands/iotop:ro"
    local cpu_env_vars="-e CONSECUTIVE_CHECKS=1 -e CHECK_INTERVAL=1 -e PATH=/mock-commands/cpu:/mock-commands:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin -e PSI_CPU_SOME_THRESHOLD=20 -e MAX_COMMAND_LENGTH=$max_command_length"
    
    local container_id
    container_id=$(create_test_container "auto" "$FIXTURES_DIR/testdata/mock-data/cpu-high-pressure/proc" "$FIXTURES_DIR/testdata/mock-data/cpu-high-pressure/sys" "$iotop_volume" "$cpu_env_vars")
    
    if ! validate_container_id "$container_id" "$test_name"; then
        return 1
    fi
    
    # Execute the CPU pressure script
    local script_output
    script_output=$(timeout 20s docker exec "$container_id" /entrypoint.sh "$SCRIPT_UNDER_TEST" 2>&1) || true
    
    # Check if CPU pressure was detected (prerequisite for iotop execution)
    if ! echo "$script_output" | grep -q "CPU pressure detected on node"; then
        buffer_output "Testing: $test_name... ${RED}FAILED${NC}"
        cleanup_container "$container_id"
        ((FAILED++))
        return 1
    fi
    
    # Validation results
    local validation_passed=true
    
    if [ "$should_succeed" = "true" ]; then
        # Test case where iotop should succeed and create valid event logs
        local validation_errors=""
        
        # Run validations and check return codes (not output)
        # Validate event log files exist
        if ! validate_event_log_files_exist "$container_id" "npd:check_cpu_pressure:iotop"; then
            validation_passed=false
            validation_errors="${validation_errors}Failed to find expected iotop event log files\n"
        fi
        
        # Validate JSON structure  
        if ! validate_event_log_json_structure "$container_id" "npd:check_cpu_pressure:iotop"; then
            validation_passed=false
            validation_errors="${validation_errors}Invalid JSON structure in iotop event log\n"
        fi
        
        # Validate iotop-specific message content
        if ! validate_iotop_message_content "$container_id" "$expected_summary" "$expected_process_count" "$test_description"; then
            validation_passed=false
            validation_errors="${validation_errors}Iotop message content validation failed\n"
        fi
        
        # Validate command truncation if max length specified
        if [ "$max_command_length" -gt 0 ]; then
            if ! validate_iotop_command_truncation "$container_id" "$max_command_length"; then
                validation_passed=false
                validation_errors="${validation_errors}Command truncation validation failed\n"
            fi
        fi
        
        # Validate JSON escaping
        if ! validate_iotop_json_escaping "$container_id"; then
            validation_passed=false
            validation_errors="${validation_errors}JSON escaping validation failed\n"
        fi
        
    else
        # Test case where iotop should fail gracefully (no iotop event log should be created)
        
        # Get host event log directory
        local host_event_log_dir
        host_event_log_dir=$(get_event_log_host_dir "$container_id")
        
        # Check that no iotop event log was created
        local iotop_files
        iotop_files=$(find "$host_event_log_dir" -name "*.json" -exec grep -l "npd:check_cpu_pressure:iotop" {} \; 2>/dev/null || echo "")
        
        if [ -z "$iotop_files" ]; then
            # Good - no iotop event log created
            :
        else
            validation_passed=false
        fi
        
        # Check that other event logs were still created (top, cgtop, etc.)
        local other_files
        other_files=$(find "$host_event_log_dir" -name "*.json" -exec grep -l "npd:check_cpu_pressure" {} \; 2>/dev/null || echo "")
        
        if [ -z "$other_files" ]; then
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
        if [ "$should_succeed" = "true" ]; then
            buffer_output "  Expected: Valid iotop event log with summary: $expected_summary"
            buffer_output "  Expected: $expected_process_count processes in output"
            if [ -n "$validation_errors" ]; then
                buffer_output "  Validation errors:"
                echo -e "$validation_errors" | sed 's/^/    /' | while read -r line; do
                    [ -n "$line" ] && buffer_output "$line"
                done
            fi
        else
            buffer_output "  Expected: iotop should fail gracefully (no event log created)"
        fi
        buffer_output "  Test description: $test_description"
        ((FAILED++))
        return 1
    fi
}

# Function to validate iotop-specific JSON content in the Message field
validate_iotop_message_content() {
    local container_id="$1"
    local expected_summary="$2"  # JSON string with expected summary values
    local expected_process_count="$3"
    local test_description="$4"
    
    debug_output "  Validating iotop message content: $test_description"
    
    # Get host event log directory
    local host_event_log_dir
    host_event_log_dir=$(get_event_log_host_dir "$container_id")
    
    debug_output "    Event log directory: $host_event_log_dir"
    debug_output "    All files in directory:"
    for file in "$host_event_log_dir"/*; do
        if [ -f "$file" ]; then
            debug_output "      $(basename "$file")"
        fi
    done
    
    # Look specifically for iotop files by TaskName field
    local iotop_files=()
    for file in "$host_event_log_dir"/*.json; do
        if [ -f "$file" ]; then
            local task_name=$(jq -r '.TaskName // ""' "$file" 2>/dev/null)
            debug_output "      Checking file $(basename "$file"):"
            debug_output "        Full TaskName: '$task_name'"
            debug_output "        TaskName length: ${#task_name}"
            debug_output "        Contains 'iotop': $(echo "$task_name" | grep -q iotop && echo "yes" || echo "no")"
            case "$task_name" in *iotop*)
                iotop_files+=("$file")
                debug_output "        -> Added to iotop files"
            ;; esac
        fi
    done
    
    if [ ${#iotop_files[@]} -eq 0 ]; then
        # Show available files for diagnosis
        local available_files=$(find "$host_event_log_dir" -name "*.json" -exec sh -c 'echo "$(basename "$1"): $(jq -r .TaskName "$1" 2>/dev/null || echo "unknown")"' _ {} \; 2>/dev/null | head -5)
        buffer_output "  No iotop event log files found"
        buffer_output "  Available files: $available_files"
        return 1
    fi
    
    debug_output "    Found ${#iotop_files[@]} iotop event log file(s)"
    
    # Sort files and concatenate Message content from all chunks
    local message_content=""
    for file in $(printf '%s\n' "${iotop_files[@]}" | sort); do
        debug_output "      Processing $(basename "$file")"
        debug_output "        Message field exists: $(jq 'has("Message")' "$file" 2>/dev/null)"
        
        local chunk_content=$(jq -r '.Message' "$file" 2>/dev/null)
        debug_output "        Message length: ${#chunk_content}"
        debug_output "        Raw message content (first 200 chars): ${chunk_content:0:200}"
        debug_output "        Is valid JSON: $(echo "$chunk_content" | jq . >/dev/null 2>&1 && echo "yes" || echo "no")"
        
        message_content="${message_content}${chunk_content}"
    done
    
    debug_output "    Final concatenated length: ${#message_content}"
    debug_output "    Starts with: ${message_content:0:50}"
    debug_output "    Ends with: ${message_content: -50}"
    debug_output "    Contains '{': $(echo "$message_content" | grep -q '{' && echo "yes" || echo "no")"
    debug_output "    Contains '}': $(echo "$message_content" | grep -q '}' && echo "yes" || echo "no")"
    
    if [ -z "$message_content" ] || [ "$message_content" = "null" ]; then
        buffer_output "  No message content found in iotop event log"
        return 1
    fi
    
    # Validate concatenated content is valid JSON
    if ! echo "$message_content" | jq . >/dev/null 2>&1; then
        local jq_error=$(echo "$message_content" | jq . 2>&1 | head -1)
        buffer_output "    Concatenated content is not valid JSON"
        buffer_output "    JSON error: $jq_error"
        buffer_output "    Content preview: ${message_content:0:200}..."
        buffer_output "    Content length: ${#message_content} characters"
        return 1
    fi
    
    debug_output "    ✅ Message content is valid JSON"
    
    # Validate top-level structure
    local has_summary
    local has_processes
    has_summary=$(echo "$message_content" | jq -e '.summary' >/dev/null 2>&1 && echo "true" || echo "false")
    has_processes=$(echo "$message_content" | jq -e '.processes' >/dev/null 2>&1 && echo "true" || echo "false")
    
    if [ "$has_summary" = "true" ] && [ "$has_processes" = "true" ]; then
        debug_output "    ✅ Has required top-level fields: summary, processes"
    else
        buffer_output "    Missing required top-level fields (summary: $has_summary, processes: $has_processes)"
        return 1
    fi
    
    # Validate summary fields match expected values (if provided)
    if [ -n "$expected_summary" ] && [ "$expected_summary" != "null" ]; then
        debug_output "    Validating summary field values..."
        
        local actual_total_read
        local actual_total_write
        local actual_current_read
        local actual_current_write
        actual_total_read=$(echo "$message_content" | jq -r '.summary.total_disk_read')
        actual_total_write=$(echo "$message_content" | jq -r '.summary.total_disk_write')
        actual_current_read=$(echo "$message_content" | jq -r '.summary.current_disk_read')
        actual_current_write=$(echo "$message_content" | jq -r '.summary.current_disk_write')
        
        local expected_total_read
        local expected_total_write
        local expected_current_read
        local expected_current_write
        expected_total_read=$(echo "$expected_summary" | jq -r '.total_disk_read')
        expected_total_write=$(echo "$expected_summary" | jq -r '.total_disk_write')
        expected_current_read=$(echo "$expected_summary" | jq -r '.current_disk_read')
        expected_current_write=$(echo "$expected_summary" | jq -r '.current_disk_write')
        
        if [ "$actual_total_read" = "$expected_total_read" ] && 
           [ "$actual_total_write" = "$expected_total_write" ] && 
           [ "$actual_current_read" = "$expected_current_read" ] && 
           [ "$actual_current_write" = "$expected_current_write" ]; then
            debug_output "      ✅ Summary fields match expected values"
            debug_output "        total_disk_read: $actual_total_read"
            debug_output "        total_disk_write: $actual_total_write"
            debug_output "        current_disk_read: $actual_current_read"
            debug_output "        current_disk_write: $actual_current_write"
        else
            buffer_output "      Summary fields don't match expected values"
            buffer_output "        Expected: total_read=$expected_total_read, total_write=$expected_total_write"
            buffer_output "        Actual:   total_read=$actual_total_read, total_write=$actual_total_write"
            buffer_output "        Expected: current_read=$expected_current_read, current_write=$expected_current_write"
            buffer_output "        Actual:   current_read=$actual_current_read, current_write=$actual_current_write"
            return 1
        fi
    else
        debug_output "    Skipping summary field validation (no expected values provided)"
    fi
    
    # Validate processes array
    local actual_process_count
    actual_process_count=$(echo "$message_content" | jq '.processes | length')
    
    if [ "$actual_process_count" -eq "$expected_process_count" ]; then
        debug_output "    ✅ Process count matches expected: $actual_process_count"
    else
        buffer_output "    Process count mismatch: expected $expected_process_count, got $actual_process_count"
        return 1
    fi
    
    # Validate first process structure (if processes exist)
    if [ "$actual_process_count" -gt 0 ]; then
        debug_output "    Validating process structure..."
        local process_fields=("pid" "prio" "user" "disk_read" "disk_write" "command")
        local all_fields_present=true
        
        for field in "${process_fields[@]}"; do
            local field_present
            field_present=$(echo "$message_content" | jq -e ".processes[0].$field" >/dev/null 2>&1 && echo "true" || echo "false")
            if [ "$field_present" = "false" ]; then
                buffer_output "      Process field '$field' missing"
                all_fields_present=false
            fi
        done
        
        if [ "$all_fields_present" = "true" ]; then
            debug_output "      ✅ All process fields present: ${process_fields[*]}"
            
            # Show first process as example
            if [ "$DEBUG_MODE" = "true" ]; then
                local first_process
                first_process=$(echo "$message_content" | jq -c '.processes[0]')
                debug_output "        Example process: $first_process"
            fi
        else
            return 1
        fi
    fi
    
    return 0
}

# Function to validate command truncation in iotop output
validate_iotop_command_truncation() {
    local container_id="$1"
    local max_command_length="$2"
    
    debug_output "  Validating command truncation (max length: $max_command_length)..."
    
    # Get host event log directory
    local host_event_log_dir
    host_event_log_dir=$(get_event_log_host_dir "$container_id")
    
    # Look for iotop files by TaskName field and concatenate Message content
    local iotop_files=()
    for file in "$host_event_log_dir"/*.json; do
        if [ -f "$file" ]; then
            local task_name=$(jq -r '.TaskName // ""' "$file" 2>/dev/null)
            case "$task_name" in *iotop*)
                iotop_files+=("$file")
            ;; esac
        fi
    done
    
    if [ ${#iotop_files[@]} -eq 0 ]; then
        buffer_output "  No iotop event log files found"
        return 1
    fi
    
    # Concatenate Message content from all chunks
    local message_content=""
    for file in $(printf '%s\n' "${iotop_files[@]}" | sort); do
        local chunk_content=$(jq -r '.Message' "$file" 2>/dev/null)
        message_content="${message_content}${chunk_content}"
    done
    
    local process_count
    process_count=$(echo "$message_content" | jq '.processes | length')
    
    if [ "$process_count" -gt 0 ]; then
        # Check each process command length
        local validation_passed=true
        
        for ((i=0; i<process_count; i++)); do
            local command
            command=$(echo "$message_content" | jq -r ".processes[$i].command")
            local command_length=${#command}
            
            if [ "$command_length" -le "$max_command_length" ]; then
                debug_output "      ✅ Process $i command length OK: $command_length chars"
            else
                buffer_output "    Process $i command too long: $command_length chars (max: $max_command_length)"
                validation_passed=false
            fi
            
            # Check if long commands end with "..."
            if [ "$command_length" -eq "$max_command_length" ]; then
                case "$command" in *"..."*) debug_output "      ✅ Long command properly truncated with ellipsis" ;; esac
            elif [ "$command_length" -gt "$((max_command_length - 10))" ]; then
                case "$command" in *"..."*) : ;; *) debug_output "      Long command may not be properly truncated: $command" ;; esac
            fi
        done
        
        if [ "$validation_passed" = true ]; then
            debug_output "    ✅ All command lengths within limits"
            return 0
        else
            buffer_output "    Some commands exceed length limits"
            return 1
        fi
    else
        debug_output "    ✅ No processes to validate (empty process list)"
        return 0
    fi
}

# Function to validate special character escaping in JSON
validate_iotop_json_escaping() {
    local container_id="$1"
    
    debug_output "  Validating JSON special character escaping..."
    
    # Get host event log directory
    local host_event_log_dir
    host_event_log_dir=$(get_event_log_host_dir "$container_id")
    
    # Look for iotop files by TaskName field and concatenate Message content
    local iotop_files=()
    for file in "$host_event_log_dir"/*.json; do
        if [ -f "$file" ]; then
            local task_name=$(jq -r '.TaskName // ""' "$file" 2>/dev/null)
            case "$task_name" in *iotop*)
                iotop_files+=("$file")
            ;; esac
        fi
    done
    
    if [ ${#iotop_files[@]} -eq 0 ]; then
        buffer_output "  No iotop event log files found"
        return 1
    fi
    
    # Concatenate Message content from all chunks
    local message_content=""
    for file in $(printf '%s\n' "${iotop_files[@]}" | sort); do
        local chunk_content=$(jq -r '.Message' "$file" 2>/dev/null)
        message_content="${message_content}${chunk_content}"
    done
    
    # Test that the JSON can be parsed (which means escaping worked)
    if echo "$message_content" | jq . >/dev/null 2>&1; then
        debug_output "    ✅ JSON parsing successful - special characters properly escaped"
            
        # Look for common special characters that should be escaped
        if [ "$DEBUG_MODE" = "true" ]; then
            local process_count
            process_count=$(echo "$message_content" | jq '.processes | length')
            
            if [ "$process_count" -gt 0 ]; then
                # Check if any commands contain quotes, spaces, or other special chars
                local has_special_chars=false
                
                for ((i=0; i<process_count; i++)); do
                    local command
                    command=$(echo "$message_content" | jq -r ".processes[$i].command")
                    
                    case "$command" in *" "*|*"\""*|*"\\"*)
                        has_special_chars=true
                        debug_output "        ✅ Found and properly handled special characters in: $command"
                        ;;
                    esac
                done
                
                if [ "$has_special_chars" = false ]; then
                    debug_output "        No special characters found in this test case"
                fi
            fi
        fi
        
        return 0
    else
        buffer_output "    JSON parsing failed - special characters not properly escaped"
        return 1
    fi
}

start_fixture "check_cpu_pressure.sh Integration Tests"

add_section "iotop Message Content Validation Tests"

# Create test data
"$FIXTURES_DIR/testdata/create_iotop_test_data.sh"

# Test 1: Standard iotop output with exact value validation
expected_summary_1='{"total_disk_read":"15.67 B/s","total_disk_write":"8.90 K/s","current_disk_read":"5.23 M/s","current_disk_write":"3.45 G/s"}'
run_iotop_message_test \
    "Standard iotop Output Validation" \
    "$FIXTURES_DIR/testdata/mock-commands/cpu/iotop-test-standard" \
    "$expected_summary_1" \
    3 \
    30 \
    "standard iotop output with known summary values and 3 processes" \
    "true"

# Test 2: Special characters handling and JSON escaping
expected_summary_2='{"total_disk_read":"2.34 K/s","total_disk_write":"1.56 K/s","current_disk_read":"1.12 K/s","current_disk_write":"0.78 K/s"}'
run_iotop_message_test \
    "Special Characters JSON Escaping" \
    "$FIXTURES_DIR/testdata/mock-commands/cpu/iotop-test-special-chars" \
    "$expected_summary_2" \
    3 \
    30 \
    "special characters in commands properly escaped in JSON" \
    "true"

# Test 3: Command truncation functionality
expected_summary_3='{"total_disk_read":"1.23 K/s","total_disk_write":"0.89 K/s","current_disk_read":"0.67 K/s","current_disk_write":"0.45 K/s"}'
run_iotop_message_test \
    "Long Command Truncation" \
    "$FIXTURES_DIR/testdata/mock-commands/cpu/iotop-test-long-commands" \
    "$expected_summary_3" \
    1 \
    30 \
    "very long commands properly truncated to max length with ellipsis" \
    "true"

# Test 4: Mixed data units handling (B/s, K/s, M/s, G/s)
expected_summary_4='{"total_disk_read":"1.25 G/s","total_disk_write":"856.34 M/s","current_disk_read":"45.67 M/s","current_disk_write":"23.45 M/s"}'
run_iotop_message_test \
    "Mixed Data Units Processing" \
    "$FIXTURES_DIR/testdata/mock-commands/cpu/iotop-test-mixed-units" \
    "$expected_summary_4" \
    5 \
    50 \
    "various data units (B/s, K/s, M/s, G/s) properly parsed" \
    "true"

# Test 5: Many processes (tests process handling with head -n 20 limitation)
expected_summary_5='{"total_disk_read":"50.25 M/s","total_disk_write":"30.45 M/s","current_disk_read":"25.12 M/s","current_disk_write":"15.23 M/s"}'
run_iotop_message_test \
    "Process Count Limiting" \
    "$FIXTURES_DIR/testdata/mock-commands/cpu/iotop-test-many-processes" \
    "$expected_summary_5" \
    12 \
    40 \
    "large process list with head -n 20 limitation producing 12 processes" \
    "true"

# Test 6: Unicode character handling
expected_summary_6='{"total_disk_read":"3.45 K/s","total_disk_write":"2.34 K/s","current_disk_read":"1.67 K/s","current_disk_write":"1.23 K/s"}'
run_iotop_message_test \
    "Unicode Character Handling" \
    "$FIXTURES_DIR/testdata/mock-commands/cpu/iotop-test-unicode" \
    "$expected_summary_6" \
    2 \
    50 \
    "Unicode characters in usernames and commands properly handled" \
    "true"

# Test 7: Control characters (tabs, newlines) escaping
expected_summary_7='{"total_disk_read":"1.89 K/s","total_disk_write":"1.23 K/s","current_disk_read":"0.95 K/s","current_disk_write":"0.67 K/s"}'
run_iotop_message_test \
    "Control Characters Escaping" \
    "$FIXTURES_DIR/testdata/mock-commands/cpu/iotop-test-control-chars" \
    "$expected_summary_7" \
    2 \
    60 \
    "tab and newline characters properly escaped in JSON" \
    "true"

# Test 8: Production issue - java command ending with dash
expected_summary_8='{"total_disk_read":"0.00 B/s","total_disk_write":"143.35 K/s","current_disk_read":"0.00 B/s","current_disk_write":"0.00 B/s"}'
run_iotop_message_test \
    "Production Java Command Issue - trailing dash" \
    "$FIXTURES_DIR/testdata/mock-commands/cpu/iotop-test-production-issue-end-dash" \
    "$expected_summary_8" \
    2 \
    200 \
    "production java command ending with dash character properly handled" \
    "true"

# Test 9: Production issue - command line including the text `ERROR`
expected_summary_9='{"total_disk_read":"0.00 B/s","total_disk_write":"143.35 K/s","current_disk_read":"0.00 B/s","current_disk_write":"0.00 B/s"}'
run_iotop_message_test \
    "Production Java Command Issue - parameter includes ERROR" \
    "$FIXTURES_DIR/testdata/mock-commands/cpu/iotop-test-production-issue-commandline_containing_error" \
    "$expected_summary_9" \
    2 \
    200 \
    "production java command including ERROR parameter properly handled" \
    "true"

add_section "iotop Error Handling Tests"

# Test 10: iotop permission denied (should fail gracefully)
run_iotop_message_test \
    "iotop Permission Denied Handling" \
    "$FIXTURES_DIR/testdata/mock-commands/cpu/iotop-test-permission-denied" \
    "null" \
    0 \
    0 \
    "graceful handling when iotop permission is denied" \
    "false"

# Test 11: iotop empty output (should fail gracefully)
run_iotop_message_test \
    "iotop Empty Output Handling" \
    "$FIXTURES_DIR/testdata/mock-commands/cpu/iotop-test-empty" \
    "null" \
    0 \
    0 \
    "graceful handling when iotop returns empty output" \
    "false"

# Test 12: iotop malformed output (should fail gracefully)
run_iotop_message_test \
    "iotop Malformed Output Handling" \
    "$FIXTURES_DIR/testdata/mock-commands/cpu/iotop-test-malformed" \
    "null" \
    0 \
    0 \
    "graceful handling when iotop returns malformed output" \
    "false"

# End fixture - this handles summary, results writing, and output flushing
end_fixture "$(basename "$0")"

# Exit with appropriate code
if [ "$FAILED" -eq 0 ]; then
    exit 0
else
    exit 1
fi
