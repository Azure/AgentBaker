#!/usr/bin/env bash
# This script checks the health of NVIDIA DCGM (Data Center GPU Manager) services.
# It is used by Node Problem Detector to monitor whether the nvidia-dcgm and
# nvidia-dcgm-exporter services are running on nodes with GPU resources. The script
# verifies that both services are installed and active. It exits with:
# - OK (0): All DCGM services are active and running normally
# - NONOK (1): One or more services exist but are not active
# - UNKNOWN (2): systemctl not available or one or more services not installed
set -euo pipefail

readonly OK=0
readonly NONOK=1
readonly UNKNOWN=2

# Services to monitor
readonly -a NVIDIA_SERVICES=(
    "nvidia-dcgm"
    "nvidia-dcgm-exporter"
)

function main() {
    # Check if systemctl command exists
    if ! command -v systemctl >/dev/null 2>&1; then
        echo "systemctl command not found"
        exit "${UNKNOWN}"
    fi

    local not_installed_services=()
    for service in "${NVIDIA_SERVICES[@]}"; do
        if ! systemctl list-unit-files "${service}.service" --no-legend 2>/dev/null | grep -q "^${service}.service"; then
            not_installed_services+=("${service}")
        fi
    done

    # We error out if any of the services are not installed
    if [ ${#not_installed_services[@]} -gt 0 ]; then
        echo "Systemd service(s) ${not_installed_services[*]} not found (may not be installed)"
        exit "${UNKNOWN}"
    fi

    # Since all the services are installed, check if they are active
    local inactive_services=()
    for service in "${NVIDIA_SERVICES[@]}"; do
        if ! systemctl is-active --quiet "${service}.service"; then
            inactive_services+=("${service}")
        fi
    done

    if [ ${#inactive_services[@]} -gt 0 ]; then
        echo "Systemd service(s) ${inactive_services[*]} are not active"
        exit "${NONOK}"
    fi

    echo "All Nvidia DCGM services are active"
    exit "${OK}"
}

main
