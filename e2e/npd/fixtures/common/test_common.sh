#!/bin/bash
# Common testing utilities for NPD Docker tests
# This file provides reusable functions for test execution and reporting

# Colors
RED='\033[0;31m'
DARK_RED='\033[1;31m'
ORANGE='\033[0;33m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Global test counters
PASSED=0
FAILED=0

# Output buffer for non-interleaved output when running test files in parallel
OUTPUT_BUFFER=""

# Debug mode flag - controls detailed validation output and immediate vs buffered output
# When DEBUG_MODE=true: shows debug messages + immediate test results (for serial debugging)
# When DEBUG_MODE=false: buffered output for clean parallel execution
DEBUG_MODE="${DEBUG_MODE:-false}"

# Single test mode flag - controls immediate output for individual test execution
# When SINGLE_TEST_MODE=true: shows immediate test results (no debug details)
# When SINGLE_TEST_MODE=false: buffered output for parallel execution
SINGLE_TEST_MODE="${SINGLE_TEST_MODE:-false}"

# Common test configuration
DOCKER_IMAGE="npd-test-persistent" #container name for tests that reuse the same container
DOCKER_ENTRYPOINT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/../../docker/docker_entrypoint.sh"

# Buffer output function - immediate output in debug/single test mode, buffered otherwise
buffer_output() {
    if [ "$DEBUG_MODE" = "true" ] || [ "$SINGLE_TEST_MODE" = "true" ]; then
        echo -e "$1"  # Immediate output in debug mode or single test mode
    else
        OUTPUT_BUFFER="$OUTPUT_BUFFER\n$1"  # Buffer for parallel mode
    fi
}

# Flush all buffered output at once (only needed when buffering)
flush_output() {
    if [ "$DEBUG_MODE" != "true" ] && [ "$SINGLE_TEST_MODE" != "true" ]; then
        echo -e "$OUTPUT_BUFFER"
    fi
}

# Debug output function - only outputs when debug mode is enabled
debug_output() {
    if [ "$DEBUG_MODE" = "true" ]; then
        echo "$1"
    fi
}

# Test fixture lifecycle functions

# Start a test fixture - initialize buffer and show header
start_fixture() {
    local fixture_name="$1"
    OUTPUT_BUFFER=""
    PASSED=0
    FAILED=0
    buffer_output "$(print_fixture_header "$fixture_name")"
}

# End a test fixture - show summary and flush output
end_fixture() {
    local script_name="$1"
    buffer_output "$(print_test_summary)"
    write_test_results "$script_name"
    flush_output
}

# Add a section header to the buffer
add_section() {
    local section_name="$1"
    buffer_output "$(print_section_header "$section_name")"
}

# Test result tracking function
test_result() {
    local test_name="$1"
    local result="$2"

    if [ "$result" = "PASSED" ]; then
        buffer_output "Testing: $test_name... ${GREEN}PASSED${NC}"
        ((PASSED++))
    else
        buffer_output "Testing: $test_name... ${RED}FAILED${NC}"
        ((FAILED++))
    fi
}

