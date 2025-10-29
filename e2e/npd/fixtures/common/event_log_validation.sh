#!/bin/bash
# Event log validation utilities for NPD Docker tests
# This file provides functions to validate event log files and their Message content

# Function to get the host event log directory for a container
get_event_log_host_dir() {
    local container_id="$1"
    # Get the container name from the container ID
    local container_name
    container_name=$(docker inspect --format='{{.Name}}' "$container_id" 2>/dev/null | sed 's|^/||')
    if [ -z "$container_name" ]; then
        echo "ERROR: Could not get container name for ID: $container_id" >&2
        return 1
    fi
    # Use FIXTURES_DIR if available (for integration tests), otherwise use SCRIPT_DIR
    local base_dir="${FIXTURES_DIR:-$SCRIPT_DIR}"
    echo "$base_dir/testdata/event-logs/$container_name"
}

# Function to validate that event log files exist for expected tasknames
validate_event_log_files_exist() {
    local container_id="$1"
    shift
    local expected_tasknames=("$@")
    
    debug_output "  Validating event log files exist for expected tasknames..."
    
    # Get host event log directory
    local host_event_log_dir
    host_event_log_dir=$(get_event_log_host_dir "$container_id")
    
    # Get all event log files
    local log_files
    log_files=$(find "$host_event_log_dir" -name "*.json" 2>/dev/null || echo "")
    
    if [ -z "$log_files" ]; then
        buffer_output "  No event log files found in $host_event_log_dir"
        return 1
    fi
    
    debug_output "    ✅ Found event log files:"
    if [ "$DEBUG_MODE" = "true" ]; then
        printf '%s\n' "$log_files" | sed 's/^/      /'
    fi
    
    # Check each expected taskname
    local validation_passed=true
    for taskname_pattern in "${expected_tasknames[@]}"; do
        debug_output "    Checking for taskname: $taskname_pattern"
        
        # Use find with -exec to search for the pattern
        local matching_files
        matching_files=$(find "$host_event_log_dir" -name "*.json" -exec grep -l "$taskname_pattern" {} \; 2>/dev/null || echo "")
        
        if [ -n "$matching_files" ]; then
            debug_output "      ✅ Found files for $taskname_pattern"
        else
            buffer_output "  No files found for $taskname_pattern"
            validation_passed=false
        fi
    done
    
    if [ "$validation_passed" = true ]; then
        debug_output "    ✅ All expected tasknames found in log files"
        return 0
    else
        buffer_output "  Some expected tasknames not found in log files"
        return 1
    fi
}

# Function to validate JSON structure of event log files
validate_event_log_json_structure() {
    local container_id="$1"
    local taskname_pattern="$2"
    
    debug_output "  Validating JSON structure for taskname pattern: $taskname_pattern"
    
    # Get host event log directory
    local host_event_log_dir
    host_event_log_dir=$(get_event_log_host_dir "$container_id")
    
    # Find files matching the taskname pattern
    local matching_files
    matching_files=$(find "$host_event_log_dir" -name "*.json" -exec grep -l "$taskname_pattern" {} \; 2>/dev/null || echo "")
    
    if [ -z "$matching_files" ]; then
        buffer_output "  No files found matching pattern: $taskname_pattern"
        return 1
    fi
    
    local validation_passed=true
    
    # Validate each file
    while IFS= read -r file; do
        debug_output "    Validating file: $(basename "$file")"
        
        # Check if file contains valid JSON
        local json_valid
        json_valid=$(jq . "$file" >/dev/null 2>&1 && echo "true" || echo "false")
        
        if [ "$json_valid" = "true" ]; then
            debug_output "      ✅ Valid JSON structure"
            
            # Validate required fields for event log format
            local required_fields=("Version" "Timestamp" "TaskName" "EventLevel" "Message" "EventPid" "EventTid" "OperationId")
            
            for field in "${required_fields[@]}"; do
                local field_present
                field_present=$(jq -e ".$field" "$file" >/dev/null 2>&1 && echo "true" || echo "false")
                
                if [ "$field_present" = "true" ]; then
                    debug_output "        ✅ Field '$field' present"
                else
                    buffer_output "        Field '$field' missing"
                    validation_passed=false
                fi
            done
            
            # Validate specific field values
            local version
            local event_level
            version=$(jq -r '.Version' "$file" 2>/dev/null)
            event_level=$(jq -r '.EventLevel' "$file" 2>/dev/null)
            
            if [ "$version" = "1.0" ]; then
                debug_output "        ✅ Version is correct: $version"
            else
                buffer_output "        Version is incorrect: $version (expected: 1.0)"
                validation_passed=false
            fi
            
            if [ "$event_level" = "Warning" ]; then
                debug_output "        ✅ EventLevel is correct: $event_level"
            else
                buffer_output "        EventLevel is incorrect: $event_level (expected: Warning)"
                validation_passed=false
            fi
            
        else
            buffer_output "      Invalid JSON structure"
            validation_passed=false
        fi
        
    done <<< "$matching_files"
    
    if [ "$validation_passed" = true ]; then
        debug_output "    ✅ All JSON files have valid structure"
        return 0
    else
        buffer_output "    Some JSON files have invalid structure"
        return 1
    fi
}

# Function to validate that NO event log files exist for specified tasknames
validate_event_log_files_not_exist() {
    local container_id="$1"
    shift
    local unexpected_tasknames=("$@")
    
    debug_output "  Validating that NO event log files exist for tasknames..."
    
    # Get host event log directory
    local host_event_log_dir
    host_event_log_dir=$(get_event_log_host_dir "$container_id")
    
    # Check each taskname that should NOT exist
    local validation_passed=true
    for taskname_pattern in "${unexpected_tasknames[@]}"; do
        debug_output "    Checking that taskname does NOT exist: $taskname_pattern"
        
        # Use find with -exec to search for the pattern
        local matching_files
        matching_files=$(find "$host_event_log_dir" -name "*.json" -exec grep -l "$taskname_pattern" {} \; 2>/dev/null || echo "")
        
        if [ -z "$matching_files" ]; then
            debug_output "      ✅ Correctly found NO files for $taskname_pattern"
        else
            buffer_output "  ERROR: Found unexpected files for $taskname_pattern when none should exist"
            if [ "$DEBUG_MODE" = "true" ]; then
                printf '%s\n' "$matching_files" | sed 's/^/    /'
            fi
            validation_passed=false
        fi
    done
    
    if [ "$validation_passed" = true ]; then
        debug_output "    ✅ All tasknames correctly absent from log files"
        return 0
    else
        buffer_output "  Some unexpected tasknames found in log files"
        return 1
    fi
}