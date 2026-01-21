#!/usr/bin/env bash
# This script checks the health of the NVIDIA Device Plugin systemd service.
# It is used by Node Problem Detector to monitor whether the nvidia-device-plugin
# service is running on nodes with GPU resources. The script exits with:
# - OK (0): Service is active and running normally
# - NONOK (1): Service exists but is not active
# - UNKNOWN (2): systemctl not available or service not installed
set -euo pipefail

readonly OK=0
readonly NONOK=1
readonly UNKNOWN=2

readonly NVIDIA_DEVICE_PLUGIN="nvidia-device-plugin"

function main() {
    if ! command -v systemctl >/dev/null 2>&1; then
        echo "systemctl command not found"
        exit "${UNKNOWN}"
    fi

    if ! systemctl list-unit-files "${NVIDIA_DEVICE_PLUGIN}.service" --no-legend 2>/dev/null | grep -q "^${NVIDIA_DEVICE_PLUGIN}.service"; then
        echo "Systemd service ${NVIDIA_DEVICE_PLUGIN} not found (may not be installed)."
        exit "${UNKNOWN}"
    fi

    if ! systemctl is-active --quiet "${NVIDIA_DEVICE_PLUGIN}.service"; then
        echo "Systemd service ${NVIDIA_DEVICE_PLUGIN} is not active."
        exit "${NONOK}"
    fi

    echo "Nvidia device plugin service is active."
    exit "${OK}"
}

main
