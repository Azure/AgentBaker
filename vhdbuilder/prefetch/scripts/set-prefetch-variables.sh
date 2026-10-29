#!/bin/bash
set -euo pipefail

set_prefetch_variables() {
    local template_dir commit
    template_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../templates" && pwd)" || return 1
    echo "##vso[task.setvariable variable=IMAGE_BUILDER_TEMPLATE_PATH]${template_dir}/optimize.json"

    if [[ "${BUILDER:-}" == imagecustomizer ]]; then
        echo "builder is imagecustomizer - VHD is not compatible with prefetch optimization"
        echo "##vso[task.setvariable variable=PREFETCH_COMPATIBLE]False"
        return 0
    fi
    if [[ "${OS_SKU:-}" == AzureContainerLinux ]]; then
        if [[ "${ARCHITECTURE,,}" != x86_64 || "${ENABLE_FIPS,,}" != false ||
            "${ENABLE_TRUSTED_LAUNCH,,}" != true || "${HYPERV_GENERATION}" != V2 ||
            "${FEATURE_FLAGS,,}" != none ]]; then
            echo "ACL prefetch currently supports non-FIPS AMD64 Gen2 Trusted Launch only"
            echo "##vso[task.setvariable variable=PREFETCH_COMPATIBLE]False"
            return 0
        fi
        commit="$(git rev-parse HEAD)" || return 1
        echo "##vso[task.setvariable variable=IMAGE_BUILDER_TEMPLATE_PATH]${template_dir}/acl.json"
        echo "##vso[task.setvariable variable=PREFETCH_WORKLOAD_OPTIMIZATION_SCRIPT_URL]https://raw.githubusercontent.com/Azure/AgentBaker/${commit}/vhdbuilder/scripts/acl-prefetch-driver.sh"
    elif [[ "${ARCHITECTURE,,}" == arm64 && "${FEATURE_FLAGS}" == *cvm* ]]; then
        echo "ARM64 with CVM - VHD is incompatible with prefetch optimization"
        echo "##vso[task.setvariable variable=PREFETCH_COMPATIBLE]False"
        return 0
    fi
    echo "VHD is compatible with prefetch optimization"
    echo "##vso[task.setvariable variable=PREFETCH_COMPATIBLE]True"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    set_prefetch_variables
fi
