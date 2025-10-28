#!/bin/bash
# Test script for NPD startup functionality
# Tests various scenarios including GPU detection, IMDS responses, configuration management, and NPD startup

# Source common functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common/test_common.sh"

# Script under test
readonly SCRIPT_UNDER_TEST="/usr/local/bin/node-problem-detector-startup.sh"

# Helper function to run NPD startup tests following CPU/Memory test patterns
run_startup_test() {
    local test_name="$1"
    local expected_output="$2"
    local mock_data_dir="${3:-$SCRIPT_DIR/testdata/mock-data/startup-gpu-with-nvlink}"
    local custom_env_vars="$4"
    local timeout_seconds="${5:-30}"
    local expected_exit_code="${6:-0}"

    # Build volume mounts
    local volume_mounts=""

    # Mount scenario-specific mock commands
    if [ -d "$mock_data_dir/mock-commands" ]; then
        volume_mounts+="-v \"$mock_data_dir/mock-commands:/mock-commands:ro\""
    fi

    # Mount scenario-specific configuration files to non-conflicting paths
    if [ -d "$mock_data_dir" ]; then
        [ -d "$mock_data_dir/etc" ] && volume_mounts+=" -v \"$mock_data_dir/etc:/mock-etc:ro\""
        [ -d "$mock_data_dir/home" ] && volume_mounts+=" -v \"$mock_data_dir/home:/mock-home:ro\""
        [ -d "$mock_data_dir/var" ] && volume_mounts+=" -v \"$mock_data_dir/var:/mock-var:ro\""
    fi

    # NPD-specific diagnostic keywords for better error reporting
    local npd_keywords="GPU|IMDS|container-runtime|public-settings|Copying|not found|missing|Skipping toggle|nvidia-smi|vmSize|jq|parse error"

    # Call the generic run_test function with timeout, expected exit code, and diagnostic keywords
    run_test "$SCRIPT_UNDER_TEST" "$test_name" "$expected_output" "$volume_mounts" "$custom_env_vars" "$timeout_seconds" "$expected_exit_code" "$npd_keywords"
}

start_fixture "node-problem-detector-startup.sh Tests"

add_section "GPU Detection Tests"

# GPU-enabled VM with working drivers
run_startup_test "GPU VM with Working Drivers" \
    "GPU detected with working NVIDIA drivers" \
    "$SCRIPT_DIR/testdata/mock-data/startup-gpu-with-nvlink"

# GPU-enabled VM without drivers
run_startup_test "GPU VM without Drivers" \
    "GPU detected but nvidia-smi is not accessible" \
    "$SCRIPT_DIR/testdata/mock-data/startup-gpu-missing-drivers"

# Non-GPU VM with comprehensive validation (positive + negative)
run_startup_test "Non-GPU VM" \
    "Error: Unsupported VM SKU 'Standard_D4s_v3'|NOT:Including GPU health check configurations" \
    "$SCRIPT_DIR/testdata/mock-data/startup-non-gpu"

add_section "NVLink Detection Tests"

# NVLink supported GPU
run_startup_test "NVLink Supported GPU" \
    "NVLink hardware support detected" \
    "$SCRIPT_DIR/testdata/mock-data/startup-gpu-with-nvlink"

# GPU without NVLink support
run_startup_test "GPU without NVLink Support" \
    "Error: Unsupported VM SKU 'Standard_NC24s_v3'|NOT:Including NVLink health check configurations" \
    "$SCRIPT_DIR/testdata/mock-data/startup-gpu-without-nvlink"

# NVLink detection with driver failure
run_startup_test "NVLink Detection with Driver Failure" \
    "Skipping NVLink health checks|NOT:Including NVLink health check configurations" \
    "$SCRIPT_DIR/testdata/mock-data/startup-gpu-driver-failure"

add_section "Azure IMDS Tests"

# Successful IMDS response
run_startup_test "Successful IMDS Response" \
    "Standard_ND96isr_H100_v5" \
    "$SCRIPT_DIR/testdata/mock-data/startup-gpu-with-nvlink"

# IMDS timeout (script exits with curl timeout)
run_startup_test "IMDS Timeout" \
    "Copying mock NPD configuration files" \
    "$SCRIPT_DIR/testdata/mock-data/startup-imds-timeout" \
    "" \
    5 \
    28  # Expected exit code from curl timeout

# IMDS malformed response
run_startup_test "IMDS Malformed Response" \
    "parse error" \
    "$SCRIPT_DIR/testdata/mock-data/startup-invalid-json" \
    "" \
    30 \
    5  # Expected exit code from jq parse error

# IMDS empty response
run_startup_test "IMDS Empty Response" \
    "GPU SKU detection will be skipped" \
    "$SCRIPT_DIR/testdata/mock-data/startup-imds-empty"

add_section "Configuration Management Tests"

# Valid kubeconfig in master node location
run_startup_test "Kubeconfig in User Home Location" \
    "test-cluster-12345678.hcp.eastus.azmk8s.io" \
    "$SCRIPT_DIR/testdata/mock-data/startup-gpu-with-nvlink"

# Valid kubeconfig in worker node location
run_startup_test "Kubeconfig in Kubelet Location" \
    "worker-cluster-87654321.hcp.westus.azmk8s.io" \
    "$SCRIPT_DIR/testdata/mock-data/startup-non-gpu"

# Missing container runtime endpoint (script exits early due to set -e)
run_startup_test "Missing Container Runtime Endpoint" \
    "Copying mock NPD configuration files" \
    "$SCRIPT_DIR/testdata/mock-data/startup-missing-config" \
    "" \
    30 \
    1  # Expected exit code from early script termination

