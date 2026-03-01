#!/usr/bin/env bash
#
# This Node Problem Detector monitor validates that the observed GPU count
# matches the expected count for the current Azure VM SKU. It queries Azure IMDS
# to determine the VM size, looks up the expected GPU count from a capabilities
# file, and compares it against the actual GPU count detected via nvidia-smi or
# rocm-smi. If there's a mismatch, it raises fault code NHC2009 indicating a GPU
# hardware configuration issue.
set -euo pipefail

readonly OK=0
readonly NONOK=1
readonly GPU_VMS_CAPABILITIES="/etc/node-problem-detector.d/plugin/gpu_vms_capabilities.json"
readonly CACHE_VM_SKU_FILE="/tmp/vm_sku.cache"

function get_sku() {
    # If the vm cache file exists, reuse the cached vm sku, this is not gonna
    # change!
    if [ -f "${CACHE_VM_SKU_FILE}" ]; then
        cat "${CACHE_VM_SKU_FILE}"
        return
    fi

    # Azure Instance Metadata Service endpoint for compute information.
    local imds_endpoint="http://169.254.169.254/metadata/instance/compute?api-version=2021-02-01"
    local imds_timeout=10

    # Fetch IMDS response
    local imds_response
    imds_response=$(curl -s --max-time "${imds_timeout}" -H "Metadata: true" "${imds_endpoint}")
    if [ -z "${imds_response}" ]; then
        # This implies that you need to retry this again.
        echo "Warning: Failed to retrieve instance metadata"
        exit "${OK}"
    fi

    echo "${imds_response}" | jq -r '.vmSize' | tr '[:upper:]' '[:lower:]' | tee "${CACHE_VM_SKU_FILE}"
}

# Function to get the expected GPU count
function get_expected_gpu_count() {
    local vm_sku="${1}"

    # Check if the VM SKU exists (case insensitive) in our list of supported GPU VMs
    local matching_key
    matching_key=$(jq -r --arg sku "${vm_sku}" 'keys[] | select(ascii_downcase == $sku)' "${GPU_VMS_CAPABILITIES}" | head -n 1)
    if [ -z "${matching_key}" ]; then
        # This means that the GPU plugin was loaded for this VM SKU even though
        # it is not supported. Something is wrong with the plugin loading logic
        # in the node-problem-detector-startup.sh script.
        echo "Error: Unsupported VM SKU '${vm_sku}'"
        exit "${NONOK}"
    fi

    # Check if the GPUs field exists for this VM SKU and extract the value
    local gpu_count
    gpu_count=$(jq -r --arg sku "${matching_key}" '.[$sku].GPUs // "0"' "${GPU_VMS_CAPABILITIES}" 2>/dev/null)

    # Ensure we return a valid number, default to 0 if parsing fails
    if [[ "${gpu_count}" =~ ^[0-9]+$ ]]; then
        echo "${gpu_count}"
        return
    fi

    # If this is returning zero, that means the gpu_vms_capabilities.json has
    # this VM SKU, but the GPUs field is not a number.
    echo "Error: Invalid GPU count '${gpu_count}' for VM SKU '${vm_sku}'"
    exit "${NONOK}"
}

function get_observed_gpu_count() {
    local gpu_count
    # Determine GPU_TYPE variable by checking if nvidia-smi or rocm-smi exist.
    if command -v nvidia-smi >/dev/null 2>&1; then
        # nvidia-smi -L without --query-gpu flag will list all GPUs, including
        # MIG instances. To count only the physical GPUs, we use
        # --query-gpu=index.
        gpu_count=$(nvidia-smi --query-gpu=index --format=csv,noheader | wc -l)
    elif command -v rocm-smi >/dev/null 2>&1; then
        gpu_count=$(rocm-smi -l | grep -c 'GPU' || true)
    else
        echo "Error: Neither nvidia-smi nor rocm-smi found. Cannot determine GPU type."
        exit "${NONOK}"
    fi

    echo "${gpu_count}"
}

function main() {
    local vm_sku
    vm_sku=$(get_sku)

    local expected
    local observed

    expected=$(get_expected_gpu_count "${vm_sku}")
    observed=$(get_observed_gpu_count)

    if [ "${expected}" -ne "${observed}" ]; then
        echo "Expected to see ${expected} GPUs but found ${observed}. FaultCode: NHC2009"
        exit "${NONOK}"
    fi

    echo "Expected ${expected} GPUs and found ${observed}"
    exit "${OK}"
}

main
