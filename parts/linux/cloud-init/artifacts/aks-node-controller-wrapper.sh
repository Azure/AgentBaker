#!/bin/bash
set -uo pipefail

BIN_PATH="/opt/azure/containers/aks-node-controller"
CONFIG_PATH="/opt/azure/containers/aks-node-controller-config.json"
LOGGER_TAG="aks-node-controller-wrapper"

log() {
    local message="$1"
    # Emit to both journal (via logger) and stdout so systemd captures it.
    logger -t "$LOGGER_TAG" "$message"
    echo "$message"
}

log "Launching aks-node-controller with config ${CONFIG_PATH}"
"$BIN_PATH" provision --provision-config="$CONFIG_PATH" &
child_pid=$!
log "Spawned aks-node-controller (pid ${child_pid})"

wait "$child_pid"
exit_code=$?

if [ "$exit_code" -eq 0 ]; then
    log "aks-node-controller completed successfully"
else
    log "aks-node-controller exited with code ${exit_code}"
fi

exit $exit_code
