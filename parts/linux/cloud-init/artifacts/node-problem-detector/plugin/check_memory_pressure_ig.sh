#!/bin/bash

# This script checks various memory metrics to determine if the node is experiencing memory pressure
# If pressure is detected, it logs the output of IG to the Extension's events directory
# Refer to this spec for fileformat for sending logs to Kusto via Wireserver:
# https://github.com/Azure/azure-vmextension-publishing/wiki/5.0-Telemetry-Events

set -o nounset
set -o pipefail

# Source common functions
SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"
source "${SCRIPT_DIR}/npd_common.sh" || { echo "ERROR: Failed to source npd_common.sh"; exit 1; }
source "${SCRIPT_DIR}/pressure_common.sh" || { echo "ERROR: Failed to source pressure_common.sh"; exit 1; }

# Exit codes
OK=0
NOTOK=0   # Always exit with OK for now so we don't raise an event

# Clean up old log files
cleanup_old_logs

# Run single pressure check
# Thresholds can be overridden via environment variables (see pressure_common.sh)
if ! check_memory_pressure; then
    log "Memory pressure detected on node"

    log_ig_top_process_results "npd:check_memory_pressure_ig:top_process" "memory"
    
    write_logs "npd:check_memory_pressure_ig:log"
    exit $NOTOK
else
    log "No memory pressure detected on node"
    exit $OK
fi