# Function to run a single test with completely generic volume mounting
# Usage: run_test "script_to_test" "test_name" "expected_output" "volume_mounts" "env_vars" [timeout_seconds] [expected_exit_code] [diagnostic_keywords]
run_test() {
    local script_to_test="$1"
    local test_name="$2"
    local expected_output="$3"
    local volume_mounts="$4"
    local env_vars="$5"
    local timeout_seconds="${6:-20}"
    local expected_exit_code="${7:-0}"
    local diagnostic_keywords="${8:-}"

    # Generate unique container name
    local test_type=$(basename "$script_to_test" | sed 's/^check_//' | sed 's/\.sh$//' | sed 's/_/-/g')
    local container_name="npd-test-${test_type}-$(date +%s%N | cut -b1-13)"

    # Debug output for test parameters
    debug_output "🔍 Starting test: $test_name"
    debug_output "  Script: $script_to_test"
    debug_output "  Container: $container_name"
    debug_output "  Expected output: $expected_output"
    debug_output "  Expected exit code: $expected_exit_code"
    debug_output "  Timeout: ${timeout_seconds}s"

    # Verify that all volume mount source paths exist before attempting Docker mount
    if [ -n "$volume_mounts" ]; then
        # Extract host paths from volume mount strings like: -v "host_path:container_path:ro"
        # Split the volume_mounts string and extract host paths from each -v option
    local volume_args
    # Use -r with read to avoid backslash interpretation (SC2162 no longer applies because -r is present)
    read -r -a volume_args <<< "$volume_mounts"

        local i=0
        while [ $i -lt ${#volume_args[@]} ]; do
            if [ "${volume_args[$i]}" = "-v" ] && [ $((i+1)) -lt ${#volume_args[@]} ]; then
                # Extract host path (everything before first colon)
                local volume_spec="${volume_args[$((i+1))]}"
                # Remove quotes and extract host path
                local host_path=$(echo "$volume_spec" | sed 's/^"//' | sed 's/"$//' | cut -d: -f1)

                # Check if path exists
                if [ -n "$host_path" ] && [ ! -e "$host_path" ]; then
                    buffer_output "Testing: $test_name... ${RED}FAILED${NC}"
                    buffer_output "  ${RED}ERROR:${NC} Test data directory missing: $host_path"
                    buffer_output "  This usually happens after 'make clean'. Try running the relevant test data creation script(s)"
                    ((FAILED++))
                    return 1
                fi
                i=$((i+2))  # Skip both -v and its argument
            else
                i=$((i+1))
            fi
        done
    fi

    # Build docker run command
    local docker_cmd="timeout ${timeout_seconds}s docker run --name $container_name --rm --privileged"

    # Always mount entrypoint script
    if [ -n "$DOCKER_ENTRYPOINT" ]; then
        docker_cmd+=" -v \"$DOCKER_ENTRYPOINT:/entrypoint.sh:ro\""
    fi

    # Add all volume mounts (completely generic)
    if [ -n "$volume_mounts" ]; then
        docker_cmd+=" $volume_mounts"
    fi

    # Add environment variables with default PATH
    local final_env_vars="$env_vars"

    # Check if PATH is already specified in env_vars (POSIX form; avoid bash [[ ]] for shellcheck --shell=sh)
    case "$env_vars" in
        *PATH=*)
            # PATH already specified, just append system paths
            final_env_vars=$(echo "$final_env_vars" | sed 's|-e PATH=\([^[:space:]]*\)|-e PATH=\1:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin|')
            ;;
        *)
            # No PATH specified, add default mock commands path
            final_env_vars+=" -e PATH=/mock-commands:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
            ;;
    esac

    if [ -n "$final_env_vars" ]; then
        docker_cmd+=" $final_env_vars"
    fi

    # Add image and script to run
    docker_cmd+=" --entrypoint /entrypoint.sh"
    docker_cmd+=" $DOCKER_IMAGE"
    docker_cmd+=" $script_to_test 2>&1"

    # Debug output for Docker command
    debug_output "🐳 Docker command: $docker_cmd"

    # Set up cleanup trap for this specific container
    trap 'docker rm -f "$container_name" 2>/dev/null || true' EXIT

    # Run the test with timeout and capture both stdout and stderr
    local actual_exit_code=0
    output=$(eval "$docker_cmd" 2>&1) || actual_exit_code=$?

    # Debug output for script results
    debug_output "📤 Script exit code: $actual_exit_code"
    debug_output "📜 Script output:"
    if [ "$DEBUG_MODE" = "true" ]; then
        echo "$output" | head -20 | sed 's/^/    /'
    fi

    # Enhanced exit code validation - check if actual matches expected
    if [ $actual_exit_code -eq $expected_exit_code ]; then
        # Exit code matches expectation, now check output validation
        debug_output "✅ Exit code matches expected: $expected_exit_code"

        # Check for dual validation (pipe delimiter: "positive|NOT:negative")
        case "$expected_output" in
        *"|"*)
            local positive_part="${expected_output%%|*}"      # Everything before |
            local negative_part="${expected_output#*|}"       # Everything after |
            debug_output "🔍 Using dual validation: positive='$positive_part' | negative='$negative_part'"

            # Validate positive case
            if ! echo "$output" | grep -Fq "$positive_part"; then
                buffer_output "Testing: $test_name... ${RED}FAILED${NC}"
                buffer_output "  Exit code: $actual_exit_code (expected: $expected_exit_code) ✓"
                buffer_output "  ${RED}ERROR:${NC} Expected text not found: '$positive_part'"
                buffer_output "  Output excerpt:"
            else
                # Positive validation passed, now check negative case
                case "$negative_part" in
                NOT:*)
                    local prohibited_text="${negative_part#NOT:}"
                    if echo "$output" | grep -Fq "$prohibited_text"; then
                        buffer_output "Testing: $test_name... ${RED}FAILED${NC}"
                        buffer_output "  Exit code: $actual_exit_code (expected: $expected_exit_code) ✓"
                        buffer_output "  ${GREEN}✓${NC} Found required text: '$positive_part'"
                        buffer_output "  ${RED}ERROR:${NC} ${ORANGE}Found unexpected text: '$prohibited_text'${NC}"
                        buffer_output "  Output excerpt:"
                    else
                        # Both validations passed
                        buffer_output "Testing: $test_name... ${GREEN}PASSED${NC}"
                        ((PASSED++))
                        # Clean up container and reset trap
                        trap - EXIT
                        docker rm -f $container_name 2>/dev/null || true
                        return 0
                    fi
                ;;
                *)
                    # No negative validation, just positive passed
                    buffer_output "Testing: $test_name... ${GREEN}PASSED${NC}"
                    ((PASSED++))
                    # Clean up container and reset trap
                    trap - EXIT
                    docker rm -f $container_name 2>/dev/null || true
                    return 0
                ;;
                esac
            fi
        # Check for negative validation (NOT: prefix)
        ;; # end dual validation case arm
        NOT:*)
            local prohibited_text="${expected_output#NOT:}"
            debug_output "🔍 Using negative validation: NOT '$prohibited_text'"
            if echo "$output" | grep -Fq "$prohibited_text"; then
                buffer_output "Testing: $test_name... ${RED}FAILED${NC}"
                buffer_output "  Exit code: $actual_exit_code (expected: $expected_exit_code) ✓"
                buffer_output "  ${RED}ERROR:${NC} ${ORANGE}Found unexpected text: '$prohibited_text'${NC}"
                buffer_output "  Output excerpt:"
            else
                buffer_output "Testing: $test_name... ${GREEN}PASSED${NC}"
                ((PASSED++))
                # Clean up container and reset trap
                trap - EXIT
                docker rm -f $container_name 2>/dev/null || true
                return 0
            fi
        ;;
        *)
            # Standard positive validation
            debug_output "🔍 Using standard validation: looking for '$expected_output'"
            if echo "$output" | grep -Fq "$expected_output"; then
                buffer_output "Testing: $test_name... ${GREEN}PASSED${NC}"
                ((PASSED++))
                # Clean up container and reset trap
                trap - EXIT
                docker rm -f $container_name 2>/dev/null || true
                return 0
            else
                buffer_output "Testing: $test_name... ${RED}FAILED${NC}"
                buffer_output "  Exit code: $actual_exit_code (expected: $expected_exit_code) ✓"
                buffer_output "  Expected to find: '$expected_output'"
                buffer_output "  Output excerpt:"
            fi
    ;;
    esac

        # Build dynamic grep pattern for diagnostic output
        local base_keywords="pressure|PSI|PRESSURE|INFO|ERROR|detected|buffer|ratio|threshold"
        local all_keywords="$base_keywords"
        if [ -n "$diagnostic_keywords" ]; then
            all_keywords="$base_keywords|$diagnostic_keywords"
        fi

        while IFS= read -r line; do
            buffer_output "$line"
        done < <(echo "$output" | grep -E "($all_keywords)" | head -10 | sed 's/^/    /')
        ((FAILED++))
        # Clean up container and reset trap
        trap - EXIT
        docker rm -f $container_name 2>/dev/null || true
        return 1
    elif [ $actual_exit_code -eq 124 ]; then
        debug_output "❌ Test failed: timeout after ${timeout_seconds}s"
        buffer_output "Testing: $test_name... ${RED}FAILED (timeout)${NC}"
        buffer_output "  Test timed out after ${timeout_seconds} seconds"
        buffer_output "  Expected exit code: $expected_exit_code"
        ((FAILED++))
        # Clean up container and reset trap
        trap - EXIT
        docker rm -f $container_name 2>/dev/null || true
        return 1
    else
        debug_output "❌ Test failed: exit code mismatch (got $actual_exit_code, expected $expected_exit_code)"
        buffer_output "Testing: $test_name... ${RED}FAILED (exit code: $actual_exit_code)${NC}"
        buffer_output "  Expected exit code: $expected_exit_code"
        buffer_output "  Script error output:"
        while IFS= read -r line; do
            buffer_output "$line"
        done < <(echo "$output" | head -15 | sed 's/^/    /')
        ((FAILED++))
        # Clean up container and reset trap
        trap - EXIT
        docker rm -f $container_name 2>/dev/null || true
        return 1
    fi
}