# Node name conversion (uppercase to lowercase)
run_startup_test "Node Name Case Conversion" \
    "aks-nodepool1-12345678-vmss000001" \
    "$SCRIPT_DIR/testdata/mock-data/startup-hostname-uppercase"

add_section "Toggle Management Tests"

# NPD validation toggle enabled
run_startup_test "NPD Validation Toggle Enabled" \
    "npd-validate-in-prod is enabled" \
    "$SCRIPT_DIR/testdata/mock-data/startup-gpu-with-nvlink"

# NPD validation toggle disabled
run_startup_test "NPD Validation Toggle Disabled" \
    "npd-validate-in-prod is disabled" \
    "$SCRIPT_DIR/testdata/mock-data/startup-gpu-with-nvlink" \
    "-e NPD_VALIDATE_ENABLED=false"

# Missing public settings file (script exits early)
run_startup_test "Missing Public Settings File" \
    "Copying mock NPD configuration files" \
    "$SCRIPT_DIR/testdata/mock-data/startup-missing-config" \
    "" \
    30 \
    1  # Expected exit code from early script termination

add_section "Plugin Configuration Tests"

# GPU plugin files included for GPU VM
run_startup_test "GPU Plugin Files Included" \
    "Including GPU health check configurations" \
    "$SCRIPT_DIR/testdata/mock-data/startup-gpu-with-nvlink"

# Positive test: GPU VM should include GPU configurations (validates the message appears)
run_startup_test "GPU VM Should Include GPU Configs" \
    "Including GPU health check configurations" \
    "$SCRIPT_DIR/testdata/mock-data/startup-gpu-with-nvlink"

# NVLink plugin files included for NVLink GPU
run_startup_test "NVLink Plugin Files Included" \
    "Including NVLink health check configurations" \
    "$SCRIPT_DIR/testdata/mock-data/startup-gpu-with-nvlink"

# No GPU plugins for non-GPU VM
run_startup_test "No GPU Plugins for Non-GPU VM" \
    "Skipping GPU health checks" \
    "$SCRIPT_DIR/testdata/mock-data/startup-non-gpu"

# Negative test: Non-GPU VM should NOT include GPU configurations
run_startup_test "Non-GPU VM Should Not Include GPU Configs" \
    "NOT:Including GPU health check configurations" \
    "$SCRIPT_DIR/testdata/mock-data/startup-non-gpu"



add_section "Error Handling Tests"

# Invalid JSON in VM capabilities file
run_startup_test "Invalid VM Capabilities JSON" \
    "parse error" \
    "$SCRIPT_DIR/testdata/mock-data/startup-invalid-json" \
    "" \
    30 \
    5  # Expected exit code from jq parse error

# Missing GPU plugin configuration file (script exits early)
run_startup_test "Missing GPU Plugin Config" \
    "Copying mock NPD configuration files" \
    "$SCRIPT_DIR/testdata/mock-data/startup-missing-config" \
    "" \
    30 \
    1  # Expected exit code from early script termination

# Missing NVLink plugin configuration file (script exits early)
run_startup_test "Missing NVLink Plugin Config" \
    "Copying mock NPD configuration files" \
    "$SCRIPT_DIR/testdata/mock-data/startup-missing-config" \
    "" \
    30 \
    1  # Expected exit code from early script termination

add_section "Integration Tests"

# Complete startup flow for GPU VM
run_startup_test "Complete GPU VM Startup" \
    "Including GPU health check configurations" \
    "$SCRIPT_DIR/testdata/mock-data/startup-gpu-with-nvlink"

# Complete startup flow for non-GPU VM
run_startup_test "Complete Non-GPU VM Startup" \
    "node-problem-detector started successfully with args|NOT:Including GPU health check configurations" \
    "$SCRIPT_DIR/testdata/mock-data/startup-non-gpu"

# Startup with mixed toggles (GPU enabled, validation disabled)
run_startup_test "Mixed Toggle Configuration" \
    "Including GPU health check configurations" \
    "$SCRIPT_DIR/testdata/mock-data/startup-gpu-with-nvlink" \
    "-e NPD_VALIDATE_ENABLED=false"

# Startup with all toggles disabled
run_startup_test "All Toggles Disabled" \
    "custom-rx-buffer-errors-monitor.json from custom-plugin-monitor|NOT:Including GPU health check configurations" \
    "$SCRIPT_DIR/testdata/mock-data/startup-non-gpu" \
    "-e GPU_CHECKS_ENABLED=false -e NPD_VALIDATE_ENABLED=false"

add_section "Performance Tests"

# Test startup time for normal case
run_startup_test "Normal Startup Performance" \
    "node-problem-detector" \
    "$SCRIPT_DIR/testdata/mock-data/startup-gpu-with-nvlink" \
    "" \
    1

add_section "Edge Case Tests"

# Very long hostname
run_startup_test "Long Hostname Handling" \
    "very-long-hostname-that-exceeds-normal-limits" \
    "$SCRIPT_DIR/testdata/mock-data/startup-hostname-long"

# Special characters in configuration
run_startup_test "Special Characters in Config" \
    "node-problem-detector" \
    "$SCRIPT_DIR/testdata/mock-data/startup-kubelet-special-chars"


# Check if main tests failed - exit early if so
if [ "$FAILED" -gt 0 ]; then
    buffer_output ""
    buffer_output "${RED}Main NPD startup tests failed. Exiting.${NC}"
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
