#!/bin/bash

# This plugin checks if NVLink is properly functioning on all GPUs.
# It verifies that all NVLink connections are active for GPUs that support NVLink.
# Note: This script assumes NVLink hardware support has already been verified at startup.

readonly OK=0
readonly NONOK=1

# Check if nvidia-smi exists
if ! command -v nvidia-smi >/dev/null 2>&1; then
    echo "Error: nvidia-smi not found. Cannot check NVLink status."
    exit $NONOK
fi

# Get the number of GPUs with improved error handling
gpu_list_output=$(nvidia-smi --list-gpus 2>&1)
gpu_list_exit_code=$?

if [ $gpu_list_exit_code -ne 0 ]; then
    echo "Error: nvidia-smi --list-gpus failed with exit code $gpu_list_exit_code: $gpu_list_output"
    exit $NONOK
fi

num_gpus=$(echo "$gpu_list_output" | wc -l)

if [ "$num_gpus" -eq 0 ]; then
    echo "nvidia-smi command succeeded but found 0 GPUs. No NVLink checks needed."
    exit $OK
fi

# Get NVLink status - this should work since we've already verified support at startup
nvlink_status=$(nvidia-smi nvlink --status 2>&1)
nvlink_exit_code=$?

if [ $nvlink_exit_code -ne 0 ]; then
    echo "Failed to get NVLink status with error code $nvlink_exit_code. FaultCode: NHC2016"
    exit $NONOK
fi

# If NVLink status returns empty, report as error since we expect NVLink to be present
if [ -z "$nvlink_status" ]; then
    echo "NVLink status returned empty output. FaultCode: NHC2016"
    exit $NONOK
fi

# Check each GPU's NVLink status
all_links_active=true
error_messages=""

for ((i=0; i<num_gpus; i++)); do
    gpu_id=$i
    # Run NVLink command for specific GPU
    nvlink_output=$(nvidia-smi nvlink -s -i "$gpu_id" 2>&1)
    nvlink_gpu_exit_code=$?
    
    if [ $nvlink_gpu_exit_code -ne 0 ]; then
        echo "Failed to get NVLink status for GPU $gpu_id. FaultCode: NHC2016"
        exit $NONOK
    fi

    # Check for inactive links
    if echo "$nvlink_output" | grep -q "all links are inactive"; then
        error_messages="${error_messages}GPU $gpu_id has all NVLinks inactive. "
        all_links_active=false
    elif echo "$nvlink_output" | grep -q "inactive"; then
        # Extract and display the information about inactive links
        inactive_links=$(echo "$nvlink_output" | grep "Link" | grep "<inactive>" | sed 's/Link \([0-9]*\): <inactive>/Link \1/')
        error_messages="${error_messages}GPU $gpu_id has inactive NVLinks: $inactive_links. "
        all_links_active=false
    fi
done

# Report results
if [ "$all_links_active" = true ]; then
    echo "All GPUs have all NVLink connections active"
    exit $OK
else
    echo "NVLink issues detected: ${error_messages}FaultCode: NHC2016"
    exit $NONOK
fi