# Function to print a comprehensive test summary
print_test_summary() {

    echo -e "\n${YELLOW}Test Summary:${NC}"
    echo -e "  ${GREEN}Passed: $PASSED${NC}"
    echo -e "  ${RED}Failed: $FAILED${NC}"
}

# Function to write test results to shared file for overall summary
write_test_results() {
    local script_name="$1"
    echo "$PASSED,$FAILED,$script_name" >> /tmp/npd_test_results.txt
}

# Function to print overall summary from all test files
print_overall_summary() {
    if [ ! -f /tmp/npd_test_results.txt ]; then
        echo "No test results found"
        return
    fi

    local total_passed=0
    local total_failed=0

    echo -e "\n${YELLOW}=== Overall Test Summary ===${NC}"
    while IFS=',' read -r passed failed script; do
        echo -e "  ${script}: ${GREEN}${passed} passed${NC}, ${RED}${failed} failed${NC}"
        total_passed=$((total_passed + passed))
        total_failed=$((total_failed + failed))
    done < /tmp/npd_test_results.txt

    echo -e "\n${YELLOW}Total: ${GREEN}${total_passed} passed${NC}, ${RED}${total_failed} failed${NC}"
    rm -f /tmp/npd_test_results.txt

    # Clean up any leftover test containers
    cleanup_containers
}

# Function to create main test fixture section header
print_fixture_header() {
    local section_name="$1"
    echo -e "\n${YELLOW}=== $section_name ===${NC}"
}

# Function to create a sub-section header
print_section_header() {
    local section_name="$1"
    echo -e "\n${DARK_RED}== $section_name ==${NC}\n"
}

# Container management functions

# Create a test container for longer running tests (multiple scenarios)
create_test_container() {
    local container_name="$1"
    local mock_proc_dir="$2"
    local mock_sys_dir="$3"
    local additional_volumes="$4"
    local env_vars="$5"

    # Auto-generate unique container name if not provided
    if [ -z "$container_name" ] || [ "$container_name" = "auto" ]; then
        container_name="npd-test-$(date +%s%N | cut -b1-13)"
    fi

    local docker_cmd="docker run -d --name $container_name --privileged"

    # Add entrypoint (always needed)
    docker_cmd="$docker_cmd -v $DOCKER_ENTRYPOINT:/entrypoint.sh:ro"

    # Create and mount event log directory for host-based validation
    # Use FIXTURES_DIR if available (for integration tests), otherwise use SCRIPT_DIR
    local base_dir="${FIXTURES_DIR:-$SCRIPT_DIR}"
    local event_log_host_dir="$base_dir/testdata/event-logs/$container_name"
    mkdir -p "$event_log_host_dir"
    docker_cmd="$docker_cmd -v $event_log_host_dir:/var/log/azure/Microsoft.AKS.Compute.AKS.Linux.AKSNode/events"

    # Add proc/sys mounts if provided
    if [ -n "$mock_proc_dir" ]; then
        docker_cmd="$docker_cmd -v $mock_proc_dir:/mock-proc:ro"
    fi
    if [ -n "$mock_sys_dir" ]; then
        docker_cmd="$docker_cmd -v $mock_sys_dir:/mock-sys:ro"
    fi

    # Add any additional volumes
    if [ -n "$additional_volumes" ]; then
        docker_cmd="$docker_cmd $additional_volumes"
    fi

    # Add environment variables
    if [ -n "$env_vars" ]; then
        docker_cmd="$docker_cmd $env_vars"
    fi

    # Complete the command
    docker_cmd="$docker_cmd --entrypoint /entrypoint.sh $DOCKER_IMAGE sleep 180"

    # Execute and return container ID
    eval "$docker_cmd" 2>/dev/null
}

# Cleanup container
cleanup_container() {
    local container_id="$1"
    if [ -n "$container_id" ]; then
        docker rm -f "$container_id" >/dev/null 2>&1
    fi
}

# Validate container ID and handle failure
validate_container_id() {
    local container_id="$1"
    local test_name="$2"

    if [ -z "$container_id" ]; then
        test_result "$test_name" "FAILED"
        return 1
    fi
    return 0
}

# Run command in container
run_in_container() {
    local container_id="$1"
    local command="$2"

    docker exec "$container_id" bash -c "$command"
}


# Build environment variables string
build_env_vars() {
    local base_vars="$1"
    local custom_vars="$2"

    local all_vars="$base_vars"
    if [ -n "$custom_vars" ]; then
        all_vars="$all_vars $custom_vars"
    fi
    echo "$all_vars"
}

# Function to build Docker image
build_test_image() {
    local image_name="$1"
    local dockerfile_path="$2"
    local build_context="$3"

    echo -e "${BLUE}Building Docker image $image_name...${NC}"
    docker build -t "$image_name" -f "$dockerfile_path" "$build_context"
}

# Function to reset test counters
reset_test_counters() {
    PASSED=0
    FAILED=0
}

# Clean up any running test containers
cleanup_containers() {
    # Find all containers with our test name patterns (both running and stopped)
    local containers=$(docker ps -a --format "{{.Names}}" | grep -E "^npd-test" 2>/dev/null || true)

    if [ -n "$containers" ]; then
        # Force remove all matching containers
        echo "$containers" | xargs -r docker rm -f 2>/dev/null || true
    fi
}
